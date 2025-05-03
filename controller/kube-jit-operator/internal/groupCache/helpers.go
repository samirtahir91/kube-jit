package groupCache

import (
	"context"

	v1 "kube-jit-operator/api/v1"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// namespacePredicate checks namespaces have the label "jit.kubejit.io/adopt=true"
// and if the annotation AnnotationGroupID has changed or exists
func namespacePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			labels := e.Object.GetLabels()
			if labels[LabelAdopt] != "true" {
				return false
			}
			annotations := e.Object.GetAnnotations()
			return annotations[AnnotationGroupID] != ""
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			labels := e.ObjectNew.GetLabels()
			if labels[LabelAdopt] != "true" {
				return false
			}

			oldAnnotations := e.ObjectOld.GetAnnotations()
			newAnnotations := e.ObjectNew.GetAnnotations()
			return oldAnnotations[AnnotationGroupID] != newAnnotations[AnnotationGroupID]
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			labels := e.Object.GetLabels()
			return labels[LabelAdopt] == "true"
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

// fetchOrCreateJitGroupCache fetches the JitGroupCache from the Kubernetes cluster, or creates a new one if it doesn't exist
func (r *JitGroupCacheReconciler) fetchOrCreateJitGroupCache(ctx context.Context, l logr.Logger) (*v1.JitGroupCache, error) {
	jitGroupCache := &v1.JitGroupCache{}
	err := r.Get(ctx, client.ObjectKey{Name: JitGroupCacheName}, jitGroupCache)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			l.Error(err, "Failed to fetch JitGroupCache")
			return nil, err
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
			return nil, err
		}
	}
	return jitGroupCache, nil
}

// updateJitGroupCache updates the JitGroupCache in the Kubernetes cluster
func (r *JitGroupCacheReconciler) updateJitGroupCache(ctx context.Context, jitGroupCache *v1.JitGroupCache, l logr.Logger) (ctrl.Result, error) {
	if err := r.Update(ctx, jitGroupCache); err != nil {
		l.Error(err, "Failed to update JitGroupCache")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}
