/*
Copyright 2022 The Tekton Authors

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

package openshiftplatform

import (
	k8sInstallerSet "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset"
	openshiftManualApprovalGate "github.com/tektoncd/operator/pkg/reconciler/openshift/manualapprovalgate"
	"github.com/tektoncd/operator/pkg/reconciler/openshift/openshiftpipelinesascode"
	openshiftAddon "github.com/tektoncd/operator/pkg/reconciler/openshift/tektonaddon"
	openshiftChain "github.com/tektoncd/operator/pkg/reconciler/openshift/tektonchain"
	openshiftConfig "github.com/tektoncd/operator/pkg/reconciler/openshift/tektonconfig"
	openshiftHub "github.com/tektoncd/operator/pkg/reconciler/openshift/tektonhub"
	openshiftPipeline "github.com/tektoncd/operator/pkg/reconciler/openshift/tektonpipeline"
	openshiftPruner "github.com/tektoncd/operator/pkg/reconciler/openshift/tektonpruner"
	openshiftResult "github.com/tektoncd/operator/pkg/reconciler/openshift/tektonresult"
	openshiftTrigger "github.com/tektoncd/operator/pkg/reconciler/openshift/tektontrigger"
	"github.com/tektoncd/operator/pkg/reconciler/platform"
	"knative.dev/pkg/injection"
)

const (
	ControllerTektonAddon              platform.ControllerName = "tektonaddon"
	ControllerOpenShiftPipelinesAsCode platform.ControllerName = "openshiftpipelinesascode"
	PlatformNameOpenShift              string                  = "openshift"
)

var (
	// openshiftControllers define a platform.ControllerMap of
	// all controllers(reconcilers) supported by OpenShift platform
	openshiftControllers = platform.ControllerMap{
		platform.ControllerTektonConfig: injection.NamedControllerConstructor{
			Name:                  string(platform.ControllerTektonConfig),
			ControllerConstructor: openshiftConfig.NewController,
		},
		platform.ControllerTektonPipeline: injection.NamedControllerConstructor{
			Name:                  string(platform.ControllerTektonPipeline),
			ControllerConstructor: openshiftPipeline.NewController,
		},
		platform.ControllerTektonTrigger: injection.NamedControllerConstructor{
			Name:                  string(platform.ControllerTektonTrigger),
			ControllerConstructor: openshiftTrigger.NewController,
		},
		platform.ControllerTektonHub: injection.NamedControllerConstructor{
			Name:                  string(platform.ControllerTektonHub),
			ControllerConstructor: openshiftHub.NewController,
		},
		platform.ControllerTektonChain: injection.NamedControllerConstructor{
			Name:                  string(platform.ControllerTektonChain),
			ControllerConstructor: openshiftChain.NewController,
		},
		platform.ControllerTektonResult: injection.NamedControllerConstructor{
			Name:                  string(platform.ControllerTektonResult),
			ControllerConstructor: openshiftResult.NewController,
		},
		platform.ControllerManualApprovalGate: injection.NamedControllerConstructor{
			Name:                  string(platform.ControllerManualApprovalGate),
			ControllerConstructor: openshiftManualApprovalGate.NewController,
		},
		ControllerTektonAddon: injection.NamedControllerConstructor{
			Name:                  string(ControllerTektonAddon),
			ControllerConstructor: openshiftAddon.NewController,
		},
		ControllerOpenShiftPipelinesAsCode: injection.NamedControllerConstructor{
			Name:                  string(ControllerOpenShiftPipelinesAsCode),
			ControllerConstructor: openshiftpipelinesascode.NewController,
		},
		// there is no openshift specific extension for TektonInstallerSet Reconciler (yet 🤓)
		platform.ControllerTektonInstallerSet: injection.NamedControllerConstructor{
			Name:                  string(platform.ControllerTektonInstallerSet),
			ControllerConstructor: k8sInstallerSet.NewController,
		},
		platform.ControllerTektonPruner: injection.NamedControllerConstructor{
			Name:                  string(platform.ControllerTektonPruner),
			ControllerConstructor: openshiftPruner.NewController,
		},
	}
)
