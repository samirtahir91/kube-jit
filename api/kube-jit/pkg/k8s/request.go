package k8s

import (
	"context"
	"fmt"
	"kube-jit/internal/models"
	"kube-jit/pkg/utils"

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
func CreateK8sObject(req models.RequestData, approverName string) error {
	// Convert time.Time to metav1.Time
	startTime := metav1.NewTime(req.StartDate)
	endTime := metav1.NewTime(req.EndDate)

	// Generate signed URL for callback
	callbackBaseURL := callbackHostOverride + "/kube-jit-api/k8s-callback"
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
				"user":           req.Username,
				"approver":       approverName,
				"justification":  req.Justification,
				"userEmails":     req.Users,
				"requestorEmail": req.Email,
				"clusterRole":    req.RoleName,
				"namespaces":     req.Namespaces,
				"ticketID":       fmt.Sprintf("%d", req.ID),
				"startTime":      startTime,
				"endTime":        endTime,
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
