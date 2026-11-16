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

package kueue

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"
)

func TestFleetManagerPropagation(t *testing.T) {
	config := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "config"},
		Spec: v1alpha1.TektonConfigSpec{
			CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "pipelines"},
			Kueue: v1alpha1.Kueue{
				Disabled:           ptr.To(false),
				MultiClusterConfig: v1alpha1.MultiClusterConfig{MultiClusterRole: v1alpha1.MultiClusterRoleHub, FleetManager: v1alpha1.FleetManagerManual},
			},
		},
	}
	old := GetTektonKueueCR(config, "v1")
	clients := fake.NewSimpleClientset(old).OperatorV1alpha1().TektonKueues()
	for _, plugin := range []v1alpha1.FleetManager{v1alpha1.FleetManagerACM, v1alpha1.FleetManagerManual} {
		config.Spec.Kueue.FleetManager = plugin
		desired := GetTektonKueueCR(config, "v1")
		assert.Equal(t, desired.Spec.FleetManager, plugin)
		_, err := UpdateKueue(t.Context(), old, desired, clients)
		assert.Equal(t, err, v1alpha1.RECONCILE_AGAIN_ERR)
		old, err = clients.Get(t.Context(), v1alpha1.TektonKueueResourceName, metav1.GetOptions{})
		assert.NilError(t, err)
		assert.Equal(t, old.Spec.FleetManager, plugin)
	}
}

type componentFixture struct {
	t       *testing.T
	config  *v1alpha1.TektonConfig
	clients *fake.Clientset
}

func newComponentFixture(t *testing.T, plugin v1alpha1.FleetManager) *componentFixture {
	return &componentFixture{t: t, clients: fake.NewSimpleClientset(), config: &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "config"},
		Spec: v1alpha1.TektonConfigSpec{
			CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "pipelines"},
			Kueue: v1alpha1.Kueue{
				Disabled:           ptr.To(false),
				MultiClusterConfig: v1alpha1.MultiClusterConfig{MultiClusterRole: v1alpha1.MultiClusterRoleHub, FleetManager: plugin},
			},
		},
	}}
}

// requeue runs EnsureTektonComponent, which must ask for a retry, and returns the reported reason.
func (f *componentFixture) requeue() string {
	f.t.Helper()
	assert.Equal(f.t, EnsureTektonComponent(f.t.Context(), f.config, f.clients, "v1"), v1alpha1.REQUEUE_EVENT_AFTER)
	return f.config.Status.GetCondition(v1alpha1.ComponentsReady).Message
}

// blocked runs EnsureTektonComponent, which must ask for a retry because of reason.
func (f *componentFixture) blocked(reason string) {
	f.t.Helper()
	got := f.requeue()
	assert.Assert(f.t, strings.Contains(got, reason), "got reason %q, want %q", got, reason)
}

func (f *componentFixture) child() *v1alpha1.TektonKueue {
	f.t.Helper()
	child, err := f.clients.OperatorV1alpha1().TektonKueues().Get(f.t.Context(), v1alpha1.TektonKueueResourceName, metav1.GetOptions{})
	assert.NilError(f.t, err)
	return child
}

// install creates a ready TektonKueue.
func (f *componentFixture) install() {
	f.t.Helper()
	f.requeue()
	child := f.child()
	child.Status.MarkDependenciesInstalled()
	child.Status.MarkPreReconcilerComplete()
	child.Status.MarkInstallerSetAvailable()
	child.Status.MarkInstallerSetReady()
	child.Status.MarkPostReconcilerComplete()
	_, err := f.clients.OperatorV1alpha1().TektonKueues().UpdateStatus(f.t.Context(), child, metav1.UpdateOptions{})
	assert.NilError(f.t, err)
	assert.NilError(f.t, EnsureTektonComponent(f.t.Context(), f.config, f.clients, "v1"))
}

// TektonKueue waits for cert-manager and Kueue itself, so it is created and
// updated without a check for them here. The fake cluster serves neither API.
func TestEnsureTektonComponentDoesNotCheckPrerequisites(t *testing.T) {
	t.Run("fresh install", func(t *testing.T) {
		for _, plugin := range []v1alpha1.FleetManager{v1alpha1.FleetManagerManual, v1alpha1.FleetManagerACM} {
			f := newComponentFixture(t, plugin)
			f.requeue()
			assert.Equal(t, f.child().Spec.FleetManager, plugin)
		}
	})

	t.Run("acm to manual is propagated", func(t *testing.T) {
		f := newComponentFixture(t, v1alpha1.FleetManagerACM)
		f.install()
		f.config.Spec.Kueue.FleetManager = v1alpha1.FleetManagerManual
		f.requeue()
		child := f.child()
		assert.Equal(t, child.Spec.FleetManager, v1alpha1.FleetManagerManual)
		assert.Assert(t, !child.Spec.Kueue.IsACMPluginEnabled())
		assert.Assert(t, child.Spec.Config.MultiKueueOverride)
	})

	t.Run("manual to acm is propagated", func(t *testing.T) {
		f := newComponentFixture(t, v1alpha1.FleetManagerManual)
		f.install()
		f.config.Spec.Kueue.FleetManager = v1alpha1.FleetManagerACM
		f.requeue()
		assert.Assert(t, f.child().Spec.Kueue.IsACMPluginEnabled())
	})

	t.Run("update failure is reported and retried", func(t *testing.T) {
		f := newComponentFixture(t, v1alpha1.FleetManagerACM)
		f.install()
		fail := true
		f.clients.PrependReactor("update", "tektonkueues", func(clienttesting.Action) (bool, runtime.Object, error) {
			if fail {
				return true, nil, fmt.Errorf("update failed")
			}
			return false, nil, nil
		})
		f.config.Spec.Kueue.FleetManager = v1alpha1.FleetManagerManual
		f.blocked("TektonKueue : update failed")
		assert.Assert(t, f.child().Spec.Kueue.IsACMPluginEnabled())
		fail = false
		f.requeue()
		assert.Assert(t, !f.child().Spec.Kueue.IsACMPluginEnabled())
	})
}

// TektonConfig reports why TektonKueue is not ready, not only that it waits.
func TestEnsureTektonComponentReportsReason(t *testing.T) {
	f := newComponentFixture(t, v1alpha1.FleetManagerACM)
	// A new TektonKueue has no reason to report yet.
	f.blocked("TektonKueue : " + v1alpha1.RECONCILE_AGAIN_ERR.Error())

	child := f.child()
	child.Status.MarkPreReconcilerFailed("ACM plugin requires API work.open-cluster-management.io/v1: requeue after: 10s")
	_, err := f.clients.OperatorV1alpha1().TektonKueues().UpdateStatus(t.Context(), child, metav1.UpdateOptions{})
	assert.NilError(t, err)
	f.blocked("TektonKueue : PreReconciliation failed with message: ACM plugin requires API work.open-cluster-management.io/v1: requeue after: 10s")

	// A missing prerequisite is reported the same way.
	child = f.child()
	child.Status.MarkPreReconcilerComplete()
	child.Status.MarkDependencyMissing("API cert-manager.io/v1 does not exist. Please install cert-manager.io/v1")
	_, err = f.clients.OperatorV1alpha1().TektonKueues().UpdateStatus(t.Context(), child, metav1.UpdateOptions{})
	assert.NilError(t, err)
	f.blocked("TektonKueue : Dependency missing: API cert-manager.io/v1 does not exist. Please install cert-manager.io/v1")
}
