package utils

import (
	"context"
	"fmt"

	jitv1 "kube-jit-operator/api/v1"

	//lint:ignore ST1001 for ginko
	. "github.com/onsi/ginkgo/v2" //nolint:golint,revive
	//lint:ignore ST1001 for ginko
	. "github.com/onsi/gomega" //nolint:golint,revive
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	JitRequestName               = "e2e-jit-test"
	RoleBindingName              = JitRequestName + "-jit"
	ValidClusterRole      string = "edit"
	InvalidClusterRole           = "admin"
	EventValidationFailed        = "ValidationFailed"

	StatusPending   = "Pending"
	StatusSucceeded = "Succeeded"
)

var k8sClient client.Client
var ctx context.Context

func JitRequestTests(namespace string) {

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
			err := CreateJitConfig(ctx, k8sClient, ValidClusterRole, namespace)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When creating a new valid JitRequest with a start time 10s from now", func() {
		It("should successfully process as a new request and issue a rolebinding", func() {
			By("Creating the JitRequest")
			jitRequest, err := CreateJitRequest(ctx, k8sClient, 10, ValidClusterRole, namespace)
			Expect(err).NotTo(HaveOccurred())

			By("Checking the status of the JitRequest for Pending")
			err = CheckJitStatus(ctx, k8sClient, jitRequest, StatusPending)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for the JitRequest Pending event to be recorded")
			err = CheckEvent(
				ctx,
				k8sClient,
				JitRequestName,
				namespace,
				"Normal",
				StatusPending,
				"ClusterRole 'edit' is allowed\nTicket: 1234567890",
			)
			Expect(err).NotTo(HaveOccurred())

			By("Checking the status of the JitRequest for completed status")
			err = CheckJitStatus(ctx, k8sClient, jitRequest, StatusSucceeded)
			Expect(err).NotTo(HaveOccurred())

			By("Checking the RoleBinding exists")
			err = CheckRoleBindingExists(ctx, k8sClient, namespace, RoleBindingName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should successfully remove the JitRequest on expiry and remove the RoleBinding", func() {
			By("Checking the RoleBinding is eventually removed")
			err := CheckRoleBindingRemoved(ctx, k8sClient, namespace, RoleBindingName)
			Expect(err).NotTo(HaveOccurred())

			By("Checking the JitRequest is eventually removed")
			err = CheckJitRemoved(ctx, k8sClient, JitRequestName)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When creating a new JitRequest with invalid start time from now", func() {
		It("should successfully process as a new request and reject the JitRequest", func() {
			By("Creating the JitRequest")
			_, err := CreateJitRequest(ctx, k8sClient, -10, ValidClusterRole, namespace)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for the JitRequest Rejected event to be recorded")
			err = CheckEvent(
				ctx,
				k8sClient,
				JitRequestName,
				namespace,
				"Warning",
				EventValidationFailed,
				"must be after current time",
			)
			Expect(err).NotTo(HaveOccurred())

			By("Checking the JitRequest is eventually removed")
			err = CheckJitRemoved(ctx, k8sClient, JitRequestName)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When creating a new JitRequest with invalid cluster role", func() {
		It("should successfully process as a new request and reject the JitRequest", func() {
			By("Creating the JitRequest")
			_, err := CreateJitRequest(ctx, k8sClient, 10, InvalidClusterRole, namespace)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for the JitRequest Rejected event to be recorded")
			err = CheckEvent(
				ctx,
				k8sClient,
				JitRequestName,
				namespace,
				"Warning",
				EventValidationFailed,
				fmt.Sprintf("ClusterRole '%s' is not allowed", InvalidClusterRole),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Checking the JitRequest is eventually removed")
			err = CheckJitRemoved(ctx, k8sClient, JitRequestName)
			Expect(err).NotTo(HaveOccurred())
		})
	})
}
