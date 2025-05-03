package k8s

import (
	"context"
	"encoding/base64"
	"fmt"
	"kube-jit/internal/models"
	"kube-jit/pkg/utils"
	"log"
	"os"
	"sync"
	"time"

	containerapiv1 "cloud.google.com/go/container/apiv1"
	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/container/v1"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Config struct {
	Clusters      []ClusterConfig `yaml:"clusters"`
	AllowedRoles  []models.Roles  `yaml:"allowedRoles"`
	ApproverTeams []models.Team   `yaml:"allowedApproverTeams"`
	AdminTeams    []models.Team   `yaml:"adminTeams"`
}

type ClusterConfig struct {
	Name        string `yaml:"name"`
	Host        string `yaml:"host"`
	CA          string `yaml:"ca"`
	Insecure    bool   `yaml:"insecure"`
	TokenSecret string `yaml:"tokenSecret"`
	Token       string `yaml:"token"`
	Type        string `yaml:"type"`      // e.g., "gke" or "generic" or "aks"
	ProjectID   string `yaml:"projectID"` // GCP project ID for GKE clusters
	Region      string `yaml:"region"`    // Region for GKE clusters
}

type CachedClient struct {
	Client       *dynamic.DynamicClient
	TokenExpires int64 // Unix timestamp for token expiration
}

type JitGroupsCache struct {
	JitGroups *unstructured.Unstructured
	ExpiresAt int64 // Unix timestamp for cache expiration
}

type JitGroup struct {
	// The group ID
	GroupID string `json:"groupID"`
	// The group namespace
	Namespace string `json:"namespace"`
}

var (
	ApiConfig      Config
	AllowedRoles   []models.Roles
	ApproverTeams  []models.Team
	AdminTeams     []models.Team
	ClusterNames   []string
	ClusterConfigs = make(map[string]ClusterConfig)
	apiNamespace   = os.Getenv("API_NAMESPACE")
	localClientset *kubernetes.Clientset
	gvr            = schema.GroupVersionResource{
		Group:    "jit.kubejit.io",
		Version:  "v1",
		Resource: "jitrequests",
	}
	dynamicClientCache sync.Map
	jitGroupsCache     sync.Map
)

// init loads clusters, roles and approver teams from configMap into global vars
func init() {
	var config *rest.Config
	var err error

	// Load in-cluster kube config
	config, err = rest.InClusterConfig()
	if err != nil {
		// Load kubeconfig from environment variable
		kubeconfigPath := os.Getenv("KUBECONFIG")
		if kubeconfigPath == "" {
			kubeconfigPath = os.Getenv("HOME") + "/.kube/config"
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			panic(err.Error())
		}
	}

	localClientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// Load ConfigMap of clusters from local file system
	configPath := os.Getenv("CONFIG_MOUNT_PATH")
	configData, err := os.ReadFile(configPath + "/apiConfig.yaml")
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	// Parse ConfigMap data to global ApiConfig
	err = yaml.Unmarshal([]byte(configData), &ApiConfig)
	if err != nil {
		log.Fatalf("Error unmarshalling config data: %v", err)
	}

	// Get cluster tokens and add to global cluster maps
	fmt.Println("\nSuccessfully loaded config for clusters:")
	for _, cluster := range ApiConfig.Clusters {
		fmt.Printf("- %s (Type: %s)\n", cluster.Name, cluster.Type)
		if cluster.Type == "generic" {
			cluster.Token = getTokenFromSecret(cluster.TokenSecret)
		}
		ClusterConfigs[cluster.Name] = cluster

		// Append cluster name to ClusterNames
		ClusterNames = append(ClusterNames, cluster.Name)
	}
	fmt.Print("\n")

	// Load allowedRoles and approverTeams
	AllowedRoles = ApiConfig.AllowedRoles
	ApproverTeams = ApiConfig.ApproverTeams
	AdminTeams = ApiConfig.AdminTeams

	fmt.Println("\nAllowed roles:")
	for _, role := range AllowedRoles {
		fmt.Printf("- %s\n", role.Name)
	}
	fmt.Println("\nApprover teams:")
	for _, team := range ApproverTeams {
		fmt.Printf("- %s (ID: %s)\n", team.Name, team.ID)
	}
	fmt.Println("\nAdmin teams:")
	for _, team := range AdminTeams {
		fmt.Printf("- %s (ID: %s)\n", team.Name, team.ID)
	}

}

// getTokenFromSecret gets and returns the sa token from a k8s secret during init of kube configs
func getTokenFromSecret(secretName string) string {
	secret, err := localClientset.CoreV1().Secrets(apiNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		log.Printf("Error getting secret %s: %v", secretName, err)
		panic(err.Error())
	}
	return string(secret.Data["token"])
}

// createDynamicClient creates and returns a dynamic client based on cluster in request
func createDynamicClient(req models.RequestData) *dynamic.DynamicClient {
	// Check if the dynamic client for the cluster is already cached
	if cached, exists := dynamicClientCache.Load(req.ClusterName); exists {
		cachedClient := cached.(*CachedClient)
		currentTime := time.Now().Unix()

		// Check if the token is expired
		if cachedClient.TokenExpires > currentTime {
			log.Printf("Using cached dynamic client for cluster: %s. Expires: %d", req.ClusterName, cachedClient.TokenExpires)
			return cachedClient.Client
		}

		log.Printf("Token expired for cluster: %s, refreshing client", req.ClusterName)
		InvalidateJitGroupsCache(req.ClusterName) // Invalidate the JitGroups cache
	}

	// Get the cluster configuration
	selectedCluster := ClusterConfigs[req.ClusterName]

	var restConfig *rest.Config
	var err error
	var tokenExpires int64

	// Use a switch statement to handle different cluster types
	switch selectedCluster.Type {
	case "gke":
		// GKE-specific logic
		log.Printf("Using Google Cloud SDK to access GKE cluster: %s", req.ClusterName)

		// Initialize Google credentials
		ctx := context.Background()
		credentials, err := google.FindDefaultCredentials(ctx, container.CloudPlatformScope)
		if err != nil {
			log.Fatalf("Failed to get default credentials: %v", err)
		}
		tokenSource := credentials.TokenSource

		client, err := containerapiv1.NewClusterManagerClient(ctx)
		if err != nil {
			log.Fatalf("Failed to create GKE client: %v", err)
		}
		defer client.Close()

		location := selectedCluster.Region
		clusterName := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", selectedCluster.ProjectID, location, selectedCluster.Name)

		clusterReq := &containerpb.GetClusterRequest{Name: clusterName}
		cluster, err := client.GetCluster(ctx, clusterReq)
		if err != nil {
			log.Fatalf("Failed to get GKE cluster details: %v", err)
		}

		clusterEndpoint := cluster.Endpoint
		caCertificate := cluster.MasterAuth.ClusterCaCertificate
		decodedCACertificate, err := base64.StdEncoding.DecodeString(caCertificate)
		if err != nil {
			log.Fatalf("Failed to decode CA certificate: %v", err)
		}

		token, err := tokenSource.Token()
		if err != nil {
			log.Fatalf("Failed to get OAuth2 token: %v", err)
		}

		restConfig = &rest.Config{
			Host:        "https://" + clusterEndpoint,
			BearerToken: token.AccessToken,
			TLSClientConfig: rest.TLSClientConfig{
				CAData: decodedCACertificate,
			},
		}

		// Set token expiration time (e.g., 5 minutes before actual expiration)
		tokenExpires = token.Expiry.Unix() - 300

	case "aks":
		// AKS-specific logic
		log.Printf("Using Azure SDK to access AKS cluster: %s", req.ClusterName)

		// Create Azure credentials
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			log.Fatalf("Failed to create Azure credential: %v", err)
		}

		// Create an AKS AdminCredentialsClient
		adminClient, err := armcontainerservice.NewManagedClustersClient(selectedCluster.ProjectID, cred, nil)
		if err != nil {
			log.Fatalf("Failed to create AKS AdminCredentialsClient: %v", err)
		}

		// Fetch the kubeconfig for the AKS cluster
		kubeconfigResp, err := adminClient.ListClusterUserCredentials(context.Background(), selectedCluster.Region, selectedCluster.Name, nil)
		if err != nil {
			log.Fatalf("Failed to get AKS cluster user credentials: %v", err)
		}

		// Parse the kubeconfig
		kubeconfigData := kubeconfigResp.Kubeconfigs[0].Value
		config, err := clientcmd.Load(kubeconfigData)
		if err != nil {
			log.Fatalf("Failed to parse kubeconfig: %v", err)
		}

		// Extract the API server URL and CA certificate
		cluster := config.Clusters[config.Contexts[config.CurrentContext].Cluster]
		clusterEndpoint := cluster.Server
		caCertificate := cluster.CertificateAuthorityData

		// Get an AAD token for the AKS API server
		token, err := cred.GetToken(context.Background(), policy.TokenRequestOptions{
			Scopes: []string{"6dae42f8-4368-4678-94ff-3960e28e3630/.default"}, // AKS API scope
		})
		if err != nil {
			log.Fatalf("Failed to get AAD token: %v", err)
		}

		// Configure the Kubernetes client
		restConfig = &rest.Config{
			Host:        clusterEndpoint,
			BearerToken: token.Token,
			TLSClientConfig: rest.TLSClientConfig{
				CAData: caCertificate,
			},
		}

		// Set token expiration time (e.g., 1 hour)
		tokenExpires = time.Now().Add(1 * time.Hour).Unix()

	default:
		// Generic cluster logic
		log.Printf("Using generic configuration for cluster: %s", req.ClusterName)

		apiServerURL := selectedCluster.Host
		saToken := selectedCluster.Token
		caData, err := base64.StdEncoding.DecodeString(selectedCluster.CA)
		if err != nil {
			log.Fatalf("Failed to decode CA certificate: %v\n", err)
		}

		restConfig = &rest.Config{
			Host:        apiServerURL,
			BearerToken: saToken,
			TLSClientConfig: rest.TLSClientConfig{
				Insecure: selectedCluster.Insecure,
				CAData:   caData,
			},
		}

		// Non-GKE clusters don't use token expiration
		tokenExpires = time.Now().Add(24 * time.Hour).Unix() // Arbitrary long expiration
	}

	// Create the dynamic client
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		log.Fatalf("Failed to create k8s client: %v", err)
	}

	// Cache the dynamic client with expiration
	dynamicClientCache.Store(req.ClusterName, &CachedClient{
		Client:       dynamicClient,
		TokenExpires: tokenExpires,
	})
	log.Printf("Cached dynamic client for cluster: %s", req.ClusterName)

	return dynamicClient
}

// CreateK8sObject creates the k8s JitRequest object on target cluster
func CreateK8sObject(req models.RequestData, approverName string) error {

	// Convert time.Time to metav1.Time
	startTime := metav1.NewTime(req.StartDate)
	endTime := metav1.NewTime(req.EndDate)

	// Generate signed URL for callback
	baseUrl := os.Getenv("CALLBACK_HOST_OVERRIDE")
	callbackBaseURL := baseUrl + "/kube-jit-api/k8s-callback"
	signedURL, err := utils.GenerateSignedURL(callbackBaseURL, req.EndDate)
	if err != nil {
		log.Printf("Failed to generate signed URL: %v\n", err)
		return err
	}

	// jitRequest payload
	jitRequest := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "jit.kubejit.io/v1",
			"kind":       "JitRequest",
			"metadata": map[string]interface{}{
				"name": fmt.Sprintf("jit-%d", req.ID),
			},
			"spec": map[string]interface{}{
				"user":          req.Username,
				"approver":      approverName,
				"justification": req.Justification,
				"userEmails":    req.Users,
				"clusterRole":   req.RoleName,
				"namespaces":    req.Namespaces,
				"ticketID":      fmt.Sprintf("%d", req.ID),
				"startTime":     startTime,
				"endTime":       endTime,
				"callbackUrl":   signedURL,
			},
		},
	}

	// Create client for selected cluster
	dynamicClient := createDynamicClient(req)

	// Create jitRequest
	log.Printf("Creating k8s object for request ID: %d", req.ID)
	_, err = dynamicClient.Resource(gvr).Create(context.TODO(), jitRequest, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Error creating k8s object for request ID %d: %v", req.ID, err)
		return err
	}

	log.Printf("Successfully created k8s object for request ID: %d", req.ID)
	return nil
}

// GetJitGroups retrieves the JitGroups for a cluster, using cache if available
func GetJitGroups(clusterName string) (*unstructured.Unstructured, error) {
	// Check if the cache exists
	if cached, exists := jitGroupsCache.Load(clusterName); exists {
		cache := cached.(*JitGroupsCache)
		currentTime := time.Now().Unix()

		// Check if the cache is still valid
		if cache.ExpiresAt > currentTime {
			log.Printf("Using cached JitGroups for cluster: %s", clusterName)
			return cache.JitGroups, nil
		}

		log.Printf("JitGroups cache expired for cluster: %s, refreshing", clusterName)
	}

	// Fetch JitGroups from the cluster
	jitGroups, err := fetchJitGroupsFromCluster(clusterName)
	if err != nil {
		return nil, err
	}

	// Cache the JitGroups with a 10-minute expiration
	expiration := time.Now().Add(10 * time.Minute).Unix()
	jitGroupsCache.Store(clusterName, &JitGroupsCache{
		JitGroups: jitGroups,
		ExpiresAt: expiration,
	})

	log.Printf("Cached JitGroups for cluster: %s", clusterName)
	return jitGroups, nil
}

func fetchJitGroupsFromCluster(clusterName string) (*unstructured.Unstructured, error) {
	dynamicClient := createDynamicClient(models.RequestData{ClusterName: clusterName})

	// Query the JitGroups CRD
	jitGroups, err := dynamicClient.Resource(schema.GroupVersionResource{
		Group:    "jit.kubejit.io",
		Version:  "v1",
		Resource: "jitgroupcaches",
	}).Get(context.TODO(), "jitgroupcache", metav1.GetOptions{}) // Static name for the JitGroupCache object is 'jitgroupcache
	if err != nil {
		log.Printf("Error fetching JitGroups for cluster %s: %v", clusterName, err)
		return nil, fmt.Errorf("failed to fetch JitGroups for cluster %s", clusterName)
	}

	return jitGroups, nil
}

// InvalidateJitGroupsCache invalidates the JitGroups cache for a specific cluster
func InvalidateJitGroupsCache(clusterName string) {
	log.Printf("Invalidating JitGroups cache for cluster: %s", clusterName)
	jitGroupsCache.Delete(clusterName)
}

// ValidateNamespaces checks if the given namespaces are valid for the cluster
func ValidateNamespaces(clusterName string, namespaces []string) (map[string]string, error) {
	// Fetch JitGroups from the cache or the cluster
	jitGroups, err := GetJitGroups(clusterName)
	if err != nil {
		return nil, fmt.Errorf("error fetching JitGroups: %v", err)
	}

	// Parse the JitGroups object
	namespaceAnnotations := make(map[string]string)
	groups, _, _ := unstructured.NestedSlice(jitGroups.Object, "spec", "groups")

	for _, namespace := range namespaces {
		found := false
		for _, group := range groups {
			// Convert the group to a JitGroup struct
			groupMap, ok := group.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid group format in JitGroups")
			}

			jitGroup := JitGroup{
				GroupID:   groupMap["groupID"].(string),
				Namespace: groupMap["namespace"].(string),
			}

			if jitGroup.Namespace == namespace {
				namespaceAnnotations[namespace] = jitGroup.GroupID
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("namespace %s does not exist in JitGroups", namespace)
		}
	}

	return namespaceAnnotations, nil
}
