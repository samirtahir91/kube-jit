/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	jitv1 "kube-jit-operator/api/v1"
	"kube-jit-operator/utils"
)

var OperatorNamespace = os.Getenv("OPERATOR_NAMESPACE")

// JitRequestReconciler reconciles a JitRequest object
type JitRequestReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=jit.kubejit.io,resources=jitrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=jit.kubejit.io,resources=jitrequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=jit.kubejit.io,resources=jitrequests/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

// Reconcile is the main loop for reconciling a JitRequest
func (r *JitRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx)

	// Fetch the JitRequest instance
	jitRequest, err := r.fetchJitRequest(ctx, req.NamespacedName)
	if err != nil {
		return r.handleFetchError(ctx, l, err, jitRequest)
	}

	// Fetch operator config
	operatorConfig, err := utils.ReadConfigFromFile()
	if err != nil {
		return ctrl.Result{}, err
	}
	allowedClusterRoles := operatorConfig.AllowedClusterRoles

	l.Info("Got JitRequest", "Requestor", jitRequest.Spec.Requestee, "Role", jitRequest.Spec.ClusterRole, "Namespace", strings.Join(jitRequest.Spec.Namespaces, ", "))

	// Handle JitRequest based on its status
	switch jitRequest.Status.State {
	case StatusRejected:
		return r.handleRejected(ctx, l, jitRequest)
	case "":
		return r.handleNewRequest(ctx, l, jitRequest, allowedClusterRoles)
	case StatusPending:
		return r.handlePreApproved(ctx, l, jitRequest)
	case StatusSucceeded:
		return r.handleCleanup(ctx, l, jitRequest)
	default:
		return r.handleCleanup(ctx, l, jitRequest)
	}
}

// jitRequestPredicate filters events for JitRequest objects and
// ignores if StatusRejected is identical for update events
func jitRequestPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldJitRequest, okOld := e.ObjectOld.(*jitv1.JitRequest)
			newJitRequest, okNew := e.ObjectNew.(*jitv1.JitRequest)
			if !okOld || !okNew {
				// Should not happen for JitRequest controller
				return true // Reconcile on type mismatch just in case
			}

			// If already rejected and stays rejected, and resource version is the same (no other changes), ignore.
			if oldJitRequest.Status.State == StatusRejected &&
				newJitRequest.Status.State == StatusRejected &&
				oldJitRequest.GetResourceVersion() == newJitRequest.GetResourceVersion() {
				return false
			}
			// Otherwise, reconcile if resource version changed (covers spec, status, metadata changes)
			return oldJitRequest.GetResourceVersion() != newJitRequest.GetResourceVersion()
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *JitRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&jitv1.JitRequest{}, builder.WithPredicates(jitRequestPredicate())).
		Named("jitrequest").
		Complete(r)
}
