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

package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sync"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	jitv1 "kube-jit-operator/api/v1"
	"kube-jit-operator/pkg/configuration"
)

var (
	ConfigCacheFilePath   string
	ConfigFile            = "config.json"
	ConfigLock            sync.RWMutex
	NamespaceAllowedRegex *regexp.Regexp
)

// KubeJitConfigReconciler reconciles a KubeJitConfig object
type KubeJitConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=jit.kubejit.io,resources=kubejitconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=jit.kubejit.io,resources=kubejitconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=jit.kubejit.io,resources=kubejitconfigs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop for KubeJitConfigReconciler
func (c *KubeJitConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx)
	l.Info("KubeJitConfig reconciliation started", "request name", req.Name)

	cfg := configuration.NewKubeJitOperatorConfiguration(ctx, c.Client, req.Name)
	l.Info(
		"KubeJitConfig",
		"allowed cluster roles",
		cfg.AllowedClusterRoles(),
		"allowed namespace regex",
		cfg.NamespaceAllowedRegex(),
	)

	// validate regex and set for global use
	namespaceRegex := cfg.NamespaceAllowedRegex()
	if namespaceRegex != "" {
		var err error
		NamespaceAllowedRegex, err = regexp.Compile(namespaceRegex)
		if err != nil {
			l.Error(err, "regex is invalid for namespaceAllowedRegex")
			return ctrl.Result{}, err
		}
	}

	// cache config to file
	if err := c.SaveConfigToFile(ctx, cfg, ConfigCacheFilePath, ConfigFile); err != nil {
		l.Error(err, "failed to save configuration to file")
		return ctrl.Result{}, err
	}

	l.Info("KubeJitConfig reconciliation finished", "request name", req.Name)

	return ctrl.Result{}, nil
}

// SaveConfigToFile saves configuration to a file
func (c *KubeJitConfigReconciler) SaveConfigToFile(ctx context.Context, cfg configuration.Configuration, filePath string, fileName string) error {
	l := logf.FromContext(ctx)
	// Create dir if does not exist
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if err := os.MkdirAll(filePath, 0700); err != nil {
			return fmt.Errorf("failed to create config directory: %v", err)
		}
	}

	// common lock (config file is read by jitrequest controller reconciles)
	ConfigLock.Lock()
	defer ConfigLock.Unlock()

	configData := jitv1.KubeJitConfigSpec{
		AllowedClusterRoles:   cfg.AllowedClusterRoles(),
		NamespaceAllowedRegex: cfg.NamespaceAllowedRegex(),
	}

	data, err := json.MarshalIndent(configData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize configuration: %w", err)
	}

	file, err := os.Create(fmt.Sprintf("%s/%s", filePath, fileName))
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	defer func() {
		err := file.Close()
		if err != nil {
			l.Error(err, "error closing config file")
		}
	}()

	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}

// nameMatchPredicate returns if KubeJitConfig is named as per `name`
func nameMatchPredicate(name string) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetName() == name
	})
}

// SetupWithManager sets up the controller with the Manager.
func (c *KubeJitConfigReconciler) SetupWithManager(mgr ctrl.Manager, configurationName string, configCacheFilePath string) error {
	ConfigCacheFilePath = configCacheFilePath
	return ctrl.NewControllerManagedBy(mgr).
		For(&jitv1.KubeJitConfig{},
			builder.WithPredicates(
				predicate.ResourceVersionChangedPredicate{},
				nameMatchPredicate(configurationName),
			)).
		Named("kubejitconfig").
		Complete(c)
}
