package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	jitv1 "kube-jit-operator/api/v1"
	"net/http"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"time"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// fetchJitRequest fetches and returns a JitRequest
func (r *JitRequestReconciler) fetchJitRequest(ctx context.Context, namespacedName types.NamespacedName) (*jitv1.JitRequest, error) {
	jitRequest := &jitv1.JitRequest{}
	err := r.Get(ctx, namespacedName, jitRequest)
	return jitRequest, err
}

// callbackToApi calls back to API with rejected status
func (r *JitRequestReconciler) callbackToApi(ctx context.Context, jitRequest *jitv1.JitRequest) error {
	l := log.FromContext(ctx)

	// Prepare data to send back to API
	message := jitRequest.Status.Message
	status := jitRequest.Status.State
	ticketID := jitRequest.Spec.TicketID
	callback := jitRequest.Spec.CallbackURL

	// Create the payload
	payload := map[string]string{
		"ticketID": ticketID,
		"status":   status,
		"message":  message,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		l.Error(err, "Failed to marshal payload")
		return err
	}

	// Send HTTP POST request to callback URL
	req, err := http.NewRequest("POST", callback, bytes.NewBuffer(payloadBytes))
	if err != nil {
		l.Error(err, "Failed to create HTTP request")
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		l.Error(err, "Failed to send HTTP request")
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		l.Error(fmt.Errorf("received non-OK response: %d", resp.StatusCode), "Received non-OK response")
		return fmt.Errorf("received non-OK response: %d", resp.StatusCode)
	}

	l.Info("Successfully sent status update for ticket ID", "ticketID", ticketID)
	return nil
}

// updateStatus updates a JitRequest status and message with retry up to maxAttempts attempts
func (r *JitRequestReconciler) updateStatus(ctx context.Context, jitRequest *jitv1.JitRequest, status, message string) error {
	jitRequest.Status.State = status
	jitRequest.Status.Message = message
	jitRequest.Status.StartTime = jitRequest.Spec.StartTime
	jitRequest.Status.EndTime = jitRequest.Spec.EndTime

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.Status().Update(ctx, jitRequest)

	})
	if err != nil {
		return fmt.Errorf("failed to update JitRequest status: %v", err)
	}

	return nil
}

// deleteJitRequest deletes a JitRequest
func (r *JitRequestReconciler) deleteJitRequest(ctx context.Context, jitRequest *jitv1.JitRequest) error {
	l := log.FromContext(ctx)
	if err := r.Client.Delete(ctx, jitRequest); err != nil {
		l.Error(err, "Failed to delete JitRequest")
		return err
	}
	l.Info("Successfully deleted JitRequest", "name", jitRequest.Name)
	return nil
}

// raiseEvent raises an event in the operator namespace
func (r *JitRequestReconciler) raiseEvent(obj client.Object, eventType, reason, message string) {
	eventRef := &corev1.ObjectReference{
		Kind:       obj.GetObjectKind().GroupVersionKind().Kind,
		APIVersion: obj.GetObjectKind().GroupVersionKind().GroupVersion().String(),
		Name:       obj.GetName(),
		Namespace:  OperatorNamespace,
		UID:        obj.GetUID(),
	}

	r.Recorder.Event(eventRef, eventType, reason, message)
}

// rejectInvalidNamespace rejects an invalid namespace
func (r *JitRequestReconciler) rejectInvalidNamespace(ctx context.Context, l logr.Logger, jitRequest *jitv1.JitRequest, namespace, err string) (ctrl.Result, error) {
	errorMsg := fmt.Sprintf("Namespace(s) %s not validated | Error: %s", namespace, err)
	r.raiseEvent(jitRequest, "Warning", EventValidationFailed, errorMsg)
	if err := r.updateStatus(ctx, jitRequest, StatusRejected, errorMsg); err != nil {
		l.Error(err, "failed to update status to Rejected")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// rejectInvalidRole rejects an invalid cluster role
func (r *JitRequestReconciler) rejectInvalidRole(ctx context.Context, l logr.Logger, jitRequest *jitv1.JitRequest) (ctrl.Result, error) {
	errorMsg := fmt.Sprintf("ClusterRole '%s' is not allowed", jitRequest.Spec.ClusterRole)
	r.raiseEvent(jitRequest, "Warning", EventValidationFailed, errorMsg)
	if err := r.updateStatus(ctx, jitRequest, StatusRejected, errorMsg); err != nil {
		l.Error(err, "failed to update status to Rejected")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// rejectUnauthorisedApiCall rejects when callback to api fails with 401
// func (r *JitRequestReconciler) rejectUnauthorisedApiCall(ctx context.Context, l logr.Logger, jitRequest *jitv1.JitRequest) (ctrl.Result, error) {
// 	errorMsg := "Callback to api failed is not allowed"
// 	r.raiseEvent(jitRequest, "Warning", UnauthorizedApi, errorMsg)
// 	if err := r.updateStatus(ctx, jitRequest, StatusRejected, errorMsg); err != nil {
// 		l.Error(err, "failed to update status to Rejected")
// 		return ctrl.Result{}, err
// 	}
// 	return ctrl.Result{}, nil
// }

// deleteOwnedObjects deletes role binding(s) in case of k8s GC failed to delete
func (r *JitRequestReconciler) deleteOwnedObjects(ctx context.Context, jitRequest *jitv1.JitRequest) error {
	for _, namespace := range jitRequest.Spec.Namespaces {
		roleBindings := &rbacv1.RoleBindingList{}

		err := r.List(ctx, roleBindings, client.InNamespace(namespace))
		if err != nil {
			return err
		}

		for _, roleBinding := range roleBindings.Items {
			for _, ownerRef := range roleBinding.OwnerReferences {
				if ownerRef.Kind == "JitRequest" && ownerRef.Name == jitRequest.Name {
					// Delete the RoleBinding if it is owned by the JitRequest
					if err := r.Delete(ctx, &roleBinding); err != nil && !apierrors.IsNotFound(err) {
						return err
					}
					break
				}
			}
		}
	}

	return nil
}

// isAlreadyExistsError checks and return true if something already exists
func isAlreadyExistsError(err error) bool {
	return err != nil && apierrors.IsAlreadyExists(err)
}

// createRoleBinding creates role binding(s) for a JitRequest's namespaces
func (r *JitRequestReconciler) createRoleBinding(ctx context.Context, jitRequest *jitv1.JitRequest) error {
	subjects := []rbacv1.Subject{}

	// Add user emails as subjects
	for _, email := range jitRequest.Spec.UserEmails {
		subjects = append(subjects, rbacv1.Subject{
			Kind: rbacv1.UserKind,
			Name: email,
		})
	}

	// Loop through namespaces in JitRequest and create role binding
	for _, namespace := range jitRequest.Spec.Namespaces {
		roleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-jit", jitRequest.Name),
				Namespace: namespace,
				Annotations: map[string]string{
					"jit.kubejit.io/expiry": jitRequest.Spec.EndTime.Time.Format(time.RFC3339),
				},
			},
			Subjects: subjects,
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     jitRequest.Spec.ClusterRole,
			},
		}

		// Set owner references
		if err := ctrl.SetControllerReference(jitRequest, roleBinding, r.Scheme); err != nil {
			return fmt.Errorf("failed to set owner reference for RoleBinding: %v", err)
		}

		// Create RoleBinding
		if err := r.Client.Create(ctx, roleBinding); err != nil {
			if !isAlreadyExistsError(err) {
				return fmt.Errorf("failed to create RoleBinding: %w", err)
			}
		}
	}

	return nil
}
