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

func TestFleetManagerDefaults(t *testing.T) {
	for _, disabled := range []*bool{nil, ptr.To(true), ptr.To(false)} {
		for _, plugin := range []FleetManager{"", FleetManagerManual, FleetManagerACM} {
			kueue := Kueue{Disabled: disabled, MultiClusterConfig: MultiClusterConfig{FleetManager: plugin}}
			want := plugin
			if want == "" {
				want = FleetManagerManual
			}
			tk := &TektonKueue{Spec: TektonKueueSpec{Kueue: kueue}}
			tk.SetDefaults(t.Context())
			assert.Equal(t, tk.Spec.FleetManager, want)
			tc := &TektonConfig{Spec: TektonConfigSpec{Kueue: kueue}}
			tc.SetDefaults(t.Context())
			assert.Equal(t, tc.Spec.Kueue.FleetManager, want)
		}
	}
}

func TestFleetManagerValidation(t *testing.T) {
	for _, tt := range []struct {
		name, platform                            string
		plugin                                    FleetManager
		role                                      MultiClusterRole
		disabled, multiDisabled, invalid, enabled bool
	}{
		{name: "omitted", role: MultiClusterRoleHub},
		{name: "manual Kubernetes hub", plugin: FleetManagerManual, role: MultiClusterRoleHub},
		{name: "manual spoke", plugin: FleetManagerManual, role: MultiClusterRoleSpoke},
		{name: "ACM hub", platform: "openshift", plugin: FleetManagerACM, role: MultiClusterRoleHub, enabled: true},
		{name: "ACM lowercase hub", platform: "openshift", plugin: FleetManagerACM, role: "hub", enabled: true},
		{name: "ACM Kubernetes", plugin: FleetManagerACM, role: MultiClusterRoleHub, invalid: true, enabled: true},
		{name: "ACM spoke", platform: "openshift", plugin: FleetManagerACM, role: MultiClusterRoleSpoke, invalid: true},
		{name: "parent disabled", platform: "openshift", plugin: FleetManagerACM, role: MultiClusterRoleHub, disabled: true},
		{name: "multi-cluster disabled", platform: "openshift", plugin: FleetManagerACM, multiDisabled: true},
		{name: "unknown", platform: "openshift", plugin: "other", role: MultiClusterRoleHub, invalid: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PLATFORM", tt.platform)
			kueue := Kueue{
				Disabled:           ptr.To(tt.disabled),
				MultiClusterConfig: MultiClusterConfig{MultiClusterDisabled: tt.multiDisabled, MultiClusterRole: tt.role, FleetManager: tt.plugin},
			}
			tk := &TektonKueue{
				ObjectMeta: metav1.ObjectMeta{Name: TektonKueueResourceName},
				Spec:       TektonKueueSpec{CommonSpec: CommonSpec{TargetNamespace: "pipelines"}, Kueue: kueue},
			}
			err := tk.Validate(t.Context())
			assert.Equal(t, err != nil, tt.invalid)
			if tt.invalid {
				assert.ErrorContains(t, err, "spec.fleet-manager")
			}
			tc := &TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: ConfigResourceName},
				Spec:       TektonConfigSpec{CommonSpec: CommonSpec{TargetNamespace: "pipelines"}, Kueue: kueue},
			}
			err = tc.Validate(t.Context())
			assert.Equal(t, strings.Contains(err.Error(), "spec.kueue.fleet-manager"), tt.invalid)
			assert.Equal(t, kueue.IsACMPluginEnabled(), tt.enabled)
		})
	}
}

func TestFleetManagerDefaultDoesNotBlockSchedulerMigration(t *testing.T) {
	spec := TektonConfigSpec{
		Scheduler: Scheduler{Disabled: ptr.To(false)},
		Kueue:     Kueue{MultiClusterConfig: MultiClusterConfig{FleetManager: FleetManagerManual}},
	}
	assert.Assert(t, spec.MigrateScheduler())
	assert.Assert(t, !spec.Kueue.IsDisabled())
	spec.Kueue.SetDefaults()
	assert.Equal(t, spec.Kueue.FleetManager, FleetManagerManual)
}
