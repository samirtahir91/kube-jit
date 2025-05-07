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

package utils

import (
	"context"
	"encoding/json"
	"fmt"
	jitv1 "kube-jit-operator/api/v1"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kube-jit-operator/internal/config"
	"os"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ReadConfigFromFile Reads operator configuration from config file
func ReadConfigFromFile() (*jitv1.KubeJitConfigSpec, error) {
	// common lock for concurrent reads
	config.ConfigLock.RLock()
	defer config.ConfigLock.RUnlock()

	data, err := os.ReadFile(fmt.Sprintf("%s/%s", config.ConfigCacheFilePath, config.ConfigFile))
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file: %w", err)
	}

	var newConfig jitv1.KubeJitConfigSpec
	if err := json.Unmarshal(data, &newConfig); err != nil {
		return nil, fmt.Errorf("failed to parse configuration file: %w", err)
	}

	return &newConfig, nil
}

// Contains checks if a string is present in a slice.
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ValidateNamespaceRegex validates namespace name with regex if provided
func ValidateNamespaceRegex(namespaces []string) (string, error) {
	if config.NamespaceAllowedRegex != nil {
		for _, namespace := range namespaces {
			if !config.NamespaceAllowedRegex.MatchString(namespace) {
				return namespace, field.Invalid(
					field.NewPath("spec").Child("namespace"),
					namespace,
					fmt.Sprintf("namespace does not match the allowed pattern: %s", config.NamespaceAllowedRegex.String()),
				)
			}
		}
	}
	return "", nil
}

// ValidateNamespaceExists validates namespaces exist
func ValidateNamespaceExists(namespaces []string, client client.Client) (string, error) {
	var invalidNamespaces []string
	for _, namespace := range namespaces {
		if err := client.Get(context.TODO(), types.NamespacedName{Name: namespace}, &corev1.Namespace{}); err != nil {
			if apierrors.IsNotFound(err) {
				invalidNamespaces = append(invalidNamespaces, namespace)
			} else {
				return "", err
			}
		}
	}
	if len(invalidNamespaces) > 0 {
		return strings.Join(invalidNamespaces, ", "), fmt.Errorf("some namespaces do not exist: %v", invalidNamespaces)
	}
	return "", nil
}
