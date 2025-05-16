package k8s

import (
	"errors"
	"testing"
	"time"

	"kube-jit/internal/models"
	"kube-jit/pkg/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestCreateK8sObject_Success(t *testing.T) {
	// Patch CreateDynamicClient and utils.GenerateSignedURL
	origCreateDynamicClient := createDynamicClient
	origGenerateSignedURL := utils.GenerateSignedURL
	defer func() {
		createDynamicClient = origCreateDynamicClient
		utils.GenerateSignedURL = origGenerateSignedURL
	}()

	utils.GenerateSignedURL = func(base string, expiry time.Time) (string, error) {
		return "http://signed-url", nil
	}

	scheme := runtime.NewScheme()
	fakeClient := fake.NewSimpleDynamicClient(scheme)
	createDynamicClient = func(req models.RequestData) dynamic.Interface {
		return fakeClient
	}

	req := models.RequestData{
		Username:      "alice",
		RoleName:      "admin",
		Namespaces:    []string{"ns1"},
		Users:         []string{"alice@example.com"},
		Email:         "alice@example.com",
		Justification: "testing",
		StartDate:     time.Now(),
		EndDate:       time.Now().Add(time.Hour),
	}

	err := CreateK8sObject(req, "approver")
	require.NoError(t, err)
}

func TestCreateK8sObject_SignedUrlError(t *testing.T) {
	origGenerateSignedURL := utils.GenerateSignedURL
	defer func() { utils.GenerateSignedURL = origGenerateSignedURL }()

	utils.GenerateSignedURL = func(base string, expiry time.Time) (string, error) {
		return "", errors.New("sign error")
	}

	req := models.RequestData{}
	err := CreateK8sObject(req, "approver")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sign error")
}

func TestCreateK8sObject_CreateError(t *testing.T) {
	origCreateDynamicClient := createDynamicClient
	origGenerateSignedURL := utils.GenerateSignedURL
	defer func() {
		createDynamicClient = origCreateDynamicClient
		utils.GenerateSignedURL = origGenerateSignedURL
	}()

	utils.GenerateSignedURL = func(base string, expiry time.Time) (string, error) {
		return "http://signed-url", nil
	}

	scheme := runtime.NewScheme()
	fakeClient := fake.NewSimpleDynamicClient(scheme)
	// Simulate error on Create
	fakeClient.PrependReactor("create", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New("create error")
	})

	createDynamicClient = func(req models.RequestData) dynamic.Interface {
		return fakeClient
	}

	req := models.RequestData{}
	err := CreateK8sObject(req, "approver")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "create error")
}

func TestCreateDynamicClient_Patch(t *testing.T) {
	origCreateDynamicClient := createDynamicClient
	createDynamicClient = func(req models.RequestData) dynamic.Interface {
		// Mock implementation
		return nil
	}
	defer func() { createDynamicClient = origCreateDynamicClient }()
}

// In utils package
var GenerateSignedURL = func(base string, expiry time.Time) (string, error) {
	return "http://signed-url", nil
}
