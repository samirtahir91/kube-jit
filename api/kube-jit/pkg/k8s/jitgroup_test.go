package k8s

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func makeFakeJitGroups(namespaces []string) *unstructured.Unstructured {
	groups := make([]interface{}, len(namespaces))
	for i, ns := range namespaces {
		groups[i] = map[string]interface{}{
			"groupID":   "gid-" + ns,
			"namespace": ns,
			"groupName": "gname-" + ns,
		}
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"groups": groups,
			},
		},
	}
}

func TestValidateNamespaces_AllValid(t *testing.T) {
	// Patch GetJitGroups to return a fake object
	origGetJitGroups := GetJitGroups
	GetJitGroups = func(clusterName string) (*unstructured.Unstructured, error) {
		return makeFakeJitGroups([]string{"ns1", "ns2"}), nil
	}
	defer func() { GetJitGroups = origGetJitGroups }()

	result, err := ValidateNamespaces("test-cluster", []string{"ns1", "ns2"})
	assert.NoError(t, err)
	assert.Equal(t, "gid-ns1", result["ns1"].GroupID)
	assert.Equal(t, "gname-ns2", result["ns2"].GroupName)
}

func TestValidateNamespaces_InvalidNamespace(t *testing.T) {
	origGetJitGroups := GetJitGroups
	GetJitGroups = func(clusterName string) (*unstructured.Unstructured, error) {
		return makeFakeJitGroups([]string{"ns1"}), nil
	}
	defer func() { GetJitGroups = origGetJitGroups }()

	_, err := ValidateNamespaces("test-cluster", []string{"ns1", "nsX"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "namespace nsX does not exist")
}

func TestValidateNamespaces_GetJitGroupsError(t *testing.T) {
	origGetJitGroups := GetJitGroups
	GetJitGroups = func(clusterName string) (*unstructured.Unstructured, error) {
		return nil, errors.New("cluster error")
	}
	defer func() { GetJitGroups = origGetJitGroups }()

	_, err := ValidateNamespaces("test-cluster", []string{"ns1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error fetching JitGroups")
}

func TestInvalidateJitGroupsCache(t *testing.T) {
	jitGroupsCache.Store("foo", &JitGroupsCache{ExpiresAt: 123})
	InvalidateJitGroupsCache("foo")
	_, exists := jitGroupsCache.Load("foo")
	assert.False(t, exists)
}
