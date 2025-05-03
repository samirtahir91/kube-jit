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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	labelAdopt        = "jit.kubejit.io/adopt"
	annotationGroupID = "jit.kubejit.io/group_id"

	JitGroupCacheName = "global-jitgroupcache" // Static name for the JitGroupCache object
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
	jitGroupCache := &v1.JitGroupCache{}
	err := r.Get(ctx, client.ObjectKey{Name: JitGroupCacheName}, jitGroupCache)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			l.Error(err, "Failed to fetch JitGroupCache")
			return ctrl.Result{}, err
		}

		// If the JitGroupCache object doesn't exist, create it
		l.Info("JitGroupCache not found, creating a new one")
		jitGroupCache = &v1.JitGroupCache{
			ObjectMeta: metav1.ObjectMeta{
				Name: JitGroupCacheName,
			},
			Spec: v1.JitGroupCacheSpec{
				Groups: []v1.JitGroup{}, // Initialize with an empty list
			},
		}
		if err := r.Create(ctx, jitGroupCache); err != nil {
			l.Error(err, "Failed to create JitGroupCache")
			return ctrl.Result{}, err
		}
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
		if err := r.Update(ctx, jitGroupCache); err != nil {
			l.Error(err, "Failed to update JitGroupCache after namespace deletion")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Namespace exists, add or update it in JitGroupCache
	l.Info("Namespace exists, adding/updating in JitGroupCache", "namespace", namespace.Name)
	groupID := namespace.Annotations[annotationGroupID]
	jitGroupCache.Spec.Groups = addOrUpdateNamespaceInCache(jitGroupCache.Spec.Groups, namespace.Name, groupID)

	// Update the JitGroupCache object
	if err := r.Update(ctx, jitGroupCache); err != nil {
		l.Error(err, "Failed to update JitGroupCache")
		return ctrl.Result{}, err
	}

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

// removeNamespaceFromCache removes a namespace from the cache
func removeNamespaceFromCache(groups []v1.JitGroup, namespace string) []v1.JitGroup {
	updatedGroups := []v1.JitGroup{}
	for _, group := range groups {
		if group.Namespace != namespace {
			updatedGroups = append(updatedGroups, group)
		}
	}
	return updatedGroups
}

// addOrUpdateNamespaceInCache adds a namespace to the cache or updates its group ID
func addOrUpdateNamespaceInCache(groups []v1.JitGroup, namespace, groupID string) []v1.JitGroup {
	updated := false
	for i, group := range groups {
		if group.Namespace == namespace {
			groups[i].GroupID = groupID
			updated = true
			break
		}
	}

	if !updated {
		groups = append(groups, v1.JitGroup{
			Namespace: namespace,
			GroupID:   groupID,
		})
	}

	return groups
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
