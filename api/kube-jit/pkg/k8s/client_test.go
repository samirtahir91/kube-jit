package k8s

import (
	"sync"
	"testing"
	"time"

	"kube-jit/internal/models"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
)

func init() {
	logger = zap.NewNop()
}

// Mock invalidateJitGroupsCacheTest to track calls
var invalidateCalled bool

var invalidateJitGroupsCacheTest = func(cluster string) {
	invalidateCalled = true
}

// Patch dynamic.NewForConfig to return a fake client
var fakeClient dynamic.Interface

func TestCreateDynamicClient_CacheReuse(t *testing.T) {
	// Reset cache and state
	dynamicClientCache = sync.Map{}
	invalidateCalled = false

	// Create a fake client with a scheme
	scheme := runtime.NewScheme()
	fakeClient = fake.NewSimpleDynamicClient(scheme)

	// Insert a cached client with future expiry
	client := &CachedClient{
		Client:       fakeClient, // Ensure CachedClient.Client is of type dynamic.Interface
		TokenExpires: time.Now().Add(10 * time.Minute).Unix(),
	}
	dynamicClientCache.Store("test-cluster", client)

	// Patch InvalidateJitGroupsCache to avoid conflicts
	origInvalidate := InvalidateJitGroupsCache
	InvalidateJitGroupsCache = invalidateJitGroupsCacheTest
	defer func() { InvalidateJitGroupsCache = origInvalidate }()

	req := models.RequestData{ClusterName: "test-cluster"}
	got := createDynamicClient(req)
	assert.Equal(t, fakeClient, got)
	assert.False(t, invalidateCalled, "Cache should not be invalidated if not expired")
}

func TestCreateDynamicClient_CacheExpired(t *testing.T) {
	// Reset cache and state
	dynamicClientCache = sync.Map{}
	invalidateCalled = false

	// Create a fake client with a scheme
	scheme := runtime.NewScheme()
	fakeClient = fake.NewSimpleDynamicClient(scheme)

	// Insert a cached client with past expiry
	client := &CachedClient{
		Client:       fakeClient, // Ensure CachedClient.Client is of type dynamic.Interface
		TokenExpires: time.Now().Add(-10 * time.Minute).Unix(),
	}
	dynamicClientCache.Store("test-cluster", client)

	// Patch InvalidateJitGroupsCache to avoid conflicts
	origInvalidate := InvalidateJitGroupsCache
	InvalidateJitGroupsCache = invalidateJitGroupsCacheTest
	defer func() { InvalidateJitGroupsCache = origInvalidate }()

	// Patch ClusterConfigs for generic cluster
	ClusterConfigs = map[string]ClusterConfig{
		"test-cluster": {
			Type:     "generic",
			Host:     "https://fake",
			Token:    "token",
			CA:       "",
			Insecure: true,
		},
	}

	req := models.RequestData{ClusterName: "test-cluster"}
	// Patch dynamicNewForConfig to return fakeClient
	origDynamicNewForConfig := dynamicNewForConfig
	dynamicNewForConfig = func(*rest.Config) (dynamic.Interface, error) { return fakeClient, nil }
	defer func() { dynamicNewForConfig = origDynamicNewForConfig }()

	got := createDynamicClient(req)
	assert.Equal(t, fakeClient, got)
	assert.True(t, invalidateCalled, "Cache should be invalidated if expired")
}
