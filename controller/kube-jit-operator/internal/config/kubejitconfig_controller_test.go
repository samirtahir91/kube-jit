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

package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	jitv1 "kube-jit-operator/api/v1"
	"kube-jit-operator/test/utils"
	"os/exec"

	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	ValidClusterRole string = "edit"
)

var (
	TestNamespace = os.Getenv("OPERATOR_NAMESPACE")
)

// init os vars
func init() {
	if TestNamespace == "" {
		panic(fmt.Errorf("OPERATOR_NAMESPACE environment variable(s) not set"))
	}
}

var _ = Describe("JustInTimeConfig Controller", Ordered, Label("integration"), func() {

	BeforeAll(func() {
		By("removing manager config")
		cmd := exec.Command("kubectl", "delete", "jitcfg", TestJitConfig)
		_, _ = utils.Run(cmd)

	})

	AfterAll(func() {
		By("removing manager config")
		cmd := exec.Command("kubectl", "delete", "jitcfg", TestJitConfig)
		_, _ = utils.Run(cmd)
	})

	Context("When initialising a context and K8s client", func() {
		It("should be successfully initialised", func() {
			By("Creating the ctx and client")
			ctx = context.TODO()
			err := jitv1.AddToScheme(scheme.Scheme)
			Expect(err).NotTo(HaveOccurred())
			cfg, err := config.GetConfig()
			if err != nil {
				fmt.Printf("Failed to load kubeconfig: %v\n", err)
				return
			}
			k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient).NotTo(BeNil())
		})
	})

	Context("When creating the JustInTime config object", func() {
		It("should successfully load the config and write the config file", func() {
			By("Creating the operator JustInTimeConfig")
			err := utils.CreateJitConfig(ctx, k8sClient, ValidClusterRole, TestNamespace)
			Expect(err).NotTo(HaveOccurred())

			filePath := ConfigCacheFilePath + "/" + ConfigFile

			By("Waiting for the config file to be written")
			Eventually(func() bool {
				_, statErr := os.Stat(filePath)
				return statErr == nil
			}, 20, 0.5).Should(BeTrue(), "expected config file to be written within 10 seconds")

			By("Checking the config json file matches expected config")
			expectedConfig := jitv1.KubeJitConfigSpec{
				AllowedClusterRoles:   []string{"edit"},
				NamespaceAllowedRegex: "^kube-jit-int-test$",
			}

			// Read the generated config file
			data, err := os.ReadFile(filePath)
			Expect(err).NotTo(HaveOccurred())

			var generatedConfig jitv1.KubeJitConfigSpec
			err = json.Unmarshal(data, &generatedConfig)
			Expect(err).NotTo(HaveOccurred())

			// Compare the generated config with the expected config
			Expect(expectedConfig).To(Equal(generatedConfig))
		})
	})
})
