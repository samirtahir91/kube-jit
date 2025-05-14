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

package controller

import (
	"fmt"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"kube-jit-operator/test/utils"
)

var (
	TestNamespace = os.Getenv("OPERATOR_NAMESPACE")
)

const (
	JitRequestName = "e2e-jit-test"
)

// Function to initialise os vars
func init() {
	if TestNamespace == "" {
		panic(fmt.Errorf("OPERATOR_NAMESPACE environment variable(s) not set"))
	}
}

var _ = Describe("JitRequest Controller", Ordered, Label("integration"), func() {

	BeforeAll(func() {
		By("removing manager config")
		cmd := exec.Command("kubectl", "delete", "kjitcfg", TestJitConfig)
		_, _ = utils.Run(cmd)

		By("removing jitRequest")
		cmd = exec.Command("kubectl", "delete", "jitreq", JitRequestName)
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

		By("removing jitRequest")
		cmd = exec.Command("kubectl", "delete", "jitreq", JitRequestName)
		_, _ = utils.Run(cmd)
	})

	// Run common controller test cases
	utils.JitRequestTests(TestNamespace)
})
