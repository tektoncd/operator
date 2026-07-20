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

package manualapprovalgate

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

func TestEnsureManualApprovalGateExists(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	c := fake.Get(ctx)
	mag := GetManualApprovalGateCR(getTektonConfig(), "v0.80.0")

	// first invocation should create instance as it is non-existent and return RECONCILE_AGAIN_ERR
	_, err := EnsureManualApprovalGateExists(ctx, c.OperatorV1alpha1().ManualApprovalGates(), mag)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// during second invocation instance exists but waiting on dependencies
	// hence returns RECONCILE_AGAIN_ERR
	_, err = EnsureManualApprovalGateExists(ctx, c.OperatorV1alpha1().ManualApprovalGates(), mag)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// make upgrade checks pass
	makeUpgradeCheckPass(t, ctx, c.OperatorV1alpha1().ManualApprovalGates())

	// next invocation should return RECONCILE_AGAIN_ERR as MAG is waiting for installation
	_, err = EnsureManualApprovalGateExists(ctx, c.OperatorV1alpha1().ManualApprovalGates(), mag)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// mark the instance ready
	markMAGReady(t, ctx, c.OperatorV1alpha1().ManualApprovalGates())

	// next invocation should return nil error as the instance is ready
	_, err = EnsureManualApprovalGateExists(ctx, c.OperatorV1alpha1().ManualApprovalGates(), mag)
	util.AssertEqual(t, err, nil)

	// test update propagation from tektonConfig
	mag.Spec.TargetNamespace = "foobar"
	_, err = EnsureManualApprovalGateExists(ctx, c.OperatorV1alpha1().ManualApprovalGates(), mag)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	_, err = EnsureManualApprovalGateExists(ctx, c.OperatorV1alpha1().ManualApprovalGates(), mag)
	util.AssertEqual(t, err, nil)
}

func TestEnsureManualApprovalGateCRNotExists(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	c := fake.Get(ctx)

	// when no instance exists, nil error is returned immediately
	err := EnsureManualApprovalGateCRNotExists(ctx, c.OperatorV1alpha1().ManualApprovalGates())
	util.AssertEqual(t, err, nil)

	// create an instance for testing other cases
	mag := GetManualApprovalGateCR(getTektonConfig(), "v0.80.0")
	_, err = EnsureManualApprovalGateExists(ctx, c.OperatorV1alpha1().ManualApprovalGates(), mag)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// when an instance exists the first invocation should make the delete API call and
	// return RECONCILE_AGAIN_ERR. So that the deletion can be confirmed in a subsequent invocation
	err = EnsureManualApprovalGateCRNotExists(ctx, c.OperatorV1alpha1().ManualApprovalGates())
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// when the instance is completely removed from a cluster, the function should return nil error
	err = EnsureManualApprovalGateCRNotExists(ctx, c.OperatorV1alpha1().ManualApprovalGates())
	util.AssertEqual(t, err, nil)
}

func TestEnsureManualApprovalGateExists_MigratesOwnerRef(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	c := fake.Get(ctx)
	clients := c.OperatorV1alpha1().ManualApprovalGates()

	// simulate a standalone MAG CR created directly by the user (no ownerRef)
	standalone := &v1alpha1.ManualApprovalGate{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.ManualApprovalGates,
		},
		Spec: v1alpha1.ManualApprovalGateSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: "tekton-pipelines",
			},
		},
	}
	_, err := clients.Create(ctx, standalone, metav1.CreateOptions{})
	util.AssertEqual(t, err, nil)

	// verify the standalone CR has no ownerReferences
	existing, err := clients.Get(ctx, v1alpha1.ManualApprovalGates, metav1.GetOptions{})
	util.AssertEqual(t, err, nil)
	util.AssertEqual(t, len(existing.OwnerReferences), 0)

	// build the desired CR (with ownerRef from TektonConfig)
	desired := GetManualApprovalGateCR(getTektonConfig(), "v0.80.0")

	// EnsureExists should adopt the standalone CR by adding the ownerRef
	_, err = EnsureManualApprovalGateExists(ctx, clients, desired)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// verify ownerRef was added
	migrated, err := clients.Get(ctx, v1alpha1.ManualApprovalGates, metav1.GetOptions{})
	util.AssertEqual(t, err, nil)
	util.AssertEqual(t, len(migrated.OwnerReferences), 1)
	util.AssertEqual(t, migrated.OwnerReferences[0].Name, v1alpha1.ConfigResourceName)
}

func TestEnsureManualApprovalGateExists_PropagatesPlatformDataHash(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	c := fake.Get(ctx)
	clients := c.OperatorV1alpha1().ManualApprovalGates()

	// create an initial MAG CR without platform-data-hash
	mag := GetManualApprovalGateCR(getTektonConfig(), "v0.80.0")
	_, err := EnsureManualApprovalGateExists(ctx, clients, mag)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	makeUpgradeCheckPass(t, ctx, clients)

	// reconcile again after upgrade check updated labels
	_, err = EnsureManualApprovalGateExists(ctx, clients, mag)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	markMAGReady(t, ctx, clients)

	_, err = EnsureManualApprovalGateExists(ctx, clients, mag)
	util.AssertEqual(t, err, nil)

	// verify no platform-data-hash annotation exists yet
	existing, err := clients.Get(ctx, v1alpha1.ManualApprovalGates, metav1.GetOptions{})
	util.AssertEqual(t, err, nil)
	util.AssertEqual(t, existing.Annotations[v1alpha1.PlatformDataHashKey], "")

	// simulate TektonConfig setting platform-data-hash (TLS profile change)
	mag.Annotations = map[string]string{
		v1alpha1.PlatformDataHashKey: "abc123",
	}
	_, err = EnsureManualApprovalGateExists(ctx, clients, mag)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// verify annotation was propagated
	updated, err := clients.Get(ctx, v1alpha1.ManualApprovalGates, metav1.GetOptions{})
	util.AssertEqual(t, err, nil)
	util.AssertEqual(t, updated.Annotations[v1alpha1.PlatformDataHashKey], "abc123")

	markMAGReady(t, ctx, clients)

	// simulate a TLS profile change (hash changes)
	mag.Annotations[v1alpha1.PlatformDataHashKey] = "def456"
	_, err = EnsureManualApprovalGateExists(ctx, clients, mag)
	util.AssertEqual(t, err, v1alpha1.RECONCILE_AGAIN_ERR)

	// verify the new hash was propagated
	updated, err = clients.Get(ctx, v1alpha1.ManualApprovalGates, metav1.GetOptions{})
	util.AssertEqual(t, err, nil)
	util.AssertEqual(t, updated.Annotations[v1alpha1.PlatformDataHashKey], "def456")
}

func TestGetManualApprovalGateCR(t *testing.T) {
	config := getTektonConfig()
	mag := GetManualApprovalGateCR(config, "v0.80.0")

	util.AssertEqual(t, mag.Name, v1alpha1.ManualApprovalGates)
	util.AssertEqual(t, mag.Spec.TargetNamespace, "tekton-pipelines")
	util.AssertEqual(t, len(mag.OwnerReferences), 1)
	util.AssertEqual(t, mag.OwnerReferences[0].Name, v1alpha1.ConfigResourceName)
	util.AssertEqual(t, mag.Labels[v1alpha1.ReleaseVersionKey], "v0.80.0")
}

func markMAGReady(t *testing.T, ctx context.Context, c op.ManualApprovalGateInterface) {
	t.Helper()
	mag, err := c.Get(ctx, v1alpha1.ManualApprovalGates, metav1.GetOptions{})
	util.AssertEqual(t, err, nil)
	mag.Status.MarkDependenciesInstalled()
	mag.Status.MarkPreReconcilerComplete()
	mag.Status.MarkInstallerSetAvailable()
	mag.Status.MarkInstallerSetReady()
	mag.Status.MarkPostReconcilerComplete()
	_, err = c.UpdateStatus(ctx, mag, metav1.UpdateOptions{})
	util.AssertEqual(t, err, nil)
}

func makeUpgradeCheckPass(t *testing.T, ctx context.Context, c op.ManualApprovalGateInterface) {
	t.Helper()
	mag, err := c.Get(ctx, v1alpha1.ManualApprovalGates, metav1.GetOptions{})
	util.AssertEqual(t, err, nil)
	setDummyVersionLabel(t, mag)
	_, err = c.Update(ctx, mag, metav1.UpdateOptions{})
	util.AssertEqual(t, err, nil)
}

func setDummyVersionLabel(t *testing.T, mag *v1alpha1.ManualApprovalGate) {
	t.Helper()

	oprVersion := "v1.2.3"
	t.Setenv(v1alpha1.VersionEnvKey, oprVersion)

	labels := mag.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[v1alpha1.ReleaseVersionKey] = oprVersion
	mag.SetLabels(labels)
}

func getTektonConfig() *v1alpha1.TektonConfig {
	return &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.ConfigResourceName,
		},
		Spec: v1alpha1.TektonConfigSpec{
			Profile: v1alpha1.ProfileAll,
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: "tekton-pipelines",
			},
		},
	}
}
