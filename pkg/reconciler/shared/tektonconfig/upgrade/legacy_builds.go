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

package upgrade

import (
	"context"
	"fmt"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func preUpgradeLegacyBuilds(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error {
	if !v1alpha1.IsOpenShiftPlatform() {
		return nil
	}
	client, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return err
	}
	return adoptLegacyBuilds(ctx, operatorClient, client)
}

// adoptLegacyBuilds preserves standalone component enablement during a Pipelines
// upgrade. Explicit settings take precedence independently for each component.
// Component reconcilers perform the handoff; adoption never deletes legacy data.
func adoptLegacyBuilds(ctx context.Context, operatorClient versioned.Interface, client dynamic.Interface) error {
	tc, err := operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	platform := tc.Spec.Platforms.OpenShift
	if platform != nil && platform.ShipwrightBuild != nil && platform.SharedResource != nil {
		return nil
	}
	gvr := schema.GroupVersionResource{Group: "operator.openshift.io", Version: "v1alpha1", Resource: "openshiftbuilds"}
	legacy, err := client.Resource(gvr).Get(ctx, "cluster", metav1.GetOptions{})
	if apierrors.IsNotFound(err) || apimeta.IsNoMatchError(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("detect standalone OpenShift Builds for upgrade: %w", err)
	}
	if legacy.GetDeletionTimestamp() != nil {
		return fmt.Errorf("cannot adopt standalone OpenShift Builds while its parent CR is being deleted")
	}
	if platform == nil {
		platform = &v1alpha1.OpenShift{}
		tc.Spec.Platforms.OpenShift = platform
	}
	changed := false
	if platform.ShipwrightBuild == nil {
		enabled, err := legacyComponentEnabled(legacy, "spec", "shipwright", "build", "state")
		if err != nil {
			return err
		}
		if enabled {
			platform.ShipwrightBuild = &v1alpha1.ShipwrightBuildConfig{Enabled: true}
			changed = true
		}
	}
	if platform.SharedResource == nil {
		enabled, err := legacyComponentEnabled(legacy, "spec", "sharedResource", "state")
		if err != nil {
			return err
		}
		if enabled {
			platform.SharedResource = &v1alpha1.SharedResourceConfig{Enabled: true}
			changed = true
		}
	}
	if !changed {
		return nil
	}
	_, err = operatorClient.OperatorV1alpha1().TektonConfigs().Update(ctx, tc, metav1.UpdateOptions{})
	return err
}

func legacyComponentEnabled(legacy *unstructured.Unstructured, fields ...string) (bool, error) {
	state, _, err := unstructured.NestedString(legacy.Object, fields...)
	if err != nil {
		return false, err
	}
	switch state {
	case "Disabled":
		return false, nil
	case "", "Enabled": // Legacy defaults both omitted components to Enabled.
		return true, nil
	default:
		return false, fmt.Errorf("unknown standalone Builds state %q at %v", state, fields)
	}
}
