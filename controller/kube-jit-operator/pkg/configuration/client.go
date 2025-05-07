/*
Copyright 2024.

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

package configuration

import (
	"context"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	jitv1 "kube-jit-operator/api/v1"
)

// kubeJitOperatorConfiguration
type kubeJitOperatorConfiguration struct {
	retrievalFn func() *jitv1.KubeJitConfig
}

// NewKubeJitOperatorConfiguration returns new KubeJitConfig or default config if not found in the cluster
func NewKubeJitOperatorConfiguration(ctx context.Context, client client.Client, name string) Configuration {
	return &kubeJitOperatorConfiguration{retrievalFn: func() *jitv1.KubeJitConfig {
		config := &jitv1.KubeJitConfig{}

		if err := client.Get(ctx, types.NamespacedName{Name: name}, config); err != nil {
			if apierrors.IsNotFound(err) {
				return &jitv1.KubeJitConfig{
					Spec: jitv1.KubeJitConfigSpec{
						AllowedClusterRoles:   []string{"edit"},
						NamespaceAllowedRegex: ".*",
					},
				}
			}
			panic(errors.Wrap(err, "Cannot retrieve configuration with name "+name))
		}

		return config
	}}
}

func (c *kubeJitOperatorConfiguration) NamespaceAllowedRegex() string {
	return c.retrievalFn().Spec.NamespaceAllowedRegex
}

func (c *kubeJitOperatorConfiguration) AllowedClusterRoles() []string {
	return c.retrievalFn().Spec.AllowedClusterRoles
}
