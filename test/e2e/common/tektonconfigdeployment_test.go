// +build e2e

/*
Copyright 2020 The Tekton Authors

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
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektonpipeline"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektontrigger"
	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/ptr"
)

// TestTektonConfigDeployment does following checks
// - make sure TektonConfig is created if AUTOINSTALL_COMPONENTS is true
// - waits till TektonConfig ready status becomes true
// - in case of OpenShift, runs rbac and addon test
// - changes profile from all to basic and validates changes
// - deletes a components and make sure it is recreated
// - updates field in TektonConfig Spec and make sure they are reflected in configmaps
// - Deletes and recreates TektonConfig and validates webhook adds all defaults
func TestTektonConfigDeployment(t *testing.T) {
	clients := client.Setup(t)

	crNames := utils.ResourceNames{
		TektonConfig:    v1alpha1.ConfigResourceName,
		Namespace:       "tekton-operator",
		TargetNamespace: "tekton-pipelines",
	}

	platform := os.Getenv("TARGET")

	if platform == "openshift" {
		crNames.Namespace = "openshift-operators"
		crNames.TargetNamespace = "openshift-pipelines"
	}

	utils.CleanupOnInterrupt(func() { utils.TearDownConfig(clients, crNames.TektonConfig) })
	defer utils.TearDownConfig(clients, crNames.TektonConfig)

	var (
		tc  *v1alpha1.TektonConfig
		err error
	)

	// Create a TektonConfig
	t.Run("create-config", func(t *testing.T) {
		tc, err = resources.EnsureTektonConfigExists(clients.KubeClientSet, clients.TektonConfig(), crNames)
		if err != nil {
			t.Fatalf("TektonConfig %q failed to create: %v", crNames.TektonConfig, err)
		}
	})

	// Test if TektonConfig can reach the READY status
	t.Run("ensure-config-ready-status", func(t *testing.T) {
		resources.AssertTektonConfigCRReadyStatus(t, clients, crNames)
	})

	if platform == "openshift" {
		runRbacTest(t, clients)
	}

	if platform == "openshift" && tc.Spec.Profile == v1alpha1.ProfileAll {
		runAddonTest(t, clients, tc)
	}

	runFeatureTest(t, clients, tc, crNames)

	// Delete the TektonConfig CR instance to see if all resources will be removed
	t.Run("delete-config", func(t *testing.T) {
		resources.AssertTektonConfigCRReadyStatus(t, clients, crNames)
		resources.TektonConfigCRDelete(t, clients, crNames)
	})
}

func runFeatureTest(t *testing.T, clients *utils.Clients, tc *v1alpha1.TektonConfig, names utils.ResourceNames) {

	t.Run("change-profile", func(t *testing.T) {

		tc, err := clients.Operator.TektonConfigs().Get(context.TODO(), v1alpha1.ConfigResourceName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get tektonconfig: %v", err)
		}

		// make sure dashboard is created in case of k8s and
		// addon in case of openshift
		if os.Getenv("TARGET") == "openshift" {
			if _, err := clients.Operator.TektonAddons().Get(context.TODO(), v1alpha1.AddonResourceName, metav1.GetOptions{}); err != nil {
				t.Fatalf("failed to get tektonaddon")
			}
		} else {
			if _, err := clients.Operator.TektonDashboards().Get(context.TODO(), v1alpha1.DashboardResourceName, metav1.GetOptions{}); err != nil {
				t.Fatalf("failed to get dashboard")
			}
		}

		// change the profile and make sure it is reflected on the cluster
		// ALL -> BASIC
		tc.Spec.Profile = v1alpha1.ProfileBasic

		tc, err = clients.Operator.TektonConfigs().Update(context.TODO(), tc, metav1.UpdateOptions{})
		if err != nil {
			t.Fatalf("failed to update tektonconfig: %v", err)
		}

		if tc.Spec.Profile != v1alpha1.ProfileBasic {
			t.Fatal("failed to change profile in TektonConfig")
		}

		// wait till the component is deleted
		time.Sleep(time.Second * 20)

		// now, make sure dashboard is deleted in case of k8s and
		// addon is deleted in case of openshift
		if os.Getenv("TARGET") == "openshift" {
			if _, err := clients.Operator.TektonAddons().Get(context.TODO(), v1alpha1.AddonResourceName, metav1.GetOptions{}); err == nil {
				t.Fatalf("expected error but got nil, tektonaddon not deleted")
			}
		} else {
			if _, err := clients.Operator.TektonDashboards().Get(context.TODO(), v1alpha1.DashboardResourceName, metav1.GetOptions{}); err == nil {
				t.Fatalf("expected error but got nil, tektondashboard not deleted")
			}
		}
	})

	t.Run("change-spec-configuration-and-validate", func(t *testing.T) {

		tc, err := clients.Operator.TektonConfigs().Get(context.TODO(), v1alpha1.ConfigResourceName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get tektonconfig: %v", err)
		}

		// Change spec field and check if it has changed in the configmap for components

		// pipelines feature-flags configMap
		tc.Spec.Pipeline.PipelineProperties.EnableCustomTasks = ptr.Bool(true)
		tc.Spec.Pipeline.PipelineProperties.EnableTektonOciBundles = ptr.Bool(true)

		// pipeline config-defaults configMap
		tc.Spec.Pipeline.OptionalPipelineProperties.DefaultServiceAccount = "foo"

		// triggers feature-flags configMap
		tc.Spec.Trigger.TriggersProperties.EnableApiFields = v1alpha1.ApiFieldAlpha

		// triggers config-defaults configMap
		tc.Spec.Trigger.OptionalTriggersProperties.DefaultServiceAccount = "foo"

		tc, err = clients.Operator.TektonConfigs().Update(context.TODO(), tc, metav1.UpdateOptions{})
		if err != nil {
			t.Fatalf("failed to update tektonconfig: %v", err)
		}

		// wait for a few seconds and it reconcile
		time.Sleep(time.Second * 5)

		// Validate changes to Pipelines ConfigMaps

		featureFlags, err := clients.KubeClient.CoreV1().ConfigMaps(tc.Spec.TargetNamespace).Get(context.TODO(), tektonpipeline.FeatureFlag, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get pipelines configMap: %s : %v", tektonpipeline.FeatureFlag, err)
		}

		if featureFlags.Data["enable-custom-tasks"] != "true" || featureFlags.Data["enable-tekton-oci-bundles"] != "true" {
			t.Fatalf("failed to update changes to pipelines configMap: %s ", tektonpipeline.FeatureFlag)
		}

		configDefaults, err := clients.KubeClient.CoreV1().ConfigMaps(tc.Spec.TargetNamespace).Get(context.TODO(), tektonpipeline.ConfigDefaults, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get pipelines configMap: %s : %v", tektonpipeline.ConfigDefaults, err)
		}

		if configDefaults.Data["default-service-account"] != "foo" {
			t.Fatalf("failed to update changes to pipelines configMap: %s ", tektonpipeline.ConfigDefaults)
		}

		// Validate changes to Triggers ConfigMaps

		featureFlags, err = clients.KubeClient.CoreV1().ConfigMaps(tc.Spec.TargetNamespace).Get(context.TODO(), tektontrigger.FeatureFlag, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get triggers configMap: %s : %v", tektontrigger.FeatureFlag, err)
		}

		if featureFlags.Data["enable-api-fields"] != v1alpha1.ApiFieldAlpha {
			t.Fatalf("failed to update changes to triggers configMap: %s", tektontrigger.FeatureFlag)
		}

		configDefaults, err = clients.KubeClient.CoreV1().ConfigMaps(tc.Spec.TargetNamespace).Get(context.TODO(), tektontrigger.ConfigDefaults, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get triggers configMap: %s : %v", tektontrigger.ConfigDefaults, err)
		}

		if configDefaults.Data["default-service-account"] != "foo" {
			t.Fatalf("failed to update changes to triggers configMap: %s :", tektontrigger.ConfigDefaults)
		}
	})

	t.Run("delete-component-and-recreate", func(t *testing.T) {

		// delete a component and make sure it is recreated
		if err := clients.Operator.TektonPipelines().Delete(context.TODO(), v1alpha1.PipelineResourceName, metav1.DeleteOptions{}); err != nil {
			t.Fatalf("failed to get delete tektonpipeline")
		}

		// wait till the component is recreated
		time.Sleep(time.Second * 20)

		// component must be recreated
		if _, err := clients.Operator.TektonPipelines().Get(context.TODO(), v1alpha1.PipelineResourceName, metav1.GetOptions{}); err != nil {
			t.Fatalf("failed to get tektonpipeline, component not recreated")
		}
	})

	t.Run("delete-current-config", func(t *testing.T) {
		resources.AssertTektonConfigCRReadyStatus(t, clients, names)
		resources.TektonConfigCRDelete(t, clients, names)
	})

	t.Run("create-new-config-and-let-webhook-add-defaults", func(t *testing.T) {

		tc := &v1alpha1.TektonConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: v1alpha1.ConfigResourceName,
			},
			Spec: v1alpha1.TektonConfigSpec{
				Profile: v1alpha1.ProfileAll,
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: names.TargetNamespace,
				},
			},
		}

		var err error
		tc, err = clients.Operator.TektonConfigs().Create(context.TODO(), tc, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create tektonconfig: %v", err)
		}

		if d := cmp.Diff(tc.Spec.Pipeline, v1alpha1.Pipeline{}); d == "" {
			t.Fatalf("expected defaulting for pipeline properties but failed: %v", d)
		}

		if d := cmp.Diff(tc.Spec.Trigger, v1alpha1.Trigger{}); d == "" {
			t.Fatalf("expected defaulting for triggers properties but failed: %v", d)
		}
	})
}

func runAddonTest(t *testing.T, clients *utils.Clients, tc *v1alpha1.TektonConfig) {

	var (
		addon *v1alpha1.TektonAddon
		err   error
	)

	// Make sure TektonAddon is created
	t.Run("ensure-addon-is-created", func(t *testing.T) {
		addon, err = clients.Operator.TektonAddons().Get(context.TODO(), v1alpha1.AddonResourceName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get TektonAddon CR: %s : %v", v1alpha1.AddonResourceName, err)
		}
	})

	// Check if number of params passed in TektonConfig would be passed in TektonAddons
	t.Run("check-addon-params", func(t *testing.T) {
		if d := cmp.Diff(tc.Spec.Addon.Params, addon.Spec.Params); d != "" {
			t.Errorf("Addon params in TektonConfig not equal to TektonAddon params: %s", diff.PrintWantGot(d))
		}
	})

	t.Run("validate-addon-params", func(t *testing.T) {

		ls := metav1.LabelSelector{
			MatchLabels: map[string]string{
				v1alpha1.CreatedByKey: "TektonAddon",
			},
		}
		labelSelector, err := common.LabelSelector(ls)
		if err != nil {
			t.Fatal(err)
		}

		addonsIS, err := clients.Operator.TektonInstallerSets().List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			t.Fatalf("failed to get InstallerSet: %v", err)
		}

		if len(addonsIS.Items) != 7 {
			t.Fatalf("expected 7 installerSets for Addon but got %v", len(addonsIS.Items))
		}

		// Now, disable clusterTasks and pipelineTemplates through TektonConfig
		tc, err := clients.Operator.TektonConfigs().Get(context.TODO(), v1alpha1.ConfigResourceName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get tektonconfig: %v", err)
		}
		tc.Spec.Addon.Params = []v1alpha1.Param{
			{
				Name:  v1alpha1.ClusterTasksParam,
				Value: "false",
			},
			{
				Name:  v1alpha1.PipelineTemplatesParam,
				Value: "false",
			},
		}
		tc, err = clients.Operator.TektonConfigs().Update(context.TODO(), tc, metav1.UpdateOptions{})
		if err != nil {
			t.Fatalf("failed to update tektonconfig: %v", err)
		}

		// wait till the installer set is deleted
		time.Sleep(time.Second * 10)

		addonsIS, err = clients.Operator.TektonInstallerSets().List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			t.Fatalf("failed to get InstallerSet: %v", err)
		}
		// Now, there must be 4 installerSet
		if len(addonsIS.Items) != 4 {
			t.Fatalf("expected 4 installerSets after disabling params for Addon but got %v", len(addonsIS.Items))
		}
	})

	t.Run("disable-pac", func(t *testing.T) {

		tc, err := clients.Operator.TektonConfigs().Get(context.TODO(), v1alpha1.ConfigResourceName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get tektonconfig: %v", err)
		}

		// Now, disable pipelinesAsCode
		tc.Spec.Addon.EnablePAC = ptr.Bool(false)

		tc, err = clients.Operator.TektonConfigs().Update(context.TODO(), tc, metav1.UpdateOptions{})
		if err != nil {
			t.Fatalf("failed to update tektonconfig: %v", err)
		}

		// wait till the installer set is deleted
		time.Sleep(time.Second * 10)

		ls := metav1.LabelSelector{
			MatchLabels: map[string]string{
				v1alpha1.CreatedByKey: "TektonAddon",
			},
		}
		labelSelector, err := common.LabelSelector(ls)
		if err != nil {
			t.Fatal(err)
		}
		addonsIS, err := clients.Operator.TektonInstallerSets().List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			t.Fatalf("failed to get InstallerSet: %v", err)
		}
		// Now, there must be 3 installerSet
		if len(addonsIS.Items) != 3 {
			t.Fatalf("expected 3 installerSets after disabling pac for Addon but got %v", len(addonsIS.Items))
		}
	})
}

func runRbacTest(t *testing.T, clients *utils.Clients) {

	// Test whether the supporting rbac resources are created for existing namespace and
	// newly created namespace

	existingNamespace := "default"
	testNamespace := "operator-test-rbac"

	// Create a Test Namespace
	if _, err := resources.EnsureTestNamespaceExists(clients, testNamespace); err != nil {
		t.Fatalf("failed to create test namespace: %s, %q", testNamespace, err)
	}

	clusterRoleName := "pipelines-scc-clusterrole"

	t.Run("verify-clusterrole", func(t *testing.T) {
		resources.AssertClusterRole(t, clients, clusterRoleName)
	})

	expectedSAName := "pipeline"

	// Test whether the `pipelineSa` is created in a "default" namespace
	t.Run("verify-service-account", func(t *testing.T) {
		resources.AssertServiceAccount(t, clients, existingNamespace, expectedSAName)
		resources.AssertServiceAccount(t, clients, testNamespace, expectedSAName)
	})

	serviceCABundleConfigMap := "config-service-cabundle"
	trustedCABundleConfigMap := "config-trusted-cabundle"

	// Test whether the configMaps are created
	t.Run("verify-configmaps", func(t *testing.T) {
		resources.AssertConfigMap(t, clients, existingNamespace, serviceCABundleConfigMap)
		resources.AssertConfigMap(t, clients, testNamespace, trustedCABundleConfigMap)
		resources.AssertConfigMap(t, clients, existingNamespace, serviceCABundleConfigMap)
		resources.AssertConfigMap(t, clients, testNamespace, trustedCABundleConfigMap)
	})

	pipelinesSCCRoleBinding := "pipelines-scc-rolebinding"
	editRoleBinding := "edit"

	// Test whether the roleBindings are created
	t.Run("verify-rolebindings", func(t *testing.T) {
		resources.AssertRoleBinding(t, clients, existingNamespace, pipelinesSCCRoleBinding)
		resources.AssertRoleBinding(t, clients, testNamespace, pipelinesSCCRoleBinding)
		resources.AssertRoleBinding(t, clients, existingNamespace, editRoleBinding)
		resources.AssertRoleBinding(t, clients, testNamespace, editRoleBinding)
	})

}
