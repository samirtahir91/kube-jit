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
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v1 "kube-jit-operator/api/v1"

	corev1 "k8s.io/api/core/v1"
)

const (
	labelAdopt        = "jit.kubejit.io/adopt"
	annotationGroupID = "jit.kubejit.io/group_id"
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

// namespacePredicate checks namespaces have the label "jit.kubejit.io/adopt=true"
// and if the annotation annotationGroupID has changed or exists
func namespacePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			labels := e.Object.GetLabels()
			if labels[labelAdopt] != "true" {
				return false
			}
			annotations := e.Object.GetAnnotations()
			return annotations[annotationGroupID] != ""
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			labels := e.ObjectNew.GetLabels()
			if labels[labelAdopt] != "true" {
				return false
			}

			oldAnnotations := e.ObjectOld.GetAnnotations()
			newAnnotations := e.ObjectNew.GetAnnotations()
			return oldAnnotations[annotationGroupID] != newAnnotations[annotationGroupID]
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			labels := e.Object.GetLabels()
			return labels[labelAdopt] == "true"
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *JitGroupCacheReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.JitGroupCache{}).
		Named("jitgroupcache").
		// Watch Namespaces with the label "jit.kubejit.io/adopt=true"
		Watches(
			&corev1.Namespace{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(
				// filter on label "jit.kubejit.io/adopt=true"
				namespacePredicate(),
			)).
		Complete(r)
}
