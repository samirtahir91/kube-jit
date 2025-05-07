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

// KubeJitConfigSpec defines the desired state of KubeJitConfig.
type KubeJitConfigSpec struct {
	// Configure allowed cluster roles to bind for a JitRequest
	AllowedClusterRoles []string `json:"allowedClusterRoles" validate:"required"`
	// Optional regex to only allow namespace names matching the regular expression
	NamespaceAllowedRegex string `json:"namespaceAllowedRegex,omitempty"`
}

// KubeJitConfigStatus defines the observed state of KubeJitConfig.
type KubeJitConfigStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=kjitcfg

// KubeJitConfig is the Schema for the kubejitconfigs API.
type KubeJitConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubeJitConfigSpec   `json:"spec,omitempty"`
	Status KubeJitConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KubeJitConfigList contains a list of KubeJitConfig.
type KubeJitConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubeJitConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubeJitConfig{}, &KubeJitConfigList{})
}
