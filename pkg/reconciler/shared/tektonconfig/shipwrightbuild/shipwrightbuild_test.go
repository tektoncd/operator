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

package shipwrightbuild

import (
	"context"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	op "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/injection/client/fake"
	util "github.com/tektoncd/operator/pkg/reconciler/common/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ts "knative.dev/pkg/reconciler/testing"
)

const (
	operatorVersion = "v1.2.3"
	// platformData is empty for the baseline tests; the annotation path is
	// exercised separately in TestShipwrightBuildCreateOrUpdatePlatformData.
	platformData = ""
)

func getTektonConfig() *v1alpha1.TektonConfig {
	tc := &v1alpha1.TektonConfig{
		Name: v1alpha1.ConfigResourceName,
		Spec: v1alpha1.TektonConfigSpec{
			Profile: v1alpha1.ProfileAll,
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: "tekton-pipelines",
			},
		},
	}
	// The Kubernetes platform path is used when PLATFORM is unset (the default in tests).
	tc.Spec.Platforms.Kubernetes = &v1alpha1.Kubernetes{
		ShipwrightBuild: &v1alpha1.ShipwrightBuildConfig{
			Enabled: true,
		},
	}
	return tc
}

func TestShipwrightBuildCreateOrUpdate(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	c := fake.Get(ctx)
	client := c.OperatorV1alpha1().ShipwrightBuilds()
	config := getTektonConfig()

	// first invocation creates the CR and asks for another reconcile
	_, err := CreateOrUpdate(ctx, client, config, operatorVersion, platformData)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// created with the expected name
	sb, err := Get(ctx, client)
	util.AssertEqual(t, err, nil)
	util.AssertEqual(t, sb.GetName(), v1alpha1.ShipwrightBuildResourceName)
	util.AssertEqual(t, sb.Labels[v1alpha1.ReleaseVersionKey], operatorVersion)

	// second invocation: CR exists but is not ready yet
	_, err = CreateOrUpdate(ctx, client, config, operatorVersion, platformData)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// mark the CR ready
	markReady(t, ctx, client)

	// now the CR is ready, so a nil error is returned
	_, err = CreateOrUpdate(ctx, client, config, operatorVersion, platformData)
	util.AssertEqual(t, err, nil)

	// changing a spec field carried by TektonConfig triggers an update
	config.Spec.Platforms.Kubernetes.ShipwrightBuild.Enabled = false
	_, err = CreateOrUpdate(ctx, client, config, operatorVersion, platformData)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// after the update was applied, no further change is needed
	_, err = CreateOrUpdate(ctx, client, config, operatorVersion, platformData)
	util.AssertEqual(t, err, nil)
}

func TestShipwrightBuildCreateOrUpdatePlatformData(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	c := fake.Get(ctx)
	client := c.OperatorV1alpha1().ShipwrightBuilds()
	config := getTektonConfig()

	const initialHash = "platform-hash-1"

	// first invocation creates the CR carrying the platform data annotation
	_, err := CreateOrUpdate(ctx, client, config, operatorVersion, initialHash)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	sb, err := Get(ctx, client)
	util.AssertEqual(t, err, nil)
	util.AssertEqual(t, sb.Annotations[v1alpha1.PlatformDataHashKey], initialHash)

	// mark ready so an unchanged reconcile would otherwise return nil
	markReady(t, ctx, client)
	_, err = CreateOrUpdate(ctx, client, config, operatorVersion, initialHash)
	util.AssertEqual(t, err, nil)

	// a new platform hash triggers an update
	const updatedHash = "platform-hash-2"
	_, err = CreateOrUpdate(ctx, client, config, operatorVersion, updatedHash)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	sb, err = Get(ctx, client)
	util.AssertEqual(t, err, nil)
	util.AssertEqual(t, sb.Annotations[v1alpha1.PlatformDataHashKey], updatedHash)

	// re-applying the same hash needs no further change
	_, err = CreateOrUpdate(ctx, client, config, operatorVersion, updatedHash)
	util.AssertEqual(t, err, nil)
}

func TestShipwrightBuildDeleteIfExists(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	c := fake.Get(ctx)
	client := c.OperatorV1alpha1().ShipwrightBuilds()

	// no CR present: nothing to do
	err := DeleteIfExists(ctx, client)
	util.AssertEqual(t, err, nil)

	// create a CR to delete
	_, err = CreateOrUpdate(ctx, client, getTektonConfig(), operatorVersion, platformData)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// first delete issues the API call and asks for another reconcile
	err = DeleteIfExists(ctx, client)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// once removed, a nil error is returned
	err = DeleteIfExists(ctx, client)
	util.AssertEqual(t, err, nil)
}

func TestShipwrightBuildIsEnabled(t *testing.T) {
	config := getTektonConfig()

	config.Spec.Platforms.Kubernetes.ShipwrightBuild.Enabled = true
	util.AssertEqual(t, IsEnabled(config), true)

	config.Spec.Platforms.Kubernetes.ShipwrightBuild.Enabled = false
	util.AssertEqual(t, IsEnabled(config), false)
}

func markReady(t *testing.T, ctx context.Context, client op.ShipwrightBuildInterface) {
	t.Helper()
	sb, err := client.Get(ctx, v1alpha1.ShipwrightBuildResourceName, metav1.GetOptions{})
	util.AssertEqual(t, err, nil)
	sb.Status.MarkDependenciesInstalled()
	sb.Status.MarkPreReconcilerComplete()
	sb.Status.MarkInstallerSetAvailable()
	sb.Status.MarkInstallerSetReady()
	sb.Status.MarkPostReconcilerComplete()
	_, err = client.UpdateStatus(ctx, sb, metav1.UpdateOptions{})
	util.AssertEqual(t, err, nil)
}
