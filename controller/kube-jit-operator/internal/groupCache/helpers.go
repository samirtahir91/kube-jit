package groupCache

import (
	"context"

	v1 "kube-jit-operator/api/v1"

	corev1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// namespacePredicate checks namespaces have the label "jit.kubejit.io/adopt=true"
// and if BOTH the annotation AnnotationGroupID AND AnnotationGroupName exist
func namespacePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			labels := e.Object.GetLabels()
			if labels[LabelAdopt] != "true" {
				return false
			}
			annotations := e.Object.GetAnnotations()
			return annotations[AnnotationGroupID] != "" && annotations[AnnotationGroupName] != ""
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			labels := e.ObjectNew.GetLabels()
			if labels[LabelAdopt] != "true" {
				return false
			}
			oldAnnotations := e.ObjectOld.GetAnnotations()
			newAnnotations := e.ObjectNew.GetAnnotations()
			return (oldAnnotations[AnnotationGroupID] != newAnnotations[AnnotationGroupID] ||
				oldAnnotations[AnnotationGroupName] != newAnnotations[AnnotationGroupName]) &&
				newAnnotations[AnnotationGroupID] != "" && newAnnotations[AnnotationGroupName] != ""
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

// addOrUpdateNamespaceInCache adds a namespace to the cache or updates its group ID and group name
func addOrUpdateNamespaceInCache(groups []v1.JitGroup, namespace, groupID, groupName string) []v1.JitGroup {
	updated := false
	for i, group := range groups {
		if group.Namespace == namespace {
			groups[i].GroupID = groupID
			groups[i].GroupName = groupName
			updated = true
			break
		}
	}

	if !updated {
		groups = append(groups, v1.JitGroup{
			Namespace: namespace,
			GroupID:   groupID,
			GroupName: groupName,
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

// RebuildJitGroupCache rebuilds the JitGroupCache from all namespaces with LabelAdopt and required annotations
func (r *JitGroupCacheReconciler) RebuildJitGroupCache(ctx context.Context, l logr.Logger) error {
	l.Info("Rebuilding JitGroupCache from scratch")

	var nsList corev1.NamespaceList
	if err := r.List(ctx, &nsList, client.MatchingLabels{LabelAdopt: "true"}); err != nil {
		l.Error(err, "Failed to list namespaces for JitGroupCache rebuild")
		return err
	}

	groups := []v1.JitGroup{}
	for _, ns := range nsList.Items {
		annotations := ns.GetAnnotations()
		groupID := annotations[AnnotationGroupID]
		groupName := annotations[AnnotationGroupName]
		if groupID != "" && groupName != "" {
			groups = append(groups, v1.JitGroup{
				Namespace: ns.Name,
				GroupID:   groupID,
				GroupName: groupName,
			})
		}
	}

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		// Always fetch the latest version before updating
		jitGroupCache := &v1.JitGroupCache{}
		err := r.Get(ctx, client.ObjectKey{Name: JitGroupCacheName}, jitGroupCache)
		if err != nil {
			if apierrors.IsNotFound(err) {
				// Not found, so create it
				jitGroupCache = &v1.JitGroupCache{
					ObjectMeta: metav1.ObjectMeta{
						Name: JitGroupCacheName,
					},
					Spec: v1.JitGroupCacheSpec{
						Groups: groups,
					},
				}
				return r.Create(ctx, jitGroupCache)
			}
			return err
		}
		// Found, update the spec and call Update
		jitGroupCache.Spec.Groups = groups
		return r.Update(ctx, jitGroupCache)
	})
}
