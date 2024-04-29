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
	"fmt"

	"github.com/tektoncd/operator/pkg/common"
	"github.com/tektoncd/operator/pkg/reconciler/openshift"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
)

func (tc *TektonConfig) Validate(ctx context.Context) (errs *apis.FieldError) {

	if apis.IsInDelete(ctx) {
		return nil
	}

	// disallow to update the targetNamespace
	if apis.IsInUpdate(ctx) {
		existingTC := apis.GetBaseline(ctx).(*TektonConfig)
		if existingTC.Spec.GetTargetNamespace() != tc.Spec.GetTargetNamespace() {
			errs = errs.Also(apis.ErrGeneric("Doesn't allow to update targetNamespace, delete existing TektonConfig and create the updated TektonConfig", "spec.targetNamespace"))
		}
	}

	if tc.GetName() != ConfigResourceName {
		errMsg := fmt.Sprintf("metadata.name,  Only one instance of TektonConfig is allowed by name, %s", ConfigResourceName)
		errs = errs.Also(apis.ErrInvalidValue(tc.GetName(), errMsg))
	}

	// execute common spec validations
	errs = errs.Also(tc.Spec.CommonSpec.validate("spec"))

	if tc.Spec.Profile != "" {
		if isValid := isValueInArray(Profiles, tc.Spec.Profile); !isValid {
			errs = errs.Also(apis.ErrInvalidValue(tc.Spec.Profile, "spec.profile"))
		}
	}

	if IsOpenShiftPlatform() && tc.Spec.Platforms.OpenShift.PipelinesAsCode != nil {
		logger := logging.FromContext(ctx)
		errs = errs.Also(tc.Spec.Platforms.OpenShift.PipelinesAsCode.PACSettings.validate(logger, "spec.platforms.openshift.pipelinesAsCode"))
	}

	// validate SCC config
	if IsOpenShiftPlatform() && tc.Spec.Platforms.OpenShift.SCC != nil {
		defaultSCC := PipelinesSCC
		if tc.Spec.Platforms.OpenShift.SCC.Default != "" {
			defaultSCC = tc.Spec.Platforms.OpenShift.SCC.Default
		}

		// verify default SCC exists on the cluster

		// we don't want to verify pipelines-scc here as it will be created
		// later when the RBAC reconciler will be run
		if defaultSCC != PipelinesSCC {
			if err := verifySCCExists(ctx, tc.Spec.Platforms.OpenShift.SCC.Default); err != nil {
				errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("error verifying SCC exists: %s - %v", tc.Spec.Platforms.OpenShift.SCC.Default, err), "spec.platforms.openshift.scc.default"))
			}
		}

		maxAllowedSCC := tc.Spec.Platforms.OpenShift.SCC.MaxAllowed
		if maxAllowedSCC != "" {
			// verify maxAllowed SCC exists on the cluster
			if err := verifySCCExists(ctx, maxAllowedSCC); err != nil {
				errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("error verifying SCC exists: %s - %v", maxAllowedSCC, err), "spec.platforms.openshift.scc.maxAllowed"))
			}

			// Check that maxAllowed SCC and default SCC are compatible wrt priority
			hasPriority, err := compareSCCAMoreRestrictiveThanB(ctx, defaultSCC, maxAllowedSCC)
			if err != nil {
				errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("error comparing priority between maxAllowed and default SCC in TektonConfig: %v", err), "spec.platforms.openshift.scc.maxAllowed"))
			} else if !hasPriority {
				errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("maxAllowed SCC (%s) must be less restrictive than the default SCC (%s)", maxAllowedSCC, defaultSCC), "spec.platforms.openshift.scc.maxAllowed"))
			}

			// Now validate maxAllowed SCC config with namespaces
			sccErrors, err := compareSCCsWithAllNamespaces(ctx, maxAllowedSCC)
			if err != nil {
				errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("error comparing priority between maxAllowed and SCCs requested in all namespaces: %v", err), "spec.platforms.openshift.scc.maxAllowed"))
			}
			errs = errs.Also(sccErrors)
		}
	}

	// validate pruner specifications
	errs = errs.Also(tc.Spec.Pruner.validate())

	if !tc.Spec.Addon.IsEmpty() {
		errs = errs.Also(validateAddonParams(tc.Spec.Addon.Params, "spec.addon.params"))
	}

	if !tc.Spec.Hub.IsEmpty() {
		errs = errs.Also(validateHubParams(tc.Spec.Hub.Params, "spec.hub.params"))
	}

	errs = errs.Also(tc.Spec.Pipeline.PipelineProperties.validate("spec.pipeline"))

	errs = errs.Also(tc.Spec.Pipeline.Options.validate("spec.pipeline.options"))
	errs = errs.Also(tc.Spec.Hub.Options.validate("spec.hub.options"))
	errs = errs.Also(tc.Spec.Dashboard.Options.validate("spec.dashboard.options"))
	errs = errs.Also(tc.Spec.Chain.Options.validate("spec.chain.options"))
	errs = errs.Also(tc.Spec.Trigger.Options.validate("spec.trigger.options"))

	return errs.Also(tc.Spec.Trigger.TriggersProperties.validate("spec.trigger"))
}

func (p Prune) validate() *apis.FieldError {
	var errs *apis.FieldError

	// if pruner job disable no validation required
	if p.Disabled {
		return errs
	}

	if len(p.Resources) != 0 {
		for i, r := range p.Resources {
			if !isValueInArray(PruningResource, r) {
				errs = errs.Also(apis.ErrInvalidArrayValue(r, "spec.pruner.resources", i))
			}
		}
	} else {
		errs = errs.Also(apis.ErrMissingField("spec.pruner.resources"))
	}

	// tkn cli supports both "keep" and "keep-since", even though there is an issue with the logic
	// when we supply both "keep" and "keep-since", the outcome always equivalent to "keep", "keep-since" ignored
	// hence we strict with a single flag support until the issue is fixed in tkn cli
	// cli issue: https://github.com/tektoncd/cli/issues/1990
	if p.Keep != nil && p.KeepSince != nil {
		errs = errs.Also(apis.ErrMultipleOneOf("spec.pruner.keep", "spec.pruner.keep-since"))
	}

	if p.Keep == nil && p.KeepSince == nil {
		errs = errs.Also(apis.ErrMissingOneOf("spec.pruner.keep", "spec.pruner.keep-since"))
	}
	if p.Keep != nil && *p.Keep == 0 {
		errs = errs.Also(apis.ErrInvalidValue(*p.Keep, "spec.pruner.keep"))
	}
	if p.KeepSince != nil && *p.KeepSince == 0 {
		errs = errs.Also(apis.ErrInvalidValue(*p.KeepSince, "spec.pruner.keep-since"))
	}

	return errs
}

func isValueInArray(arr []string, key string) bool {
	for _, p := range arr {
		if p == key {
			return true
		}
	}
	return false
}

func verifySCCExists(ctx context.Context, sccName string) error {
	securityClient := common.GetSecurityClient(ctx)
	_, err := securityClient.SecurityV1().SecurityContextConstraints().Get(ctx, sccName, metav1.GetOptions{})
	return err
}

func compareSCCAMoreRestrictiveThanB(ctx context.Context, sccA, sccB string) (bool, error) {
	securityClient := common.GetSecurityClient(ctx)
	prioritizedSCCList, err := common.GetSCCRestrictiveList(ctx, securityClient)
	if err != nil {
		return false, err
	}
	return common.SCCAMoreRestrictiveThanB(prioritizedSCCList, sccA, sccB)
}

func compareSCCsWithAllNamespaces(ctx context.Context, maxAllowedSCC string) (*apis.FieldError, error) {
	kc := kubeclient.Get(ctx)
	allNamespaces, err := kc.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var sccErrors *apis.FieldError
	for _, ns := range allNamespaces.Items {
		nsSCC := ns.Annotations[openshift.NamespaceSCCAnnotation]
		if nsSCC == "" {
			continue
		}

		// Compare namespace SCC with maxAllowed
		hasPriority, err := compareSCCAMoreRestrictiveThanB(ctx, nsSCC, maxAllowedSCC)
		if err != nil {
			return nil, err
		}

		if !hasPriority {
			sccErrors = sccErrors.Also(apis.ErrGeneric(fmt.Sprintf("SCC requested in namespace %s: %s violates the maxAllowed SCC: %s set in TektonConfig", ns.Name, nsSCC, maxAllowedSCC)))
		}
	}
	return sccErrors, nil
}
