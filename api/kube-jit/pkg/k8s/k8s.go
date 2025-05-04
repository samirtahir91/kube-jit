package k8s

import (
	"context"
	"encoding/base64"
	"fmt"
	"kube-jit/internal/models"
	"kube-jit/pkg/utils"
	"os"
	"sync"
	"time"

	containerapiv1 "cloud.google.com/go/container/apiv1"
	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
	"go.uber.org/zap"
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

var logger *zap.Logger

func InitLogger(l *zap.Logger) {
	logger = l
}

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
	GroupID   string `json:"groupID"`
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

// InitK8sConfig loads clusters, roles and approver teams from configMap into global vars
func InitK8sConfig() {
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
		logger.Fatal("Error reading config file", zap.Error(err))
	}

	// Parse ConfigMap data to global ApiConfig
	err = yaml.Unmarshal([]byte(configData), &ApiConfig)
	if err != nil {
		logger.Fatal("Error unmarshalling config data", zap.Error(err))
	}

	// Get cluster tokens and add to global cluster maps
	logger.Info("Successfully loaded config for clusters",
		zap.Strings("clusters", ClusterNames),
	)
	for _, cluster := range ApiConfig.Clusters {
		logger.Info("Loaded cluster", zap.String("name", cluster.Name), zap.String("type", cluster.Type))
		if cluster.Type == "generic" {
			cluster.Token = getTokenFromSecret(cluster.TokenSecret)
		}
		ClusterConfigs[cluster.Name] = cluster
		ClusterNames = append(ClusterNames, cluster.Name)
	}

	AllowedRoles = ApiConfig.AllowedRoles
	ApproverTeams = ApiConfig.ApproverTeams
	AdminTeams = ApiConfig.AdminTeams

	logger.Info("Allowed roles loaded", zap.Int("count", len(AllowedRoles)))
	for _, role := range AllowedRoles {
		logger.Info("Allowed role", zap.String("name", role.Name))
	}
	logger.Info("Approver teams loaded", zap.Int("count", len(ApproverTeams)))
	for _, team := range ApproverTeams {
		logger.Info("Approver team", zap.String("name", team.Name), zap.String("id", team.ID))
	}
	logger.Info("Admin teams loaded", zap.Int("count", len(AdminTeams)))
	for _, team := range AdminTeams {
		logger.Info("Admin team", zap.String("name", team.Name), zap.String("id", team.ID))
	}

	// Cache dynamic clients for all clusters on startup
	for _, clusterName := range ClusterNames {
		req := models.RequestData{ClusterName: clusterName}
		go func(r models.RequestData) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("Failed to cache dynamic client for cluster", zap.String("cluster", r.ClusterName), zap.Any("error", err))
				}
			}()
			createDynamicClient(r)
		}(req)
	}
}

// getTokenFromSecret gets and returns the sa token from a k8s secret during init of kube configs
func getTokenFromSecret(secretName string) string {
	secret, err := localClientset.CoreV1().Secrets(apiNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		logger.Error("Error getting secret", zap.String("secret", secretName), zap.Error(err))
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
			logger.Info("Using cached dynamic client for cluster", zap.String("cluster", req.ClusterName), zap.Int64("expires", cachedClient.TokenExpires))
			return cachedClient.Client
		}

		logger.Info("Token expired for cluster, refreshing client", zap.String("cluster", req.ClusterName))
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
		logger.Info("Using Google Cloud SDK to access GKE cluster", zap.String("cluster", req.ClusterName))
		// GKE-specific logic
		ctx := context.Background()
		credentials, err := google.FindDefaultCredentials(ctx, container.CloudPlatformScope)
		if err != nil {
			logger.Fatal("Failed to get default credentials", zap.Error(err))
		}
		tokenSource := credentials.TokenSource

		client, err := containerapiv1.NewClusterManagerClient(ctx)
		if err != nil {
			logger.Fatal("Failed to create GKE client", zap.Error(err))
		}
		defer client.Close()

		location := selectedCluster.Region
		clusterName := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", selectedCluster.ProjectID, location, selectedCluster.Name)
		cluster, err := client.GetCluster(ctx, &containerpb.GetClusterRequest{Name: clusterName})
		if err != nil {
			logger.Fatal("Failed to get GKE cluster details", zap.Error(err))
		}

		clusterEndpoint := cluster.Endpoint
		caCertificate := cluster.MasterAuth.ClusterCaCertificate
		decodedCACertificate, err := base64.StdEncoding.DecodeString(caCertificate)
		if err != nil {
			logger.Fatal("Failed to decode CA certificate", zap.Error(err))
		}

		token, err := tokenSource.Token()
		if err != nil {
			logger.Fatal("Failed to get OAuth2 token", zap.Error(err))
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
		logger.Info("Using Azure SDK to access AKS cluster", zap.String("cluster", req.ClusterName))
		// AKS-specific logic
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			logger.Fatal("Failed to create Azure credential", zap.Error(err))
		}
		adminClient, err := armcontainerservice.NewManagedClustersClient(selectedCluster.ProjectID, cred, nil)
		if err != nil {
			logger.Fatal("Failed to create AKS AdminCredentialsClient", zap.Error(err))
		}
		kubeconfigResp, err := adminClient.ListClusterUserCredentials(context.Background(), selectedCluster.Region, selectedCluster.Name, nil)
		if err != nil {
			logger.Fatal("Failed to get AKS cluster user credentials", zap.Error(err))
		}
		kubeconfigData := kubeconfigResp.Kubeconfigs[0].Value
		config, err := clientcmd.Load(kubeconfigData)
		if err != nil {
			logger.Fatal("Failed to parse kubeconfig", zap.Error(err))
		}
		cluster := config.Clusters[config.Contexts[config.CurrentContext].Cluster]
		clusterEndpoint := cluster.Server
		caCertificate := cluster.CertificateAuthorityData

		token, err := cred.GetToken(context.Background(), policy.TokenRequestOptions{
			Scopes: []string{"6dae42f8-4368-4678-94ff-3960e28e3630/.default"},
		})
		if err != nil {
			logger.Fatal("Failed to get AAD token", zap.Error(err))
		}

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
		logger.Info("Using generic configuration for cluster", zap.String("cluster", req.ClusterName))
		// Generic cluster logic
		apiServerURL := selectedCluster.Host
		saToken := selectedCluster.Token
		caData, err := base64.StdEncoding.DecodeString(selectedCluster.CA)
		if err != nil {
			logger.Fatal("Failed to decode CA certificate", zap.Error(err))
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
		logger.Fatal("Failed to create k8s client", zap.Error(err))
	}

	// Cache the dynamic client with expiration
	dynamicClientCache.Store(req.ClusterName, &CachedClient{
		Client:       dynamicClient,
		TokenExpires: tokenExpires,
	})
	logger.Info("Cached dynamic client for cluster", zap.String("cluster", req.ClusterName))
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
		logger.Error("Failed to generate signed URL", zap.Error(err))
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
	logger.Info("Creating k8s object for request", zap.Uint("requestID", req.ID))
	_, err = dynamicClient.Resource(gvr).Create(context.TODO(), jitRequest, metav1.CreateOptions{})
	if err != nil {
		logger.Error("Error creating k8s object for request", zap.Uint("requestID", req.ID), zap.Error(err))
		return err
	}
	logger.Info("Successfully created k8s object for request", zap.Uint("requestID", req.ID))
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
			logger.Info("Using cached JitGroups for cluster", zap.String("cluster", clusterName))
			return cache.JitGroups, nil
		}

		logger.Info("JitGroups cache expired for cluster, refreshing", zap.String("cluster", clusterName))
	}

	// Fetch JitGroups from the cluster
	jitGroups, err := fetchJitGroupsFromCluster(clusterName)
	if err != nil {
		logger.Error("Error fetching JitGroups for cluster", zap.String("cluster", clusterName), zap.Error(err))
		return nil, fmt.Errorf("failed to fetch JitGroups for cluster %s", clusterName)
	}

	// Cache the JitGroups with a 10-minute expiration
	expiration := time.Now().Add(10 * time.Minute).Unix()
	jitGroupsCache.Store(clusterName, &JitGroupsCache{
		JitGroups: jitGroups,
		ExpiresAt: expiration,
	})
	logger.Info("Cached JitGroups for cluster", zap.String("cluster", clusterName))
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
		logger.Error("Error fetching JitGroups from cluster", zap.String("cluster", clusterName), zap.Error(err))
		return nil, fmt.Errorf("failed to fetch JitGroups for cluster %s", clusterName)
	}
	return jitGroups, nil
}

// InvalidateJitGroupsCache invalidates the JitGroups cache for a specific cluster
func InvalidateJitGroupsCache(clusterName string) {
	logger.Info("Invalidating JitGroups cache for cluster", zap.String("cluster", clusterName))
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
