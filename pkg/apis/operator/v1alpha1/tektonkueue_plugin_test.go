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

package v1alpha1

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestMultiClusterPluginDefaults(t *testing.T) {
	for _, disabled := range []*bool{nil, ptr.To(true), ptr.To(false)} {
		for _, plugin := range []MultiClusterPlugin{"", MultiClusterPluginManual, MultiClusterPluginACM} {
			kueue := Kueue{Disabled: disabled, MultiClusterPlugin: plugin}
			want := plugin
			if want == "" {
				want = MultiClusterPluginManual
			}
			tk := &TektonKueue{Spec: TektonKueueSpec{Kueue: kueue}}
			tk.SetDefaults(t.Context())
			assert.Equal(t, tk.Spec.MultiClusterPlugin, want)
			tc := &TektonConfig{Spec: TektonConfigSpec{Kueue: kueue}}
			tc.SetDefaults(t.Context())
			assert.Equal(t, tc.Spec.Kueue.MultiClusterPlugin, want)
		}
	}
}

func TestMultiClusterPluginValidation(t *testing.T) {
	for _, tt := range []struct {
		name, platform                            string
		plugin                                    MultiClusterPlugin
		role                                      MultiClusterRole
		disabled, multiDisabled, invalid, enabled bool
	}{
		{name: "omitted", role: MultiClusterRoleHub},
		{name: "manual Kubernetes hub", plugin: MultiClusterPluginManual, role: MultiClusterRoleHub},
		{name: "manual spoke", plugin: MultiClusterPluginManual, role: MultiClusterRoleSpoke},
		{name: "ACM hub", platform: "openshift", plugin: MultiClusterPluginACM, role: MultiClusterRoleHub, enabled: true},
		{name: "ACM lowercase hub", platform: "openshift", plugin: MultiClusterPluginACM, role: "hub", enabled: true},
		{name: "ACM Kubernetes", plugin: MultiClusterPluginACM, role: MultiClusterRoleHub, invalid: true, enabled: true},
		{name: "ACM spoke", platform: "openshift", plugin: MultiClusterPluginACM, role: MultiClusterRoleSpoke, invalid: true},
		{name: "parent disabled", platform: "openshift", plugin: MultiClusterPluginACM, role: MultiClusterRoleHub, disabled: true},
		{name: "multi-cluster disabled", platform: "openshift", plugin: MultiClusterPluginACM, multiDisabled: true},
		{name: "unknown", platform: "openshift", plugin: "other", role: MultiClusterRoleHub, invalid: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PLATFORM", tt.platform)
			kueue := Kueue{
				Disabled: ptr.To(tt.disabled), MultiClusterPlugin: tt.plugin,
				MultiClusterConfig: MultiClusterConfig{MultiClusterDisabled: tt.multiDisabled, MultiClusterRole: tt.role},
			}
			tk := &TektonKueue{
				ObjectMeta: metav1.ObjectMeta{Name: TektonKueueResourceName},
				Spec:       TektonKueueSpec{CommonSpec: CommonSpec{TargetNamespace: "pipelines"}, Kueue: kueue},
			}
			err := tk.Validate(t.Context())
			assert.Equal(t, err != nil, tt.invalid)
			if tt.invalid {
				assert.ErrorContains(t, err, "spec.multi-cluster-plugin")
			}
			tc := &TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: ConfigResourceName},
				Spec:       TektonConfigSpec{CommonSpec: CommonSpec{TargetNamespace: "pipelines"}, Kueue: kueue},
			}
			err = tc.Validate(t.Context())
			assert.Equal(t, strings.Contains(err.Error(), "spec.kueue.multi-cluster-plugin"), tt.invalid)
			assert.Equal(t, kueue.IsACMPluginEnabled(), tt.enabled)
		})
	}
}

func TestPluginDefaultDoesNotBlockSchedulerMigration(t *testing.T) {
	spec := TektonConfigSpec{
		Scheduler: Scheduler{Disabled: ptr.To(false)},
		Kueue:     Kueue{MultiClusterPlugin: MultiClusterPluginManual},
	}
	assert.Assert(t, spec.MigrateScheduler())
	assert.Assert(t, !spec.Kueue.IsDisabled())
	spec.Kueue.SetDefaults()
	assert.Equal(t, spec.Kueue.MultiClusterPlugin, MultiClusterPluginManual)
}
