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

package tektontrigger

import (
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// replace the value of a deployment arg provided argToReplace and value
func replaceDeploymentArgs(argToReplace, value string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" {
			return nil
		}

		d := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		containers := d.Spec.Template.Spec.Containers
		for _, container := range containers {
			if len(container.Args) == 0 {
				continue
			}
			for j, arg := range container.Args {
				// for scenario when key and value are in single arg e.g. "key=value"
				if argVal, hasArg := common.SplitsByEqual(arg); hasArg {
					if argVal[0] == argToReplace {
						container.Args[j] = argVal[0] + "=" + value
					}
					continue
				}
				// for scenario, when key and value are in different args, eg. "key","value"
				if arg == argToReplace {
					container.Args[j+1] = value
				}
			}

		}

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}
