//go:build e2e
// +build e2e

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
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestTektonMulticlusterProxyAAENetworkPolicy(t *testing.T) {
	crNames := utils.GetResourceNames()
	clients := client.Setup(t, crNames.TargetNamespace)

	if !utils.IsOpenShift() {
		t.Skip("MultiCluster components are OpenShift-only, skipping")
	}
	if _, err := clients.KubeClientSet.Discovery().ServerResourcesForGroupVersion("kueue.x-k8s.io/v1beta1"); err != nil {
		t.Skipf("Kueue API (kueue.x-k8s.io/v1beta1) not available, skipping: %v", err)
	}

	utils.CleanupOnInterrupt(func() { utils.TearDownMulticlusterProxyAAE(clients, crNames.TektonMulticlusterProxyAAE) })
	utils.CleanupOnInterrupt(func() { utils.TearDownScheduler(clients, crNames.TektonScheduler) })
	defer utils.TearDownMulticlusterProxyAAE(clients, crNames.TektonMulticlusterProxyAAE)
	defer utils.TearDownScheduler(clients, crNames.TektonScheduler)

	utils.CleanupOnInterrupt(func() { resources.DeleteDummyWorkerCluster(clients) })
	defer resources.DeleteDummyWorkerCluster(clients)

	resources.EnsureNoTektonConfigInstance(t, clients, crNames)

	if _, err := clients.TektonConfig().Create(context.TODO(), &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{Name: crNames.TektonConfig},
		Spec: v1alpha1.TektonConfigSpec{
			Profile:    v1alpha1.ProfileAll,
			CommonSpec: v1alpha1.CommonSpec{TargetNamespace: crNames.TargetNamespace},
			Scheduler: v1alpha1.Scheduler{
				Disabled: ptr.To(false),
				MultiClusterConfig: v1alpha1.MultiClusterConfig{
					MultiClusterDisabled: false,
					MultiClusterRole:     v1alpha1.MultiClusterRoleHub,
				},
			},
		},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("TektonConfig %q failed to create: %v", crNames.TektonConfig, err)
	}

	resources.EnsureDummyWorkerCluster(t, clients)

	resources.AssertTektonConfigCRReadyStatus(t, clients, crNames)
	resources.AssertTektonSchedulerCRReadyStatus(t, clients, crNames)
	resources.AssertTektonMulticlusterProxyAAECRReadyStatus(t, clients, crNames)

	expectedPolicies := []string{
		"proxy-aae-default-deny",
		"proxy-aae",
	}

	t.Run("default-policies-created", func(t *testing.T) {
		resources.AssertNetworkPoliciesExist(t, clients, crNames.TargetNamespace, expectedPolicies)
	})

	t.Run("disable-removes-policies", func(t *testing.T) {
		proxy, err := clients.TektonMulticlusterProxyAAEs().Get(context.TODO(), crNames.TektonMulticlusterProxyAAE, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get TektonMulticlusterProxyAAE: %v", err)
		}
		proxy.Spec.NetworkPolicy.Disabled = true
		if _, err := clients.TektonMulticlusterProxyAAEs().Update(context.TODO(), proxy, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("failed to disable NetworkPolicy on TektonMulticlusterProxyAAE: %v", err)
		}
		resources.AssertTektonMulticlusterProxyAAECRReadyStatus(t, clients, crNames)
		resources.AssertNetworkPoliciesAbsent(t, clients, crNames.TargetNamespace, expectedPolicies)
	})

	t.Run("reenable-restores-policies", func(t *testing.T) {
		proxy, err := clients.TektonMulticlusterProxyAAEs().Get(context.TODO(), crNames.TektonMulticlusterProxyAAE, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get TektonMulticlusterProxyAAE: %v", err)
		}
		proxy.Spec.NetworkPolicy.Disabled = false
		if _, err := clients.TektonMulticlusterProxyAAEs().Update(context.TODO(), proxy, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("failed to re-enable NetworkPolicy on TektonMulticlusterProxyAAE: %v", err)
		}
		resources.AssertTektonMulticlusterProxyAAECRReadyStatus(t, clients, crNames)
		resources.AssertNetworkPoliciesExist(t, clients, crNames.TargetNamespace, expectedPolicies)
	})
}
