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

package kubernetesplatform

import (
	k8sManualApprovalGate "github.com/tektoncd/operator/pkg/reconciler/kubernetes/manualapprovalgate"
	k8sChain "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektonchain"
	k8sConfig "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektonconfig"
	k8sDashboard "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektondashboard"
	k8sHub "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektonhub"
	k8sInstallerSet "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset"
	k8sPipeline "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektonpipeline"
	k8sTektonPruner "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektonpruner"
	k8sResult "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektonresult"
	k8stektonscheduler "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektonscheduler"
	k8sTrigger "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektontrigger"
	"github.com/tektoncd/operator/pkg/reconciler/platform"
	"knative.dev/pkg/injection"
)

const (
	ControllerTektonDashboard platform.ControllerName = "tektondashboard"
	ControllerTektonResults   platform.ControllerName = "tektonresult"
	PlatformNameKubernetes    string                  = "kubernetes"
)

var (
	// kubernetesControllers define a platform.ControllerMap of
	// all controllers(reconcilers) supported by Kubernetes platform
	kubernetesControllers = platform.ControllerMap{
		platform.ControllerTektonConfig: injection.NamedControllerConstructor{
			Name:                  string(platform.ControllerTektonConfig),
			ControllerConstructor: k8sConfig.NewController,
		},
		platform.ControllerTektonPipeline: injection.NamedControllerConstructor{
			Name:                  string(platform.ControllerTektonPipeline),
			ControllerConstructor: k8sPipeline.NewController,
		},
		platform.ControllerTektonTrigger: injection.NamedControllerConstructor{
			Name:                  string(platform.ControllerTektonTrigger),
			ControllerConstructor: k8sTrigger.NewController,
		},
		platform.ControllerTektonHub: injection.NamedControllerConstructor{
			Name:                  string(platform.ControllerTektonHub),
			ControllerConstructor: k8sHub.NewController},
		platform.ControllerTektonChain: injection.NamedControllerConstructor{
			Name:                  string(platform.ControllerTektonChain),
			ControllerConstructor: k8sChain.NewController},
		platform.ControllerManualApprovalGate: injection.NamedControllerConstructor{
			Name:                  string(platform.ControllerManualApprovalGate),
			ControllerConstructor: k8sManualApprovalGate.NewController},
		platform.ControllerTektonScheduler: injection.NamedControllerConstructor{
			Name:                  string(platform.ControllerTektonScheduler),
			ControllerConstructor: k8stektonscheduler.NewController},
		platform.ControllerTektonPruner: injection.NamedControllerConstructor{
			Name:                  string(platform.ControllerTektonPruner),
			ControllerConstructor: k8sTektonPruner.NewController},
		platform.ControllerTektonInstallerSet: injection.NamedControllerConstructor{
			Name:                  string(platform.ControllerTektonInstallerSet),
			ControllerConstructor: k8sInstallerSet.NewController},
		ControllerTektonDashboard: injection.NamedControllerConstructor{
			Name:                  string(ControllerTektonDashboard),
			ControllerConstructor: k8sDashboard.NewController},
		ControllerTektonResults: injection.NamedControllerConstructor{
			Name:                  string(ControllerTektonResults),
			ControllerConstructor: k8sResult.NewController},
	}
)
