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

package tektonconfig

import (
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorfake "github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/pipeline"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/upgrade"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	discoveryfake "k8s.io/client-go/discovery/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
	"knative.dev/pkg/apis"
)

func TestPendingProxyDoesNotBlockKueue(t *testing.T) {
	ctx := t.Context()
	tc := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.ConfigResourceName, Labels: map[string]string{v1alpha1.ReleaseVersionKey: "test"}},
		Spec: v1alpha1.TektonConfigSpec{
			CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "pipelines"},
			Profile:    v1alpha1.ProfileLite,
			Kueue: v1alpha1.Kueue{
				Disabled:           ptr.To(false),
				MultiClusterConfig: v1alpha1.MultiClusterConfig{MultiClusterRole: v1alpha1.MultiClusterRoleHub, FleetManager: v1alpha1.FleetManagerACM},
			},
		},
	}
	tc.Spec.TektonPruner.Disabled = ptr.To(true)
	tc.Status.SetPreUpgradeVersion("test")
	tc.Status.MarkPreUpgradeComplete()
	tp := pipeline.GetTektonPipelineCR(tc, "test")
	tp.Status.MarkPreReconcilerComplete()
	tp.Status.MarkInstallerSetAvailable()
	tp.Status.MarkInstallerSetReady()
	tp.Status.MarkPostReconcilerComplete()
	op := operatorfake.NewSimpleClientset(tc, tp)
	op.Discovery().(*discoveryfake.FakeDiscovery).Resources = []*metav1.APIResourceList{
		{GroupVersion: "cert-manager.io/v1"},
		{GroupVersion: "kueue.x-k8s.io/v1beta1"},
	}
	kube := kubefake.NewSimpleClientset()
	r := &Reconciler{
		operatorClientSet: op, kubeClientSet: kube, operatorVersion: "test",
		extension: common.NoExtension(ctx), upgrade: upgrade.New("test", kube, op, nil),
	}
	assert.Equal(t, r.ReconcileKind(ctx, tc), v1alpha1.REQUEUE_EVENT_AFTER)
	proxy, err := op.OperatorV1alpha1().TektonMulticlusterProxyAAEs().Get(ctx, v1alpha1.MultiClusterProxyAAEResourceName, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Assert(t, !proxy.Status.IsReady())
	kueue, err := op.OperatorV1alpha1().TektonKueues().Get(ctx, v1alpha1.TektonKueueResourceName, metav1.GetOptions{})
	assert.NilError(t, err, "worker registration must start before the proxy has workers")
	assert.Equal(t, kueue.Spec.FleetManager, v1alpha1.FleetManagerACM)

	kueue.Status.MarkDependenciesInstalled()
	kueue.Status.MarkPreReconcilerComplete()
	kueue.Status.MarkInstallerSetAvailable()
	kueue.Status.MarkInstallerSetReady()
	kueue.Status.MarkPostReconcilerComplete()
	_, err = op.OperatorV1alpha1().TektonKueues().UpdateStatus(ctx, kueue, metav1.UpdateOptions{})
	assert.NilError(t, err)

	// Deselecting the plugin must also propagate while proxy readiness is pending.
	tc.Spec.Kueue.FleetManager = v1alpha1.FleetManagerManual
	assert.Equal(t, r.ReconcileKind(ctx, tc), v1alpha1.REQUEUE_EVENT_AFTER)
	kueue, err = op.OperatorV1alpha1().TektonKueues().Get(ctx, v1alpha1.TektonKueueResourceName, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, kueue.Spec.FleetManager, v1alpha1.FleetManagerManual)
	assert.Equal(t, r.ReconcileKind(ctx, tc), v1alpha1.REQUEUE_EVENT_AFTER)
	assert.Assert(t, !tc.Status.IsReady())
	assert.Equal(t, tc.Status.GetCondition(apis.ConditionReady).Message, "Components not in ready state: TektonMulticlusterProxyAAE is not ready")
}
