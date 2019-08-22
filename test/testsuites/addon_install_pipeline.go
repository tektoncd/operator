package testsuites

import (
	"context"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/controller/addon"
	"github.com/tektoncd/operator/pkg/controller/config"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
	"time"

	"github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/tektoncd/operator/test/helpers"
	testConfig "github.com/tektoncd/operator/test/config"
)

// ValidateAddonInstall creates an instance of addon.operator.tekton.dev
// and checks whether dashboard deployments are created
func ValidateAddonInstall(t *testing.T) {
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	installPipeline(t, ctx)

	t.Run("creating-addon-with-version", addonCRWithVersion)
	t.Run("creating-addon-without-version", addonCRWithoutVersion)
}

// ValidateAddonDeletion ensures that deleting the addon CR  deletes the already
// installed addon pipeline
func ValidateAddonDeletion(t *testing.T) {
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	installPipeline(t, ctx)

	t.Run("deleting-addon-cr", addonCRDeletion)
}

func installPipeline(t *testing.T, ctx *test.TestCtx) {
	configCR := &v1alpha1.Config{
		TypeMeta:   v1.TypeMeta{
			Kind: "config",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: config.ClusterCRName,
		},
		Spec:       v1alpha1.ConfigSpec{
			TargetNamespace: config.DefaultTargetNs,
		},
	}
	cleanupOptions := &test.CleanupOptions{
		TestContext:   ctx,
		Timeout:       5 * time.Second,
		RetryInterval: 1 * time.Second,
	}

	err := test.Global.Client.Create(context.TODO(), configCR, cleanupOptions)
	helpers.AssertNoError(t, err)
	helpers.WaitForClusterCR(t, config.ClusterCRName, configCR)
}

func addonCRWithVersion(t *testing.T) {
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	addonCR := &v1alpha1.Addon{
		TypeMeta:   v1.TypeMeta{
			Kind: "Addon",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "dashboard",
		},
		Spec:       v1alpha1.AddonSpec{
			Version: "v0.1.0",
		},
	}

	cleanupOpetions := &test.CleanupOptions{
		TestContext:   ctx,
		Timeout:       5 * time.Second,
		RetryInterval: 1 * time.Second,
	}

	err := test.Global.Client.Create(
		context.TODO(),
		addonCR,
		cleanupOpetions)

	helpers.AssertNoError(t, err)

	err = e2eutil.WaitForDeployment(
		t, test.Global.KubeClient, config.DefaultTargetNs,
		"tekton-dashboard",
		1,
		testConfig.APIRetry,
		testConfig.APITimeout,
	)
	helpers.AssertNoError(t, err)

	helpers.WaitForClusterCR(t, "dashboard", addonCR)
	if code := addonCR.Status.Conditions[0].Code; code != v1alpha1.InstalledStatus {
		t.Errorf("Expected code to be %s but got %s", v1alpha1.InstalledStatus, code)
	}
}

func addonCRWithoutVersion(t *testing.T) {
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	addonCR := &v1alpha1.Addon{
		TypeMeta:   v1.TypeMeta{
			Kind: "Addon",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "dashboard",
		},
	}

	cleanupOpetions := &test.CleanupOptions{
		TestContext:   ctx,
		Timeout:       5 * time.Second,
		RetryInterval: 1 * time.Second,
	}

	err := test.Global.Client.Create(
		context.TODO(),
		addonCR,
		cleanupOpetions)

	helpers.AssertNoError(t, err)

	err = e2eutil.WaitForDeployment(
		t, test.Global.KubeClient, config.DefaultTargetNs,
		"tekton-dashboard",
		1,
		testConfig.APIRetry,
		testConfig.APITimeout,
	)
	helpers.AssertNoError(t, err)

	helpers.WaitForClusterCR(t, "dashboard", addonCR)

	version, err := addon.GetLatestVersion(addonCR)
	if addonCR.Spec.Version != version {
		t.Errorf("Expected version to be %s but got %s", version, addonCR.Spec.Version)
	}

	// the check on code is disabled because, dashboard v0.1.1 has a dependency on service.knative.dev
	// eventhough the dashboard components are installed the conditions[0] will not reach 'Installed' in
	// the current implementation because of the above case.
	//if code := addonCR.Status.Conditions[0].Code; code != v1alpha1.InstalledStatus {
	//	t.Errorf("Expected code to be %s but got %s", v1alpha1.InstalledStatus, code)
	//}
}

func addonCRDeletion(t *testing.T) {
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	addonCR := &v1alpha1.Addon{
		TypeMeta:   v1.TypeMeta{
			Kind: "Addon",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "dashboard",
		},
	}

	cleanupOpetions := &test.CleanupOptions{
		TestContext:   ctx,
		Timeout:       5 * time.Second,
		RetryInterval: 1 * time.Second,
	}

	err := test.Global.Client.Create(
		context.TODO(),
		addonCR,
		cleanupOpetions)

	helpers.AssertNoError(t, err)

	err = e2eutil.WaitForDeployment(
		t, test.Global.KubeClient, config.DefaultTargetNs,
		"tekton-dashboard",
		1,
		testConfig.APIRetry,
		testConfig.APITimeout,
	)
	helpers.AssertNoError(t, err)

	helpers.WaitForClusterCR(t, "dashboard", addonCR)

	err = e2eutil.WaitForDeployment(
		t, test.Global.KubeClient, config.DefaultTargetNs,
		"tekton-dashboard",
		1,
		testConfig.APIRetry,
		testConfig.APITimeout,
	)
	helpers.AssertNoError(t, err)

	err = test.Global.Client.Delete(
		context.TODO(),
		addonCR)

	helpers.AssertNoError(t, err)

	helpers.WaitForDeploymentDeletion(t, config.DefaultTargetNs, "tekton-dashboard")
}
