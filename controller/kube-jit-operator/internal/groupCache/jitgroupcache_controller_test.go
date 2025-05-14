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

package groupCache

import (
	"context"
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
		cmd := exec.Command("kubectl", "delete", "kjitcfg", TestJitConfig)
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", TestNamespace)
		_, _ = utils.Run(cmd)

		By("creating manager namespace")
		err := utils.CreateNamespace(ctx, k8sClient, TestNamespace)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		By("removing manager namespace")
		cmd := exec.Command("kubectl", "delete", "ns", TestNamespace)
		_, _ = utils.Run(cmd)

		By("removing manager config")
		cmd = exec.Command("kubectl", "delete", "kjitcfg", TestJitConfig)
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

	Context("When creating the KubeJit config object", func() {
		It("should successfully load the config and write the config file", func() {
			By("Creating the operator KubeJitConfig")
			err := utils.CreateJitConfig(ctx, k8sClient, ValidClusterRole, TestNamespace)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
