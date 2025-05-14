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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

var _ = Describe("JitGroupCache Controller", Ordered, Label("integration"), func() {

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

	Context("When reconciling JitGroupCache", func() {
		const (
			testGroupID   = "group-123"
			testGroupName = "Test Group"
		)

		BeforeEach(func() {
			// Clean up before each test
			_, _ = utils.Run(exec.Command("kubectl", "delete", "ns", TestNamespace))
		})

		AfterEach(func() {
			_, _ = utils.Run(exec.Command("kubectl", "delete", "ns", TestNamespace))
		})

		It("should create JitGroupCache if it does not exist", func() {
			By("Deleting the JitGroupCache CR if it exists")
			_, _ = utils.Run(exec.Command("kubectl", "delete", "jitgroupcache", "jitgroupcache"))

			By("Triggering a reconcile by creating a labeled namespace")
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: TestNamespace,
					Labels: map[string]string{
						"jit.kubejit.io/adopt": "true",
					},
					Annotations: map[string]string{
						"jit.kubejit.io/group_id":   testGroupID,
						"jit.kubejit.io/group_name": testGroupName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())

			By("Eventually the JitGroupCache CR should exist and contain the group")
			Eventually(func(g Gomega) {
				cache := &jitv1.JitGroupCache{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "jitgroupcache"}, cache)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(cache.Spec.Groups).To(ContainElement(jitv1.JitGroup{
					Namespace: TestNamespace,
					GroupID:   testGroupID,
					GroupName: testGroupName,
				}))
			}, "20s", "1s").Should(Succeed())
		})

		It("should update JitGroupCache when a namespace is deleted", func() {
			By("Creating a labeled namespace")
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: TestNamespace,
					Labels: map[string]string{
						"jit.kubejit.io/adopt": "true",
					},
					Annotations: map[string]string{
						"jit.kubejit.io/group_id":   testGroupID,
						"jit.kubejit.io/group_name": testGroupName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())

			By("Ensuring the group is present in the cache")
			Eventually(func(g Gomega) {
				cache := &jitv1.JitGroupCache{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "jitgroupcache"}, cache)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(cache.Spec.Groups).To(ContainElement(jitv1.JitGroup{
					Namespace: TestNamespace,
					GroupID:   testGroupID,
					GroupName: testGroupName,
				}))
			}, "20s", "1s").Should(Succeed())

			By("Deleting the namespace")
			Expect(k8sClient.Delete(ctx, ns)).To(Succeed())

			By("Eventually the group should be removed from the cache")
			Eventually(func(g Gomega) {
				cache := &jitv1.JitGroupCache{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "jitgroupcache"}, cache)
				g.Expect(err).NotTo(HaveOccurred())
				for _, group := range cache.Spec.Groups {
					g.Expect(group.Namespace).NotTo(Equal(TestNamespace))
				}
			}, "20s", "1s").Should(Succeed())
		})

		It("should update JitGroupCache when a namespace's group info changes", func() {
			By("Creating a labeled namespace")
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: TestNamespace,
					Labels: map[string]string{
						"jit.kubejit.io/adopt": "true",
					},
					Annotations: map[string]string{
						"jit.kubejit.io/group_id":   testGroupID,
						"jit.kubejit.io/group_name": testGroupName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())

			By("Ensuring the group is present in the cache")
			Eventually(func(g Gomega) {
				cache := &jitv1.JitGroupCache{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "jitgroupcache"}, cache)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(cache.Spec.Groups).To(ContainElement(jitv1.JitGroup{
					Namespace: TestNamespace,
					GroupID:   testGroupID,
					GroupName: testGroupName,
				}))
			}, "20s", "1s").Should(Succeed())

			By("Updating the namespace's group info")
			patch := client.MergeFrom(ns.DeepCopy())
			ns.Annotations["jit.kubejit.io/group_id"] = "group-456"
			ns.Annotations["jit.kubejit.io/group_name"] = "New Group"
			Expect(k8sClient.Patch(ctx, ns, patch)).To(Succeed())

			By("Eventually the cache should reflect the new group info")
			Eventually(func(g Gomega) {
				cache := &jitv1.JitGroupCache{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "jitgroupcache"}, cache)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(cache.Spec.Groups).To(ContainElement(jitv1.JitGroup{
					Namespace: TestNamespace,
					GroupID:   "group-456",
					GroupName: "New Group",
				}))
			}, "20s", "1s").Should(Succeed())
		})
	})
})
