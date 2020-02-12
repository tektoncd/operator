/*
Copyright 2019 The Knative Authors

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
package common

import (
	mf "github.com/jcrossley3/manifestival"
	v1alpha1 "github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("common")

// Activity is a interface, func Configure will be implemented by each Extension as a register. It will decide if
// the Extension will be add to process chain.
type Activity interface {
	Configure(client.Client, *runtime.Scheme, *v1alpha1.ExtensionWapper) (*Extension, error)
}

// Activities is array of Activity
type Activities []Activity

// Extensions is array of Extension
type Extensions []Extension

// Extension is a plugin, could be remove/add easily
type Extension struct {
	Transformers []mf.Transformer
}

// Extend produce Extensions by invoke Activities
func (activities Activities) Extend(c client.Client, scheme *runtime.Scheme, extensionWapper *v1alpha1.ExtensionWapper) (result Extensions, err error) {
	for _, activity := range activities {
		ext, err := activity.Configure(c, scheme, extensionWapper)
		if err != nil {
			return result, err
		}
		if ext != nil {
			result = append(result, *ext)
		}
	}
	return
}

// Transform do real work
func (exts Extensions) Transform() []mf.Transformer {
	result := []mf.Transformer{}

	for _, extension := range exts {
		result = append(result, extension.Transformers...)
	}

	return result
}
