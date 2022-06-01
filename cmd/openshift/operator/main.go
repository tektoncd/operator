/*
Copyright 2019 The Tekton Authors

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

package main

import (
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset"
	"github.com/tektoncd/operator/pkg/reconciler/openshift/tektonaddon"
	"github.com/tektoncd/operator/pkg/reconciler/openshift/tektonchain"
	"github.com/tektoncd/operator/pkg/reconciler/openshift/tektonconfig"
	"github.com/tektoncd/operator/pkg/reconciler/openshift/tektonhub"
	"github.com/tektoncd/operator/pkg/reconciler/openshift/tektonpipeline"
	"github.com/tektoncd/operator/pkg/reconciler/openshift/tektontrigger"
	installer "github.com/tektoncd/operator/pkg/reconciler/shared/tektoninstallerset"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
)

func main() {

	cfg := injection.ParseAndGetRESTConfigOrDie()
	ctx, _ := injection.EnableInjectionOrDie(signals.NewContext(), cfg)

	installer.InitTektonInstallerSetClient(ctx)

	sharedmain.MainWithConfig(ctx, "tekton-operator", cfg,
		tektonpipeline.NewController,
		tektontrigger.NewController,
		tektonaddon.NewController,
		tektonconfig.NewController,
		tektoninstallerset.NewController,
		tektonhub.NewController,
		tektonchain.NewController,
	)
}
