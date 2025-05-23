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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// JitGroupCacheSpec defines the desired state of JitGroupCache.
type JitGroupCacheSpec struct {
	// The JitGroups to
	Groups []JitGroup `json:"groups"`
}

// JitGroup defines the group ID, namespace, and group name
type JitGroup struct {
	// The group ID
	// +kubebuilder:validation:Required
	GroupID string `json:"groupID"`
	// The group namespace
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
	// The group name
	// +kubebuilder:validation:Required
	GroupName string `json:"groupName"`
}

// JitGroupCacheStatus defines the observed state of JitGroupCache.
type JitGroupCacheStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=kjitcache

// JitGroupCache is the Schema for the jitgroupcaches API.
type JitGroupCache struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JitGroupCacheSpec   `json:"spec,omitempty"`
	Status JitGroupCacheStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// JitGroupCacheList contains a list of JitGroupCache.
type JitGroupCacheList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []JitGroupCache `json:"items"`
}

func init() {
	SchemeBuilder.Register(&JitGroupCache{}, &JitGroupCacheList{})
}
