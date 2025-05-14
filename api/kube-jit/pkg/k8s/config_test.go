package k8s

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func init() {
	logger = zap.NewNop()
}

func TestInitK8sConfig_LoadsConfigAndSecrets(t *testing.T) {
	// Setup temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "apiConfig.yaml")
	configYaml := `
clusters:
  - name: test-cluster
    host: https://fake
    ca: ""
    insecure: true
    tokenSecret: test-secret
    type: generic
allowedRoles:
  - name: admin
platformApproverTeams:
  - name: team1
    id: t1
adminTeams:
  - name: team2
    id: t2
`
	require.NoError(t, os.WriteFile(configPath, []byte(configYaml), 0644))

	// Set required env vars
	os.Setenv("CONFIG_MOUNT_PATH", tmpDir)
	os.Setenv("API_NAMESPACE", "default")
	os.Setenv("CALLBACK_HOST_OVERRIDE", "localhost")

	// Setup fake k8s client with a secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"token": []byte("fake-token"),
		},
	}
	clientset := fake.NewSimpleClientset(secret)
	localClientset = clientset
	apiNamespace = "default"

	// Clear global state
	ClusterConfigs = map[string]ClusterConfig{}
	ClusterNames = []string{}
	AllowedRoles = nil
	PlatformApproverTeams = nil
	AdminTeams = nil

	// Run
	runClusterInitAsync = false
	defer func() { runClusterInitAsync = true }()
	InitK8sConfig()

	// Assertions
	assert.Contains(t, ClusterConfigs, "test-cluster")
	assert.Equal(t, "fake-token", ClusterConfigs["test-cluster"].Token)
	assert.Equal(t, []string{"test-cluster"}, ClusterNames)
	assert.Equal(t, "admin", AllowedRoles[0].Name)
	assert.Equal(t, "team1", PlatformApproverTeams[0].Name)
	assert.Equal(t, "team2", AdminTeams[0].Name)
}

func TestGetTokenFromSecret_Error(t *testing.T) {
	// Setup fake clientset with no secret
	clientset := fake.NewSimpleClientset()
	localClientset = clientset
	apiNamespace = "default"

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic when secret not found")
		}
	}()

	getTokenFromSecret("nonexistent-secret")
}
