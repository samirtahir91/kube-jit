package controller

import (
	"context"
	"fmt"
	jitv1 "kube-jit-operator/api/v1"
	"kube-jit-operator/utils"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
)

// handleRejected rejects ticket and deletes a JitRequest
func (r *JitRequestReconciler) handleRejected(ctx context.Context, l logr.Logger, jitRequest *jitv1.JitRequest) (ctrl.Result, error) {
	// Reject ticket
	if err := r.callbackToApi(ctx, jitRequest); err != nil {
		l.Error(err, "Failed to callback (rejected) to API")
		r.raiseEvent(jitRequest, "Warning", "FailedCallback", fmt.Sprintf("Error: %s", err))
	}

	// Delete JitRequest
	if err := r.deleteJitRequest(ctx, jitRequest); err != nil {
		l.Error(err, "failed to delete JitRequest")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// preApproveRequest pre-approves a JitRequest, updates the Jira ticket and re-queues for start time
func (r *JitRequestReconciler) preApproveRequest(ctx context.Context, l logr.Logger, jitRequest *jitv1.JitRequest) (ctrl.Result, error) {
	startTime := jitRequest.Spec.StartTime.Time

	if startTime.After(time.Now()) {

		// record event
		r.raiseEvent(jitRequest, "Normal", StatusPending, fmt.Sprintf("ClusterRole '%s' is allowed\nTicket: %s", jitRequest.Spec.ClusterRole, jitRequest.Spec.TicketID))

		// msg for status and comment
		jitRequestStatusMsg := "Pending - Access will be granted at start time"

		// update jitRequest status
		if err := r.updateStatus(ctx, jitRequest, StatusPending, jitRequestStatusMsg); err != nil {
			l.Error(err, "failed to update status to Pending")
			return ctrl.Result{}, err
		}

		// callback to api
		if err := r.callbackToApi(ctx, jitRequest); err != nil {
			l.Error(err, "Failed to callback (pending) to API, but proceeding with granting access")
			r.raiseEvent(jitRequest, "Warning", "FailedCallback", fmt.Sprintf("Error: %s", err))
		}

		// requeue for start time
		delay := time.Until(startTime)
		l.Info("Start time not reached, requeuing", "requeueAfter", delay)
		return ctrl.Result{RequeueAfter: delay}, nil
	}

	// invalid start time, reject
	errMsg := fmt.Errorf("start time %s must be after current time", jitRequest.Spec.StartTime.Time)
	l.Error(errMsg, "start time validation failed")

	// record event
	r.raiseEvent(jitRequest, "Warning", EventValidationFailed, errMsg.Error())

	// update jitRequest status
	if err := r.updateStatus(ctx, jitRequest, StatusRejected, errMsg.Error()); err != nil {
		l.Error(err, "failed to update status to Rejected")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// handleNewRequest validates new JitRequests
func (r *JitRequestReconciler) handleNewRequest(ctx context.Context, l logr.Logger, jitRequest *jitv1.JitRequest, allowedClusterRoles []string) (ctrl.Result, error) {

	// check cluster role is allowed
	if !utils.Contains(allowedClusterRoles, jitRequest.Spec.ClusterRole) {
		return r.rejectInvalidRole(ctx, l, jitRequest)
	}

	// check namespaces match regex defined in config
	nsRegex, err := utils.ValidateNamespaceRegex(jitRequest.Spec.Namespaces)
	if err != nil {
		return r.rejectInvalidNamespace(ctx, l, jitRequest, nsRegex, err.Error())
	}

	// check namespaces exist
	namespacesExist, err := utils.ValidateNamespaceExists(jitRequest.Spec.Namespaces, r.Client)
	if err != nil {
		return r.rejectInvalidNamespace(ctx, l, jitRequest, namespacesExist, err.Error())
	}

	return r.preApproveRequest(ctx, l, jitRequest)
}

// handlePreApproved creates the role binding for approved JitRequests if the Jira ticket is approved
func (r *JitRequestReconciler) handlePreApproved(ctx context.Context, l logr.Logger, jitRequest *jitv1.JitRequest) (ctrl.Result, error) {
	// check if it needs to be re-queued
	startTime := jitRequest.Status.StartTime.Time
	if startTime.After(time.Now()) {
		// requeue for start time
		delay := time.Until(startTime)
		l.Info("Start time not reached, requeuing", "requeueAfter", delay)
		return ctrl.Result{RequeueAfter: delay}, nil
	}

	l.Info("Creating role binding")
	if err := r.createRoleBinding(ctx, jitRequest); err != nil {
		l.Error(err, "failed to create rbac for JIT request")
		r.raiseEvent(jitRequest, "Warning", "FailedRBAC", fmt.Sprintf("Error: %s", err))
		return ctrl.Result{}, err
	}

	if err := r.updateStatus(ctx, jitRequest, StatusSucceeded, "Access granted until end time"); err != nil {
		return ctrl.Result{}, err
	}

	// callback to api
	if err := r.callbackToApi(ctx, jitRequest); err != nil {
		l.Error(err, "Failed to callback (succeeded) to API, but proceeding with granting access")
		r.raiseEvent(jitRequest, "Warning", "FailedCallback", fmt.Sprintf("Error: %s", err))
	}

	// Queue for deletion at end time
	return r.handleCleanup(ctx, l, jitRequest)
}

// handleCleanup cleans up and re-queue succeeded and unknown JitRequests for deletion
func (r *JitRequestReconciler) handleCleanup(ctx context.Context, l logr.Logger, jitRequest *jitv1.JitRequest) (ctrl.Result, error) {
	endTime := jitRequest.Status.EndTime.Time
	if endTime.After(time.Now()) {
		delay := time.Until(endTime)
		l.Info("End time not reached, re-queuing", "requeueAfter", delay)
		return ctrl.Result{RequeueAfter: delay}, nil
	}

	l.Info("End time reached, deleting JitRequest")
	if err := r.deleteJitRequest(ctx, jitRequest); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// handleFetchError cleans-up owned objects (role bindings) on deleted JitRequests
func (r *JitRequestReconciler) handleFetchError(ctx context.Context, l logr.Logger, err error, jitRequest *jitv1.JitRequest) (ctrl.Result, error) {
	if apierrors.IsNotFound(err) {
		l.Info("JitRequest resource not found. Deleting managed objects.")
		if err := r.deleteOwnedObjects(ctx, jitRequest); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	l.Error(err, "failed to get JitRequest")
	return ctrl.Result{}, err
}
