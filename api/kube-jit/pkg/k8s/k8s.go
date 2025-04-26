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

	container "cloud.google.com/go/container/apiv1"
	"cloud.google.com/go/container/apiv1/containerpb"
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
	Type        string `yaml:"type"`      // e.g., "gke" or "generic"
	ProjectID   string `yaml:"projectID"` // GCP project ID for GKE clusters
	Region      string `yaml:"region"`    // Region for GKE clusters
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
		if cluster.Type != "gke" {
			cluster.Token = getTokenFromSecret(cluster.TokenSecret)
		}
		ClusterConfigs[cluster.Name] = cluster
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
		fmt.Printf("- %s (ID: %d)\n", team.Name, team.ID)
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
	if client, exists := dynamicClientCache.Load(req.ClusterName); exists {
		log.Printf("Using cached dynamic client for cluster: %s", req.ClusterName)
		return client.(*dynamic.DynamicClient)
	}

	// Get the cluster configuration
	selectedCluster := ClusterConfigs[req.ClusterName]

	var restConfig *rest.Config
	var err error

	if selectedCluster.Type == "gke" {
		// Use Google Cloud SDK to fetch cluster details
		log.Printf("Using Google Cloud SDK to access GKE cluster: %s", req.ClusterName)

		// Set up the GKE client
		ctx := context.Background()
		client, err := container.NewClusterManagerClient(ctx)
		if err != nil {
			log.Fatalf("Failed to create GKE client: %v", err)
		}
		defer client.Close()

		// Fetch cluster details using ProjectID and Region from the config
		clusterReq := &containerpb.GetClusterRequest{
			ProjectId: selectedCluster.ProjectID,
			Zone:      selectedCluster.Region,
			ClusterId: selectedCluster.Name,
		}

		cluster, err := client.GetCluster(ctx, clusterReq)
		if err != nil {
			log.Fatalf("Failed to get GKE cluster details: %v", err)
		}

		// Extract the cluster endpoint and CA certificate
		clusterEndpoint := cluster.Endpoint
		caCertificate := cluster.MasterAuth.ClusterCaCertificate

		// Generate the rest.Config
		restConfig = &rest.Config{
			Host: "https://" + clusterEndpoint,
			TLSClientConfig: rest.TLSClientConfig{
				CAData: []byte(caCertificate),
			},
		}
	} else {
		// Use custom config for non-GKE clusters
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
	}

	// Create the dynamic client
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		log.Printf("Failed to create k8s client: %v\n", err)
		panic(err.Error())
	}

	// Cache the dynamic client
	dynamicClientCache.Store(req.ClusterName, dynamicClient)
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
