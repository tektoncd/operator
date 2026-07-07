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
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

const testCABundle = "-----BEGIN CERTIFICATE-----\nMIIBtest\n-----END CERTIFICATE-----\n"

// sourceConfigMap returns a pre-populated kube-system/extension-apiserver-authentication
// ConfigMap that EnsureMetricsClientCA reads from.
func sourceConfigMap(caBundle string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SourceConfigMapName,
			Namespace: SystemNamespace,
		},
		Data: map[string]string{
			MetricsClientCAKey: caBundle,
		},
	}
}

func TestEnsureMetricsClientCA_Creates(t *testing.T) {
	client := kubefake.NewSimpleClientset(sourceConfigMap(testCABundle))
	ctx := context.Background()

	assert.NilError(t, EnsureMetricsClientCA(ctx, client, "openshift-pipelines"))

	cm, err := client.CoreV1().ConfigMaps("openshift-pipelines").Get(
		ctx, MetricsClientCAConfigMap, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, testCABundle, cm.Data[MetricsClientCAKey])
	assert.Equal(t, "tekton-pipelines", cm.Labels["app.kubernetes.io/part-of"])
}

func TestEnsureMetricsClientCA_NoOp_WhenUpToDate(t *testing.T) {
	existing := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MetricsClientCAConfigMap,
			Namespace: "openshift-pipelines",
		},
		Data: map[string]string{MetricsClientCAKey: testCABundle},
	}
	client := kubefake.NewSimpleClientset(sourceConfigMap(testCABundle), existing)
	ctx := context.Background()

	// Record the resource version before the call.
	before, err := client.CoreV1().ConfigMaps("openshift-pipelines").Get(
		ctx, MetricsClientCAConfigMap, metav1.GetOptions{})
	assert.NilError(t, err)

	assert.NilError(t, EnsureMetricsClientCA(ctx, client, "openshift-pipelines"))

	after, err := client.CoreV1().ConfigMaps("openshift-pipelines").Get(
		ctx, MetricsClientCAConfigMap, metav1.GetOptions{})
	assert.NilError(t, err)

	// ResourceVersion should be unchanged (no update was issued).
	assert.Equal(t, before.ResourceVersion, after.ResourceVersion,
		"ConfigMap should not have been updated when CA bundle is unchanged")
}

func TestEnsureMetricsClientCA_Updates_WhenStale(t *testing.T) {
	const updatedBundle = "-----BEGIN CERTIFICATE-----\nMIIBupdated\n-----END CERTIFICATE-----\n"

	existing := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MetricsClientCAConfigMap,
			Namespace: "openshift-pipelines",
		},
		Data: map[string]string{MetricsClientCAKey: "old-bundle"},
	}
	client := kubefake.NewSimpleClientset(sourceConfigMap(updatedBundle), existing)
	ctx := context.Background()

	assert.NilError(t, EnsureMetricsClientCA(ctx, client, "openshift-pipelines"))

	cm, err := client.CoreV1().ConfigMaps("openshift-pipelines").Get(
		ctx, MetricsClientCAConfigMap, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, updatedBundle, cm.Data[MetricsClientCAKey],
		"ConfigMap should have been updated with the new CA bundle")
}

func TestEnsureMetricsClientCA_ErrorWhenSourceMissing(t *testing.T) {
	// No source ConfigMap pre-seeded.
	client := kubefake.NewSimpleClientset()
	ctx := context.Background()

	err := EnsureMetricsClientCA(ctx, client, "openshift-pipelines")
	assert.ErrorContains(t, err, SourceConfigMapName)
}

func TestEnsureMetricsClientCA_ErrorWhenKeyMissing(t *testing.T) {
	srcNoKey := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SourceConfigMapName,
			Namespace: SystemNamespace,
		},
		Data: map[string]string{
			"some-other-key": "value",
		},
	}
	client := kubefake.NewSimpleClientset(srcNoKey)
	ctx := context.Background()

	err := EnsureMetricsClientCA(ctx, client, "openshift-pipelines")
	assert.ErrorContains(t, err, MetricsClientCAKey)
}

// ---------------------------------------------------------------------------
// IsMetricsMTLSEnabled
// ---------------------------------------------------------------------------

// stubTektonConfigLister implements TektonConfigLister for testing.
type stubTektonConfigLister struct {
	tc  *v1alpha1.TektonConfig
	err error
}

func (s *stubTektonConfigLister) Get(_ string) (*v1alpha1.TektonConfig, error) {
	return s.tc, s.err
}

func newTektonConfigWithMTLS(enabled *bool) *v1alpha1.TektonConfig {
	return &v1alpha1.TektonConfig{
		Spec: v1alpha1.TektonConfigSpec{
			Platforms: v1alpha1.Platforms{
				OpenShift: v1alpha1.OpenShift{
					EnableMetricsMTLS: enabled,
				},
			},
		},
	}
}

func boolPtr(b bool) *bool { return &b }

func TestIsMetricsMTLSEnabled_ReturnsTrueWhenExplicitlySet(t *testing.T) {
	lister := &stubTektonConfigLister{tc: newTektonConfigWithMTLS(boolPtr(true))}
	assert.Assert(t, IsMetricsMTLSEnabled(lister))
}

func TestIsMetricsMTLSEnabled_ReturnsFalseWhenExplicitlyFalse(t *testing.T) {
	lister := &stubTektonConfigLister{tc: newTektonConfigWithMTLS(boolPtr(false))}
	assert.Assert(t, !IsMetricsMTLSEnabled(lister))
}

func TestIsMetricsMTLSEnabled_ReturnsFalseWhenNil(t *testing.T) {
	lister := &stubTektonConfigLister{tc: newTektonConfigWithMTLS(nil)}
	assert.Assert(t, !IsMetricsMTLSEnabled(lister))
}

func TestIsMetricsMTLSEnabled_ReturnsFalseOnListerError(t *testing.T) {
	lister := &stubTektonConfigLister{err: fmt.Errorf("not found")}
	assert.Assert(t, !IsMetricsMTLSEnabled(lister))
}

func TestEnsureMetricsClientCA_DifferentTargetNamespaces(t *testing.T) {
	namespaces := []string{"openshift-pipelines", "tekton-chains", "tekton-results"}
	client := kubefake.NewSimpleClientset(sourceConfigMap(testCABundle))
	ctx := context.Background()

	for _, ns := range namespaces {
		assert.NilError(t, EnsureMetricsClientCA(ctx, client, ns))

		cm, err := client.CoreV1().ConfigMaps(ns).Get(
			ctx, MetricsClientCAConfigMap, metav1.GetOptions{})
		assert.NilError(t, err, "namespace %q", ns)
		assert.Equal(t, testCABundle, cm.Data[MetricsClientCAKey], "namespace %q", ns)
	}
}
