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
package imagereplacement

import (
	mf "github.com/jcrossley3/manifestival"
	v1alpha1 "github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/controller/common"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var (
	log = logf.Log.WithName("image-replacement")
)

// New creates a new Imagereplacement
func New() (*Imagereplacement, error) {
	return &Imagereplacement{}, nil
}

// Imagereplacement define Activity to replace image url in the yaml
type Imagereplacement struct {
	Extension        common.Extension
	extensionWrapper *v1alpha1.ExtensionWapper
	scheme           *runtime.Scheme
}

// Configure decide if the imageReplacement need to be added to process chain
func (ir Imagereplacement) Configure(c client.Client, s *runtime.Scheme, extensionWapper *v1alpha1.ExtensionWapper) (*common.Extension, error) {
	if extensionWapper.Registry != nil && extensionWapper.Registry.Override != nil {
		ir.scheme = s
		ir.extensionWrapper = extensionWapper
		ir.Extension = common.Extension{
			Transformers: []mf.Transformer{ir.egress},
		}
		return &ir.Extension, nil
	}

	return nil, nil
}

func (ir Imagereplacement) egress(u *unstructured.Unstructured) error {
	if u.GetKind() == "Deployment" {
		var deploy = &appsv1.Deployment{}
		if err := ir.scheme.Convert(u, deploy, nil); err != nil {
			return err
		}
		registry := ir.extensionWrapper.Registry
		err := UpdateDeployment(deploy, registry, log)
		if err != nil {
			return err
		}
		if err := ir.scheme.Convert(deploy, u, nil); err != nil {
			return err
		}
	}
	return nil
}
