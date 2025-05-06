package k8s

import (
	"context"
	"encoding/base64"
	"fmt"
	"kube-jit/internal/models"
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
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	// imports...
)

var dynamicClientCache sync.Map

type CachedClient struct {
	Client       *dynamic.DynamicClient
	TokenExpires int64
}

// createDynamicClient creates and returns a dynamic client based on cluster in request
// It caches the client and token expiration time to avoid creating a new client for each request
// It also invalidates the JitGroups cache if the token is expired
// It uses the cluster type to determine how to create the client (GKE, AKS, or generic)
// It uses the Google Cloud SDK for GKE and Azure SDK for AKS
// It uses the Kubernetes client-go library for generic clusters
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
	case "gke": // Google Kubernetes Engine
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

	case "aks": // Azure Kubernetes Service
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

	default: // Generic Kubernetes cluster
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
