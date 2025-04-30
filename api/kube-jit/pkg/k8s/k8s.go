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
	"golang.org/x/oauth2"
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

var (
	ApiConfig      Config
	AllowedRoles   []models.Roles
	ApproverTeams  []models.Team
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
	tokenSource        oauth2.TokenSource
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
	fmt.Println("\nAllowed roles:")
	for _, role := range AllowedRoles {
		fmt.Printf("- %s\n", role.Name)
	}
	fmt.Println("\nApprover teams:")
	for _, team := range ApproverTeams {
		fmt.Printf("- %s (ID: %s)\n", team.Name, team.ID)
	}

	// Initialize token source for GKE clusters
	ctx := context.Background()
	credentials, err := google.FindDefaultCredentials(ctx, container.CloudPlatformScope)
	if err != nil {
		log.Fatalf("Failed to get default credentials: %v", err)
	}
	tokenSource = credentials.TokenSource
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
	}

	// Get the cluster configuration
	selectedCluster := ClusterConfigs[req.ClusterName]

	var restConfig *rest.Config
	var err error
	var tokenExpires int64

	if selectedCluster.Type == "gke" {
		// Use Google Cloud SDK to fetch cluster details
		log.Printf("Using Google Cloud SDK to access GKE cluster: %s", req.ClusterName)

		ctx := context.Background()
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
	} else {
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
