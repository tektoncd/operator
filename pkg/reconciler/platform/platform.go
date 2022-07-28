package platform

import (
	"context"
	"fmt"
	"log"
	"strings"

	installer "github.com/tektoncd/operator/pkg/reconciler/shared/tektoninstallerset"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
)

// validateControllerNamesOrDie ensures that the list of controller names to be enabled
// are supported by a platform. This function exits on error
func validateControllerNamesOrDie(p Platform) {
	if err := validateControllerNames(p); err != nil {
		log.Fatalf("error validating provided controller names: %v", err)
	}

}

// validateControllerNames ensures that the list of controller names to be enabled
// are supported by a platform
func validateControllerNames(p Platform) error {
	pParams := p.PlatformParams()
	supportedCtrls := p.AllSupportedControllers()
	invalidNamesStr := invalidNames(supportedCtrls, pParams.ControllerNames)
	if len(invalidNamesStr) == 0 {
		return nil
	}
	return ErrorControllerNames(invalidNamesStr, supportedCtrls.ControllerNames())
}

// invalidNames checks if whether there are any names in []CotrollerNames which are
// not present in (supported by) given ControllerMap
func invalidNames(supportedCtrls ControllerMap, cNames []ControllerName) string {
	invalidNames := strings.Builder{}
	for _, cName := range cNames {
		if _, ok := supportedCtrls[cName]; !ok {
			invalidNames.WriteString(string(cName))
			invalidNames.WriteString(",")
		}
	}

	return strings.TrimSuffix(invalidNames.String(), ",")
}

// ErrorControllerNames is a error message format helper
func ErrorControllerNames(invalidNames string, validNames []string) error {
	return fmt.Errorf("un-identified controller names: %s, supported names: %v", invalidNames, validNames)
}

// activeControllers returns a map of the controllers that should be run
// the returned map is a subset of the platform specific map which stores all-supported-controllers
func activeControllers(p Platform) ControllerMap {
	pParams := p.PlatformParams()
	result := ControllerMap{}
	for _, name := range pParams.ControllerNames {
		if namedCtrl, ok := p.AllSupportedControllers()[name]; ok {
			result[name] = namedCtrl
		}
	}
	return result
}

// disabledControllers returns a map of the controllers that should not be run
// the result of disabledControllers is the set of controllers excluded by activeControllers function
// in other words, disabledControllers returns a map which has controllers "not" specified in the controlelrNames input to a platform
// the returned map is a subset of the platform specific map which stores all-supported-controllers
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

func disabledControllers(p Platform) ControllerMap {
	pParams := p.PlatformParams()
	result := p.AllSupportedControllers()
	for _, name := range pParams.ControllerNames {
		delete(result, name)
	}
	return result
}

// contextWithPlatformName  adds platform name to a given context
func contextWithPlatformName(ctx context.Context, pName string) context.Context {
	ctx = context.WithValue(ctx, PlatformNameKey{}, pName)
	return ctx
}

// startMain starts a knative/pkg sharedMain with a context that stores platform name
// and a list of controllers which should be enabled for the given platform
func startMain(p Platform, ctrls ControllerMap) {
	pParams := p.PlatformParams()
	cfg := injection.ParseAndGetRESTConfigOrDie()
	cfg.QPS = 50
	ctx, _ := injection.EnableInjectionOrDie(signals.NewContext(), cfg)
	ctx = contextWithPlatformName(ctx, pParams.Name)
	installer.InitTektonInstallerSetClient(ctx)
	sharedmain.MainWithConfig(ctx,
		pParams.SharedMainName,
		cfg,
		ctrls.ControllerConstructors()...,
	)
}

// StartMainWithAllControllers calls startMain with all controllers
// supported by a platform
func StartMainWithAllControllers(p Platform) {
	startMain(p, p.AllSupportedControllers())
}

// StartMainWithAllControllers calls startMain with a subset of controllers
// specified in the platformConfig of a platform
func StartMainWithSelectedControllers(p Platform) {
	validateControllerNamesOrDie(p)
	selectedCtrls := activeControllers(p)
	startMain(p, selectedCtrls)
}
