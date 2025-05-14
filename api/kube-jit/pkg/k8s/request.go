package k8s

import (
	"context"
	"fmt"
	"kube-jit/internal/models"
	"kube-jit/pkg/utils"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	gvr = schema.GroupVersionResource{
		Group:    "jit.kubejit.io",
		Version:  "v1",
		Resource: "jitrequests",
	}
)

// CreateK8sObject creates the k8s JitRequest object on target cluster
// It uses the dynamic client to create the object
// It takes the request data and approver name as input
// It generates a signed URL for the callback and sets the start and end times
// It returns an error if the creation fails
var CreateK8sObject = func(req models.RequestData, approverName string) error {
	// Generate signed URL for callback
	callbackBaseURL := CallbackHostOverride + "/k8s-callback"
	signedURL, err := utils.GenerateSignedURL(callbackBaseURL, req.EndDate)
	if err != nil {
		logger.Error("Failed to generate signed URL", zap.Error(err))
		return err
	}

	// Convert []string to []interface{} for unstructured
	namespaces := make([]interface{}, len(req.Namespaces))
	for i, ns := range req.Namespaces {
		namespaces[i] = ns
	}
	users := make([]interface{}, len(req.Users))
	for i, u := range req.Users {
		users[i] = u
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
				"user":           req.Username,
				"approver":       approverName,
				"justification":  req.Justification,
				"userEmails":     users,
				"requestorEmail": req.Email,
				"clusterRole":    req.RoleName,
				"namespaces":     namespaces,
				"ticketID":       fmt.Sprintf("%d", req.ID),
				"startTime":      req.StartDate.Format(time.RFC3339),
				"endTime":        req.EndDate.Format(time.RFC3339),
				"callbackUrl":    signedURL,
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
