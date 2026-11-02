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
	"reflect"
	"strings"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"gotest.tools/v3/assert"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	discoveryfake "k8s.io/client-go/discovery/fake"
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

func (f *componentFixture) setAPIs(groupVersions ...string) {
	resources := []*metav1.APIResourceList{}
	for _, gv := range groupVersions {
		resources = append(resources, &metav1.APIResourceList{GroupVersion: gv})
	}
	f.clients.Discovery().(*discoveryfake.FakeDiscovery).Resources = resources
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

// install creates a ready TektonKueue while both prerequisites exist, then leaves only the available one.
func (f *componentFixture) install(available string) {
	f.t.Helper()
	f.setAPIs(CERT_GVK, KUEUE_GVK)
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
	f.setAPIs(available)
}

func TestEnsureTektonComponentMissingPrerequisite(t *testing.T) {
	for _, tt := range []struct{ name, available, message string }{
		{name: "cert-manager", available: KUEUE_GVK, message: "Please install cert-manager"},
		{name: "kueue", available: CERT_GVK, message: "Please install kueue"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("acm to manual is propagated", func(t *testing.T) {
				f := newComponentFixture(t, v1alpha1.FleetManagerACM)
				f.install(tt.available)
				f.config.Spec.Kueue.FleetManager = v1alpha1.FleetManagerManual
				f.requeue()
				child := f.child()
				assert.Equal(t, child.Spec.FleetManager, v1alpha1.FleetManagerManual)
				assert.Assert(t, !child.Spec.Kueue.IsACMPluginEnabled())
				assert.Assert(t, child.Spec.Config.MultiKueueOverride)
				// The prerequisite still gates readiness once the deselection is propagated.
				f.blocked(tt.message)
			})

			t.Run("fresh install waits", func(t *testing.T) {
				for _, plugin := range []v1alpha1.FleetManager{v1alpha1.FleetManagerManual, v1alpha1.FleetManagerACM} {
					f := newComponentFixture(t, plugin)
					f.setAPIs(tt.available)
					f.blocked(tt.message)
					_, err := f.clients.OperatorV1alpha1().TektonKueues().Get(t.Context(), v1alpha1.TektonKueueResourceName, metav1.GetOptions{})
					assert.Assert(t, apierrs.IsNotFound(err), "TektonKueue must not be created: %v", err)
				}
			})

			t.Run("acm activation waits", func(t *testing.T) {
				f := newComponentFixture(t, v1alpha1.FleetManagerManual)
				f.install(tt.available)
				want := f.child()
				f.config.Spec.Kueue.FleetManager = v1alpha1.FleetManagerACM
				f.blocked(tt.message)
				assert.Assert(t, reflect.DeepEqual(f.child(), want), "TektonKueue must not change")
			})

			t.Run("update failure is reported and retried", func(t *testing.T) {
				f := newComponentFixture(t, v1alpha1.FleetManagerACM)
				f.install(tt.available)
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

			t.Run("ready manual child waits", func(t *testing.T) {
				f := newComponentFixture(t, v1alpha1.FleetManagerManual)
				f.install(tt.available)
				want := f.child()
				f.blocked(tt.message)
				// A pending change must wait for the prerequisite as well.
				f.config.Spec.TargetNamespace = "other"
				f.blocked(tt.message)
				assert.Assert(t, reflect.DeepEqual(f.child(), want), "TektonKueue must not change")
			})
		})
	}
}
