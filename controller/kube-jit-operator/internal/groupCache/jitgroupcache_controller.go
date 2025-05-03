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
	l := logf.FromContext(ctx)
	l.Info("Reconciling JitGroupCache")

	// Fetch the JitGroupCache object
	jitGroupCache, err := r.fetchOrCreateJitGroupCache(ctx, l)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Fetch the Namespace object
	namespace := &corev1.Namespace{}
	err = r.Get(ctx, req.NamespacedName, namespace)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			l.Error(err, "Failed to fetch Namespace")
			return ctrl.Result{}, err
		}

		// Namespace was deleted, remove it from JitGroupCache
		l.Info("Namespace deleted, removing from JitGroupCache", "namespace", req.Name)
		jitGroupCache.Spec.Groups = removeNamespaceFromCache(jitGroupCache.Spec.Groups, req.Name)
		return r.updateJitGroupCache(ctx, jitGroupCache, l)
	}

	// Namespace exists, add or update it in JitGroupCache
	l.Info("Namespace exists, adding/updating in JitGroupCache", "namespace", namespace.Name)
	groupID := namespace.Annotations[AnnotationGroupID]
	jitGroupCache.Spec.Groups = addOrUpdateNamespaceInCache(jitGroupCache.Spec.Groups, namespace.Name, groupID)

	// Update the JitGroupCache object
	return r.updateJitGroupCache(ctx, jitGroupCache, l)
}

// SetupWithManager sets up the controller with the Manager.
func (r *JitGroupCacheReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Watch Namespaces with the label "jit.kubejit.io/adopt=true"
		Watches(
			&corev1.Namespace{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(
				// Filter on label "jit.kubejit.io/adopt=true"
				namespacePredicate(),
			),
		).
		Complete(r)
}
