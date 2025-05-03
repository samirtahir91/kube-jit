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

package groupCache

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v1 "kube-jit-operator/api/v1"

	corev1 "k8s.io/api/core/v1"
)

// JitGroupCacheReconciler reconciles a JitGroupCache object
type JitGroupCacheReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=jit.kubejit.io,resources=jitgroupcaches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=jit.kubejit.io,resources=jitgroupcaches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=jit.kubejit.io,resources=jitgroupcaches/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

// JitGroupCacheReconciler reconciles a JitGroupCache object
func (r *JitGroupCacheReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	return ctrl.Result{}, nil
}

// Only watch namespaces with the label "kube-jit.io/adopt=true"
func namespacePredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		labels := object.GetLabels()
		return labels["kube-jit.io/adopt"] == "true"
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *JitGroupCacheReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.JitGroupCache{}).
		Named("jitgroupcache").
		// Watch Namespaces with the label "kube-jit.io/adopt=true"
		Watches(
			&corev1.Namespace{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(
				// on create/update/delete
				predicate.ResourceVersionChangedPredicate{},
				// filter on label "kube-jit.io/adopt=true"
				namespacePredicate(),
			)).
		Complete(r)
}
