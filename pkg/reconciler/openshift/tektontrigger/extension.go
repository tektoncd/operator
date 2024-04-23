/*
Copyright 2020 The Tekton Authors

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

package tektontrigger

import (
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektontrigger"
	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	"knative.dev/pkg/ptr"
)

// triggersProperties holds fields for configuring runAsUser and runAsGroup.
type triggersProperties struct {
	DefaultRunAsUser  *string `json:"default-run-as-user,omitempty"`
	DefaultRunAsGroup *string `json:"default-run-as-group,omitempty"`
}

// Updating the default values of runAsUser and runAsGroup to an empty string
// to ensure compatibility with OpenShift's requirements for managing these settings
// in Triggers Eventlistener containers SCC.
var triggersData = triggersProperties{
	DefaultRunAsUser:  ptr.String(""),
	DefaultRunAsGroup: ptr.String(""),
}

func OpenShiftExtension(ctx context.Context) common.Extension {
	return openshiftExtension{}
}

type openshiftExtension struct{}

func (oe openshiftExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	return []mf.Transformer{
		occommon.RemoveRunAsUser(),
		occommon.RemoveRunAsGroup(),
		occommon.ApplyCABundles,
		common.AddConfigMapValues(tektontrigger.ConfigDefaults, triggersData),
		replaceDeploymentArgs("-el-events", "enable"),
	}
}
func (oe openshiftExtension) PreReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {
	return nil
}
func (oe openshiftExtension) PostReconcile(context.Context, v1alpha1.TektonComponent) error {
	return nil
}
func (oe openshiftExtension) Finalize(context.Context, v1alpha1.TektonComponent) error {
	return nil
}
