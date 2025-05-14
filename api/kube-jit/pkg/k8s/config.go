package k8s

import (
	"context"
	"kube-jit/internal/models"
	"kube-jit/pkg/utils"
	"os"

	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	apiNamespace        = utils.MustGetEnv("API_NAMESPACE")
	localClientset      kubernetes.Interface
	runClusterInitAsync = true
)

// Config represents the configuration for the API
type Config struct {
	Clusters              []ClusterConfig `yaml:"clusters"`
	AllowedRoles          []models.Roles  `yaml:"allowedRoles"`
	PlatformApproverTeams []models.Team   `yaml:"platformApproverTeams"`
	AdminTeams            []models.Team   `yaml:"adminTeams"`
}

// ClusterConfig represents the configuration for a cluster
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

// InitK8sConfig loads clusters, roles and approver teams from configMap into global vars
// and creates a dynamic client for each cluster
// It also loads the kubeconfig from the local file system or in-cluster config
// and creates a local clientset for the API server
// It is called during the initialization of the API in main.go
func InitK8sConfig() {
	var config *rest.Config
	var err error

	CallbackHostOverride = utils.MustGetEnv("CALLBACK_HOST_OVERRIDE")

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

	if localClientset == nil {
		localClientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			panic(err.Error())
		}
	}

	// Load ConfigMap of clusters from local file system
	configPath := utils.MustGetEnv("CONFIG_MOUNT_PATH")
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
	PlatformApproverTeams = ApiConfig.PlatformApproverTeams
	AdminTeams = ApiConfig.AdminTeams

	// Log loaded config
	logger.Info("Allowed roles loaded", zap.Int("count", len(AllowedRoles)))
	for _, role := range AllowedRoles {
		logger.Info("Allowed role", zap.String("name", role.Name))
	}
	logger.Info("Approver teams loaded", zap.Int("count", len(PlatformApproverTeams)))
	for _, team := range PlatformApproverTeams {
		logger.Info("Approver team", zap.String("name", team.Name), zap.String("id", team.ID))
	}
	logger.Info("Admin teams loaded", zap.Int("count", len(AdminTeams)))
	for _, team := range AdminTeams {
		logger.Info("Admin team", zap.String("name", team.Name), zap.String("id", team.ID))
	}

	// Cache dynamic clients for all clusters on startup
	for _, clusterName := range ClusterNames {
		req := models.RequestData{ClusterName: clusterName}
		if runClusterInitAsync { // for production
			go func(r models.RequestData) {
				defer func() {
					if err := recover(); err != nil {
						logger.Error("Failed to cache dynamic client for cluster", zap.String("cluster", r.ClusterName), zap.Any("error", err))
					}
				}()
				createDynamicClient(r)
			}(req)
		} else {
			// for testing
			createDynamicClient(req)
		}
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
