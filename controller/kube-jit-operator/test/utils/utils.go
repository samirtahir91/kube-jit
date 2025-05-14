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

package utils

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	//lint:ignore ST1001 for ginko
	. "github.com/onsi/ginkgo/v2" //nolint:golint,revive
	//lint:ignore ST1001 for ginko
	. "github.com/onsi/gomega" //nolint:golint,revive
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	jitv1 "kube-jit-operator/api/v1"

	rbacv1 "k8s.io/api/rbac/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	prometheusOperatorVersion = "v0.77.1"
	prometheusOperatorURL     = "https://github.com/prometheus-operator/prometheus-operator/" +
		"releases/download/%s/bundle.yaml"

	certmanagerVersion = "v1.16.3"
	certmanagerURLTmpl = "https://github.com/cert-manager/cert-manager/releases/download/%s/cert-manager.yaml"
)

// CheckJitRemoved checks and JitRequest is deleted
func CheckJitRemoved(ctx context.Context, k8sClient client.Client, name string) error {
	Eventually(func() bool {
		jitRequest := &jitv1.JitRequest{}
		key := types.NamespacedName{Name: name, Namespace: jitRequest.Namespace}
		err := k8sClient.Get(ctx, key, jitRequest)
		if err != nil {
			if client.IgnoreNotFound(err) == nil {
				return true // JitRequest is removed
			}
			fmt.Printf("Error retrieving JitRequest: %v\n", err)
			return false // Error other than not found
		}
		return false // JitRequest still exists
	}, "30s", "5s").Should(BeTrue(), "JitRequest %s was not removed", name)

	fmt.Printf("JitRequest %s is removed\n", name)
	return nil
}

// CheckRoleBindingRemoved checks a role binding is removed
func CheckRoleBindingRemoved(ctx context.Context, k8sClient client.Client, namespace string, name string) error {
	Eventually(func() bool {
		roleBinding := &rbacv1.RoleBinding{}
		key := types.NamespacedName{Name: name, Namespace: namespace}
		err := k8sClient.Get(ctx, key, roleBinding)
		if err != nil {
			if client.IgnoreNotFound(err) == nil {
				return true // RoleBinding is removed
			}
			fmt.Printf("Error retrieving RoleBinding: %v\n", err)
			return false // Error other than not found
		}
		return false // RoleBinding still exists
	}, "60s", "5s").Should(BeTrue(), "RoleBinding %s in namespace %s was not removed", name, namespace)

	fmt.Printf("RoleBinding %s in namespace %s is removed\n", name, namespace)
	return nil
}

// CheckRoleBindingExists checks a Role Binding exists
func CheckRoleBindingExists(ctx context.Context, k8sClient client.Client, namespace string, name string) error {
	Eventually(func() bool {
		roleBinding := &rbacv1.RoleBinding{}
		key := types.NamespacedName{Name: name, Namespace: namespace}
		err := k8sClient.Get(ctx, key, roleBinding)
		if err != nil {
			fmt.Printf("Error retrieving RoleBinding: %v\n", err)
			return false // Unable to retrieve the RoleBinding
		}
		return true // RoleBinding exists
	}, "30s", "5s").Should(BeTrue(), "RoleBinding %s in namespace %s does not exist", name, namespace)

	fmt.Printf("RoleBinding %s in namespace %s exists\n", name, namespace)
	return nil
}

// CheckEvent checks and waits for an event in a namespace
func CheckEvent(ctx context.Context, k8sClient client.Client, objectName string, namespace string, eventType string, reason string, message string) error { //nolint:lll
	listOptions := &client.ListOptions{
		Namespace: namespace,
	}

	// Event not found, wait for it
	Eventually(func() error {
		// list events
		eventList := &corev1.EventList{}
		err := k8sClient.List(ctx, eventList, listOptions)
		if err != nil {
			return fmt.Errorf("failed to list events: %v", err)
		}
		// Check the event exists
		for _, evt := range eventList.Items {
			if evt.InvolvedObject.Name == objectName &&
				evt.Type == eventType &&
				evt.Reason == reason &&
				strings.Contains(evt.Message, message) {
				return nil // Event found
			}
		}

		// Event not found yet
		return fmt.Errorf("matching event not found for: %s", message)
	}, "20s", "5s").Should(Succeed())

	return nil
}

// CheckJitStatus checks the status of a JustInTimeRequest
func CheckJitStatus(ctx context.Context, k8sClient client.Client, jitRequest *jitv1.JitRequest, status string) error { //nolint:lll

	// Check if the status gets populated with the expected
	Eventually(func() bool {
		// Retrieve the jitRequest object
		key := types.NamespacedName{Name: jitRequest.Name, Namespace: jitRequest.Namespace}
		retrievedJitRequest := &jitv1.JitRequest{}
		err := k8sClient.Get(ctx, key, retrievedJitRequest)
		if err != nil {
			GinkgoWriter.Printf("Error retrieving JitRequest: %v\n", err)
			return false // Unable to retrieve the jitRequest
		}
		// Check if the status field contains the expected
		return retrievedJitRequest.Status.State == status
	}, "30s", "5s").Should(BeTrue(), "Failed on expected state for jitRequest")

	return nil
}

// CreateJitRequest creates a JustInTimeRequest with a startTime delay in seconds
func CreateJitRequest(ctx context.Context, k8sClient client.Client, startDelay time.Duration, clusterRole, namespace string, label ...string) (*jitv1.JitRequest, error) { //nolint:lll
	// optional label
	namespaceLabels := make(map[string]string)
	if len(label) > 0 && label[0] != "" {
		namespaceLabels["foo"] = label[0]
	}

	jit := &jitv1.JitRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "e2e-jit-test",
		},
		Spec: jitv1.JitRequestSpec{
			ClusterRole:   clusterRole,
			Requestee:     "master-chief",
			Justification: "e2e test",
			Approver:      "captain-keys",
			UserEmails:    []string{"master-chief@unsc.com"},
			Email:         "master-chief@unsc.com",
			TicketID:      "1234567890",
			CallbackURL:   "http://localhost/callback",
			Namespaces: []string{
				namespace,
			},
			StartTime: metav1.NewTime(metav1.Now().Add(startDelay * time.Second)),
			EndTime:   metav1.NewTime(metav1.Now().Add(20 * time.Second)),
		},
	}

	if err := k8sClient.Create(ctx, jit); err != nil {
		return nil, fmt.Errorf("failed to create JIT request: %w", err)
	}

	return jit, nil
}

// CreateNamespace creates a namespace
func CreateNamespace(ctx context.Context, k8sClient client.Client, namespace string) error {

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}

	if err := k8sClient.Create(ctx, ns); err != nil {
		return fmt.Errorf("failed to create Namespace: %w", err)
	}

	return nil
}

// CreateJitConfig creates a KubeJitConfig
func CreateJitConfig(ctx context.Context, k8sClient client.Client, clusterRole, namespace string) error {

	jitCfg := &jitv1.KubeJitConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-jit-operator-default",
		},
		Spec: jitv1.KubeJitConfigSpec{
			AllowedClusterRoles: []string{
				clusterRole,
			},
			NamespaceAllowedRegex: fmt.Sprintf("^%s$", namespace),
		},
	}

	if err := k8sClient.Create(ctx, jitCfg); err != nil {
		return fmt.Errorf("failed to create JIT config: %w", err)
	}

	return nil
}

func warnError(err error) {
	_, _ = fmt.Fprintf(GinkgoWriter, "warning: %v\n", err)
}

// Run executes the provided command within this context
func Run(cmd *exec.Cmd) (string, error) {
	dir, _ := GetProjectDir()
	cmd.Dir = dir

	if err := os.Chdir(cmd.Dir); err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "chdir dir: %s\n", err)
	}

	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	_, _ = fmt.Fprintf(GinkgoWriter, "running: %s\n", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s failed with error: (%v) %s", command, err, string(output))
	}

	return string(output), nil
}

// InstallPrometheusOperator installs the prometheus Operator to be used to export the enabled metrics.
func InstallPrometheusOperator() error {
	url := fmt.Sprintf(prometheusOperatorURL, prometheusOperatorVersion)
	cmd := exec.Command("kubectl", "create", "-f", url)
	_, err := Run(cmd)
	return err
}

// UninstallPrometheusOperator uninstalls the prometheus
func UninstallPrometheusOperator() {
	url := fmt.Sprintf(prometheusOperatorURL, prometheusOperatorVersion)
	cmd := exec.Command("kubectl", "delete", "-f", url)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// IsPrometheusCRDsInstalled checks if any Prometheus CRDs are installed
// by verifying the existence of key CRDs related to Prometheus.
func IsPrometheusCRDsInstalled() bool {
	// List of common Prometheus CRDs
	prometheusCRDs := []string{
		"prometheuses.monitoring.coreos.com",
		"prometheusrules.monitoring.coreos.com",
		"prometheusagents.monitoring.coreos.com",
	}

	cmd := exec.Command("kubectl", "get", "crds", "-o", "custom-columns=NAME:.metadata.name")
	output, err := Run(cmd)
	if err != nil {
		return false
	}
	crdList := GetNonEmptyLines(output)
	for _, crd := range prometheusCRDs {
		for _, line := range crdList {
			if strings.Contains(line, crd) {
				return true
			}
		}
	}

	return false
}

// UninstallCertManager uninstalls the cert manager
func UninstallCertManager() {
	url := fmt.Sprintf(certmanagerURLTmpl, certmanagerVersion)
	cmd := exec.Command("kubectl", "delete", "-f", url)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// InstallCertManager installs the cert manager bundle.
func InstallCertManager() error {
	url := fmt.Sprintf(certmanagerURLTmpl, certmanagerVersion)
	cmd := exec.Command("kubectl", "apply", "-f", url)
	if _, err := Run(cmd); err != nil {
		return err
	}
	// Wait for cert-manager-webhook to be ready, which can take time if cert-manager
	// was re-installed after uninstalling on a cluster.
	cmd = exec.Command("kubectl", "wait", "deployment.apps/cert-manager-webhook",
		"--for", "condition=Available",
		"--namespace", "cert-manager",
		"--timeout", "5m",
	)

	_, err := Run(cmd)
	return err
}

// IsCertManagerCRDsInstalled checks if any Cert Manager CRDs are installed
// by verifying the existence of key CRDs related to Cert Manager.
func IsCertManagerCRDsInstalled() bool {
	// List of common Cert Manager CRDs
	certManagerCRDs := []string{
		"certificates.cert-manager.io",
		"issuers.cert-manager.io",
		"clusterissuers.cert-manager.io",
		"certificaterequests.cert-manager.io",
		"orders.acme.cert-manager.io",
		"challenges.acme.cert-manager.io",
	}

	// Execute the kubectl command to get all CRDs
	cmd := exec.Command("kubectl", "get", "crds")
	output, err := Run(cmd)
	if err != nil {
		return false
	}

	// Check if any of the Cert Manager CRDs are present
	crdList := GetNonEmptyLines(output)
	for _, crd := range certManagerCRDs {
		for _, line := range crdList {
			if strings.Contains(line, crd) {
				return true
			}
		}
	}

	return false
}

// LoadImageToKindClusterWithName loads a local docker image to the kind cluster
func LoadImageToKindClusterWithName(name string) error {
	cluster := "kind"
	if v, ok := os.LookupEnv("KIND_CLUSTER"); ok {
		cluster = v
	}
	kindOptions := []string{"load", "docker-image", name, "--name", cluster}
	cmd := exec.Command("kind", kindOptions...)
	_, err := Run(cmd)
	return err
}

// GetNonEmptyLines converts given command output string into individual objects
// according to line breakers, and ignores the empty elements in it.
func GetNonEmptyLines(output string) []string {
	var res []string
	elements := strings.Split(output, "\n")
	for _, element := range elements {
		if element != "" {
			res = append(res, element)
		}
	}

	return res
}

// GetProjectDir will return the directory where the project is
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, err
	}
	wd = strings.Replace(wd, "/test/e2e", "", -1)
	return wd, nil
}

// UncommentCode searches for target in the file and remove the comment prefix
// of the target content. The target content may span multiple lines.
func UncommentCode(filename, target, prefix string) error {
	// false positive
	// nolint:gosec
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	strContent := string(content)

	idx := strings.Index(strContent, target)
	if idx < 0 {
		return fmt.Errorf("unable to find the code %s to be uncomment", target)
	}

	out := new(bytes.Buffer)
	_, err = out.Write(content[:idx])
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(bytes.NewBufferString(target))
	if !scanner.Scan() {
		return nil
	}
	for {
		_, err := out.WriteString(strings.TrimPrefix(scanner.Text(), prefix))
		if err != nil {
			return err
		}
		// Avoid writing a newline in case the previous line was the last in target.
		if !scanner.Scan() {
			break
		}
		if _, err := out.WriteString("\n"); err != nil {
			return err
		}
	}

	_, err = out.Write(content[idx+len(target):])
	if err != nil {
		return err
	}
	// false positive
	// nolint:gosec
	return os.WriteFile(filename, out.Bytes(), 0644)
}
