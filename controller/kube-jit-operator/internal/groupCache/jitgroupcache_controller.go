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
	v1 "kube-jit-operator/api/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

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

	// On startup or on JitGroupCache delete (empty request), rebuild the cache from scratch
	if req.Name == "" && req.Namespace == "" {
		if err := r.RebuildJitGroupCache(ctx, l); err != nil {
			return ctrl.Result{}, err
		}
		// Return early so we don't run the rest of the logic with empty name
		return ctrl.Result{}, nil
	}

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
	groupName := namespace.Annotations[AnnotationGroupName]
	jitGroupCache.Spec.Groups = addOrUpdateNamespaceInCache(jitGroupCache.Spec.Groups, namespace.Name, groupID, groupName)

	// Update the JitGroupCache object
	return r.updateJitGroupCache(ctx, jitGroupCache, l)
}

// SetupWithManager sets up the controller with the Manager.
func (r *JitGroupCacheReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// On startup, trigger a reconcile with empty request to rebuild the cache
	mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		l := logf.FromContext(ctx)
		return r.RebuildJitGroupCache(ctx, l)
	}))

	// Predicate to only allow events for the JitGroupCache with the correct name
	onlyJitGroupCacheName := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == JitGroupCacheName
	})

	if err := ctrl.NewControllerManagedBy(mgr).
		For(
			&corev1.Namespace{},
			builder.WithPredicates(
				namespacePredicate(),
			),
		).
		Watches(
			&v1.JitGroupCache{},
			handler.Funcs{
				DeleteFunc: func(ctx context.Context, e event.TypedDeleteEvent[client.Object], q workqueue.TypedRateLimitingInterface[ctrl.Request]) {
					// Enqueue a dummy request to trigger a reconcile (which will rebuild the cache)
					q.Add(ctrl.Request{})
				},
			},
			// Only watch for events on the solo named JitGroupCache 'jitgroupcache'
			builder.WithPredicates(onlyJitGroupCacheName),
		).
		Named("JitGroupCache").
		Complete(r); err != nil {
		return err
	}

	return nil
}
