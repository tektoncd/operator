/*
Copyright 2026 The Tekton Authors

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

package resources

import (
	"context"
	"os"
	"testing"

	"github.com/tektoncd/operator/test/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	dummyWorkerSecretName  = "e2e-dummy-worker"
	dummyWorkerClusterName = "e2e-dummy-worker"
	kueueNamespaceK8s      = "kueue-system"
	kueueNamespaceOCP      = "openshift-kueue-operator"
)

var multiKueueClusterGVR = schema.GroupVersionResource{
	Group:    "kueue.x-k8s.io",
	Version:  "v1beta2",
	Resource: "multikueueclusters",
}

// KueueNamespace returns the namespace where Kueue components run.
func KueueNamespace() string {
	if utils.IsOpenShift() {
		return kueueNamespaceOCP
	}
	return kueueNamespaceK8s
}

// EnsureDummyWorkerCluster creates a self-referential kubeconfig secret and
// MultiKueueCluster CR so that proxy-aae can detect at least one worker
// cluster and pass its readiness probe (/ready).
func EnsureDummyWorkerCluster(t *testing.T, clients *utils.Clients) {
	t.Helper()

	ns := KueueNamespace()
	cfg := clients.Config

	// Resolve the cluster CA — Kueue rejects insecure-skip-tls-verify.
	caData := cfg.CAData
	if len(caData) == 0 && cfg.CAFile != "" {
		var err error
		caData, err = os.ReadFile(cfg.CAFile)
		if err != nil {
			t.Fatalf("failed to read CA file %s: %v", cfg.CAFile, err)
		}
	}
	if len(caData) == 0 {
		cm, err := clients.KubeClient.CoreV1().ConfigMaps(ns).Get(
			context.TODO(), "kube-root-ca.crt", metav1.GetOptions{},
		)
		if err != nil {
			t.Fatalf("failed to get kube-root-ca.crt ConfigMap in %s: %v", ns, err)
		}
		caData = []byte(cm.Data["ca.crt"])
	}

	cluster := clientcmdapi.NewCluster()
	cluster.Server = cfg.Host
	cluster.CertificateAuthorityData = caData

	authInfo := clientcmdapi.NewAuthInfo()
	authInfo.Token = cfg.BearerToken
	authInfo.ClientCertificateData = cfg.CertData
	authInfo.ClientKeyData = cfg.KeyData

	kubeConfig := clientcmdapi.NewConfig()
	kubeConfig.Clusters[dummyWorkerClusterName] = cluster
	kubeConfig.AuthInfos[dummyWorkerClusterName] = authInfo
	kubeConfig.Contexts[dummyWorkerClusterName] = &clientcmdapi.Context{
		Cluster:  dummyWorkerClusterName,
		AuthInfo: dummyWorkerClusterName,
	}
	kubeConfig.CurrentContext = dummyWorkerClusterName

	kubeconfigBytes, err := clientcmd.Write(*kubeConfig)
	if err != nil {
		t.Fatalf("failed to serialize dummy worker kubeconfig: %v", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dummyWorkerSecretName,
			Namespace: ns,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"kubeconfig": kubeconfigBytes,
		},
	}
	_, err = clients.KubeClient.CoreV1().Secrets(ns).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		t.Fatalf("failed to create dummy worker secret in %s: %v", ns, err)
	}

	mkc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kueue.x-k8s.io/v1beta2",
			"kind":       "MultiKueueCluster",
			"metadata": map[string]interface{}{
				"name": dummyWorkerClusterName,
			},
			"spec": map[string]interface{}{
				"clusterSource": map[string]interface{}{
					"kubeConfig": map[string]interface{}{
						"locationType": "Secret",
						"location":     dummyWorkerSecretName,
					},
				},
			},
		},
	}
	_, err = clients.Dynamic.Resource(multiKueueClusterGVR).Create(context.TODO(), mkc, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		t.Fatalf("failed to create MultiKueueCluster %s: %v", dummyWorkerClusterName, err)
	}
}

// DeleteDummyWorkerCluster removes the dummy worker secret and
// MultiKueueCluster created by EnsureDummyWorkerCluster.
func DeleteDummyWorkerCluster(clients *utils.Clients) {
	if clients == nil {
		return
	}
	if clients.Dynamic != nil {
		_ = clients.Dynamic.Resource(multiKueueClusterGVR).Delete(
			context.TODO(), dummyWorkerClusterName, metav1.DeleteOptions{},
		)
	}
	if clients.KubeClient != nil {
		ns := KueueNamespace()
		_ = clients.KubeClient.CoreV1().Secrets(ns).Delete(
			context.TODO(), dummyWorkerSecretName, metav1.DeleteOptions{},
		)
	}
}
