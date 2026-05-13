/*
Copyright 2021 The Tekton Authors

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
	"context"
	"strings"

	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
)

func (tc *TektonConfig) SetDefaults(ctx context.Context) {
	if tc.Spec.Profile == "" {
		tc.Spec.Profile = ProfileBasic
	}
	tc.Spec.Pipeline.setDefaults()
	tc.Spec.Trigger.setDefaults()
	tc.Spec.Chain.setDefaults()
	tc.Spec.Result.setDefaults()
	tc.Spec.TektonPruner.SetDefaults()
	tc.Spec.Scheduler.SetDefaults()

	if IsOpenShiftPlatform() {
		// PAC may appear under spec.platforms.kubernetes if the mutating webhook ran without
		// PLATFORM=openshift (e.g. wrong image/order) or from older releases. Move it to
		// spec.platforms.openshift so the stored TektonConfig matches the OpenShift operator.
		if tc.Spec.Platforms.Kubernetes.PipelinesAsCode != nil {
			if tc.Spec.Platforms.OpenShift.PipelinesAsCode == nil {
				p := *tc.Spec.Platforms.Kubernetes.PipelinesAsCode
				tc.Spec.Platforms.OpenShift.PipelinesAsCode = &p
			}
			tc.Spec.Platforms.Kubernetes.PipelinesAsCode = nil
		}

		if tc.Spec.Platforms.OpenShift.PipelinesAsCode != nil {
			tc.Spec.Addon.EnablePAC = nil
		} else {
			tc.Spec.Platforms.OpenShift.PipelinesAsCode = &PipelinesAsCode{
				Enable: ptr.Bool(true),
				PACSettings: PACSettings{
					Settings: map[string]string{},
				},
			}
		}

		// check if PAC is disabled through addon before enabling through OpenShift PipelinesAsCode
		if tc.Spec.Addon.EnablePAC != nil && !*tc.Spec.Addon.EnablePAC {
			if tc.Spec.Platforms.OpenShift.PipelinesAsCode != nil {
				tc.Spec.Platforms.OpenShift.PipelinesAsCode.Enable = ptr.Bool(false)
				tc.Spec.Platforms.OpenShift.PipelinesAsCode.PACSettings.Settings = nil
			}
		}

		if p := tc.Spec.Platforms.OpenShift.PipelinesAsCode; p != nil && p.Enable != nil && *p.Enable {
			logger := logging.FromContext(ctx)
			p.PACSettings.setPACDefaults(logger)
		}

		// Central TLS is enabled by default on OpenShift; users may set
		// enableCentralTLSConfig: false in the CR to opt out.
		if tc.Spec.Platforms.OpenShift.EnableCentralTLSConfig == nil {
			tc.Spec.Platforms.OpenShift.EnableCentralTLSConfig = ptr.Bool(true)
		}

		// SCC defaulting
		if tc.Spec.Platforms.OpenShift.SCC == nil {
			tc.Spec.Platforms.OpenShift.SCC = &SCC{}
		}
		if tc.Spec.Platforms.OpenShift.SCC.Default == "" {
			tc.Spec.Platforms.OpenShift.SCC.Default = PipelinesSCC
		}

		setAddonDefaults(&tc.Spec.Addon)
	} else {
		// Kubernetes Platform
		if tc.Spec.Platforms.Kubernetes.PipelinesAsCode == nil {
			tc.Spec.Platforms.Kubernetes.PipelinesAsCode = &PipelinesAsCode{
				Enable: ptr.Bool(true),
				PACSettings: PACSettings{
					Settings: map[string]string{},
				},
			}
		} else {
			tc.Spec.Addon.EnablePAC = nil
		}

		if tc.Spec.Addon.EnablePAC != nil && !*tc.Spec.Addon.EnablePAC {
			tc.Spec.Platforms.Kubernetes.PipelinesAsCode.Enable = ptr.Bool(false)
			tc.Spec.Platforms.Kubernetes.PipelinesAsCode.PACSettings.Settings = nil
		}

		if *tc.Spec.Platforms.Kubernetes.PipelinesAsCode.Enable {
			logger := logging.FromContext(ctx)
			tc.Spec.Platforms.Kubernetes.PipelinesAsCode.PACSettings.setPACDefaults(logger)
		}
		setAddonDefaults(&tc.Spec.Addon)
	}

	// earlier pruner was disabled with empty schedule or empty resources
	// now empty schedule, disables only the global cron job,
	// if a namespace has prune schedule annotation, a cron job will be created for that
	// to disable the pruner feature, "disabled" should be set as "true"
	if !tc.Spec.Pruner.Disabled {
		// if keep and keep-since is nil, update default keep value
		if tc.Spec.Pruner.Keep == nil && tc.Spec.Pruner.KeepSince == nil {
			keep := PrunerDefaultKeep
			tc.Spec.Pruner.Keep = &keep
		}

		// if empty resources, update default resources
		if len(tc.Spec.Pruner.Resources) == 0 {
			tc.Spec.Pruner.Resources = PruningDefaultResources
		}

		// trim space and to lower case resource names
		for index := range tc.Spec.Pruner.Resources {
			value := tc.Spec.Pruner.Resources[index]
			value = strings.TrimSpace(value)
			value = strings.ToLower(value)
			tc.Spec.Pruner.Resources[index] = value
		}
	}
}
