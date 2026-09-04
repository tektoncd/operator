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

package common

import (
	"context"
	"fmt"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
)

const (
	// MetricsClientCAConfigMap is the name of the ConfigMap synced into each
	// component namespace so that the Prometheus metrics server can verify
	// the client certificate presented by Prometheus during mTLS scraping.
	MetricsClientCAConfigMap = "metrics-client-ca"

	// MetricsClientCAKey is the data key within MetricsClientCAConfigMap that
	// holds the PEM-encoded CA bundle.
	MetricsClientCAKey = "client-ca-file"

	// SourceConfigMapName is the authoritative ConfigMap in kube-system that
	// contains the Prometheus client CA bundle on OpenShift.
	//
	// When scrapeClass: tls-client-certificate-auth is set on a ServiceMonitor,
	// CMO configures Prometheus to present metrics-client-certs as the scraping
	// client certificate. That cert is signed by the kubernetes.io/kube-apiserver-client
	// signer (kubelet-signer), which is included in extension-apiserver-authentication.
	SourceConfigMapName = "extension-apiserver-authentication"
	SystemNamespace     = "kube-system"
)

// ResolveMetricsMTLS is the single call components make in PreReconcile to
// determine whether mTLS transformers should be applied this cycle.
//
// It reads TektonConfig directly via a live API call (not the lister) to avoid
// a race condition where the component reconciler fires between the TektonConfig
// platform-data-hash annotation being updated and the TektonConfig informer
// cache being refreshed. Using a stale lister in that window would cause the
// component to stamp the new hash with stale mTLS state, making subsequent
// reconciles see a matching hash and never correcting the resources.
func ResolveMetricsMTLS(ctx context.Context, operatorClient versioned.Interface, kubeClient kubernetes.Interface, targetNamespace string) (bool, error) {
	tc, err := operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("reading TektonConfig: %w", err)
	}
	if tc.Spec.Platforms.OpenShift.EnableMetricsMTLS == nil || !*tc.Spec.Platforms.OpenShift.EnableMetricsMTLS {
		return false, nil
	}

	_, err = kubeClient.CoreV1().ConfigMaps(targetNamespace).Get(
		ctx, MetricsClientCAConfigMap, metav1.GetOptions{})
	if err == nil {
		return true, nil
	}
	if errors.IsNotFound(err) {
		return false, nil
	}
	return false, fmt.Errorf("checking %s/%s: %w", targetNamespace, MetricsClientCAConfigMap, err)
}

// IsMetricsMTLSEnabled reports whether spec.platforms.openshift.enableMetricsMTLS
// is explicitly set to true in the TektonConfig CR.
// Returns false when the CR cannot be read or the field is nil (default = disabled).
func IsMetricsMTLSEnabled(lister TektonConfigLister) bool {
	tc, err := lister.Get(v1alpha1.ConfigResourceName)
	if err != nil {
		return false
	}
	return tc.Spec.Platforms.OpenShift.EnableMetricsMTLS != nil &&
		*tc.Spec.Platforms.OpenShift.EnableMetricsMTLS
}

// EnsureMetricsClientCA reads the Prometheus client CA from
// kube-system/extension-apiserver-authentication and creates-or-updates
// the metrics-client-ca ConfigMap in targetNamespace.
//
// The ConfigMap is mounted by Tekton component containers so that the
// knative prometheus.Server can verify the Prometheus client certificate
// during mTLS scraping (METRICS_PROMETHEUS_TLS_CLIENT_CA_FILE).
func EnsureMetricsClientCA(ctx context.Context, kubeClient kubernetes.Interface, targetNamespace string) error {
	logger := logging.FromContext(ctx)

	src, err := kubeClient.CoreV1().ConfigMaps(SystemNamespace).Get(
		ctx, SourceConfigMapName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return fmt.Errorf(
			"%s/%s not found — the OpenShift Cluster Monitoring Operator (CMO) may not be "+
				"installed on this cluster. To resolve, either install CMO or disable metrics mTLS "+
				"by setting spec.platforms.openshift.enableMetricsMTLS: false in TektonConfig",
			SystemNamespace, SourceConfigMapName)
	}
	if err != nil {
		return fmt.Errorf("reading %s/%s: %w", SystemNamespace, SourceConfigMapName, err)
	}

	caBundle, ok := src.Data[MetricsClientCAKey]
	if !ok {
		return fmt.Errorf("%s/%s has no %q key", SystemNamespace, SourceConfigMapName, MetricsClientCAKey)
	}

	cmClient := kubeClient.CoreV1().ConfigMaps(targetNamespace)

	existing, err := cmClient.Get(ctx, MetricsClientCAConfigMap, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("getting %s/%s: %w", targetNamespace, MetricsClientCAConfigMap, err)
	}

	if errors.IsNotFound(err) {
		logger.Infof("Creating %s/%s", targetNamespace, MetricsClientCAConfigMap)
		desired := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      MetricsClientCAConfigMap,
				Namespace: targetNamespace,
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "tekton-pipelines",
				},
			},
			Data: map[string]string{
				MetricsClientCAKey: caBundle,
			},
		}
		_, err = cmClient.Create(ctx, desired, metav1.CreateOptions{})
		return err
	}

	// Already exists — update only if the CA bundle has changed.
	if existing.Data[MetricsClientCAKey] == caBundle {
		logger.Debugf("%s/%s is up to date", targetNamespace, MetricsClientCAConfigMap)
		return nil
	}

	logger.Infof("Updating %s/%s (CA bundle changed)", targetNamespace, MetricsClientCAConfigMap)
	updated := existing.DeepCopy()
	if updated.Data == nil {
		updated.Data = map[string]string{}
	}
	updated.Data[MetricsClientCAKey] = caBundle
	_, err = cmClient.Update(ctx, updated, metav1.UpdateOptions{})
	return err
}
