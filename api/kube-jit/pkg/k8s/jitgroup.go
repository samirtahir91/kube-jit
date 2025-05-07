package k8s

import (
	"context"
	"fmt"
	"kube-jit/internal/models"
	"sync"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var jitGroupsCache sync.Map

type JitGroup struct {
	GroupID   string `json:"groupID"`
	Namespace string `json:"namespace"`
	GroupName string `json:"groupName"`
}

type JitGroupsCache struct {
	JitGroups *unstructured.Unstructured
	ExpiresAt int64
}

// GetJitGroups retrieves the JitGroups for a cluster, using cache if available
// It checks if the cache is still valid and refreshes it if expired
// It fetches the JitGroups from the cluster if not in cache or if expired
// It returns the JitGroups object or an error if fetching fails
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

// fetchJitGroupsFromCluster fetches the JitGroups from the cluster using the dynamic client
// It uses the dynamic client to query the JitGroups CRD
// It returns the JitGroups object or an error if fetching fails
func fetchJitGroupsFromCluster(clusterName string) (*unstructured.Unstructured, error) {
	dynamicClient := createDynamicClient(models.RequestData{ClusterName: clusterName})

	// Query the JitGroups CRD
	jitGroups, err := dynamicClient.Resource(schema.GroupVersionResource{
		Group:    "jit.kubejit.io",
		Version:  "v1",
		Resource: "jitgroupcaches",
	}).Get(context.TODO(), jitgroupcacheName, metav1.GetOptions{}) // Static name for the JitGroupCache object is 'jitgroupcache
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
// It fetches the JitGroups for the cluster and checks if the namespaces exist in the JitGroups
// It returns a map of namespaces with their corresponding group IDs and names
// or an error if any namespace is invalid
func ValidateNamespaces(clusterName string, namespaces []string) (map[string]struct{ GroupID, GroupName string }, error) {
	jitGroups, err := GetJitGroups(clusterName)
	if err != nil {
		return nil, fmt.Errorf("error fetching JitGroups: %v", err)
	}

	namespaceAnnotations := make(map[string]struct{ GroupID, GroupName string })
	groups, _, _ := unstructured.NestedSlice(jitGroups.Object, "spec", "groups")

	for _, namespace := range namespaces {
		found := false
		for _, group := range groups {
			groupMap, ok := group.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid group format in JitGroups")
			}
			jitGroup := JitGroup{}
			if id, ok := groupMap["groupID"].(string); ok {
				jitGroup.GroupID = id
			}
			if ns, ok := groupMap["namespace"].(string); ok {
				jitGroup.Namespace = ns
			}
			if name, ok := groupMap["groupName"].(string); ok {
				jitGroup.GroupName = name
			}
			if jitGroup.Namespace == namespace {
				namespaceAnnotations[namespace] = struct{ GroupID, GroupName string }{
					GroupID:   jitGroup.GroupID,
					GroupName: jitGroup.GroupName,
				}
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
