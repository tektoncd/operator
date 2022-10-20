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

package openshiftpipelinesascode

import (
	"os"
	"path/filepath"
	"strings"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

const (
	pacRuntimeLabel = "pipelinesascode.openshift.io/runtime"
)

var configmapTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: pipelines-as-code-template
  labels:
    app.kubernetes.io/part-of: pipelines-as-code
data:
  template: ""`

func fetchPipelineRunTemplates() (*mf.Manifest, error) {
	prManifests := mf.Manifest{}
	koDataDir := os.Getenv(common.KoEnvKey)
	templateLocation := filepath.Join(koDataDir, "tekton-addon", "pipelines-as-code-templates")
	if err := common.AppendManifest(&prManifests, templateLocation); err != nil {
		return nil, err
	}
	cmManifest, err := pipelineRunToConfigMapConverter(&prManifests)
	if err != nil {
		return nil, err
	}
	return cmManifest, nil
}

func pipelineRunToConfigMapConverter(prManifests *mf.Manifest) (*mf.Manifest, error) {
	cm := &v1.ConfigMap{}
	err := yaml.Unmarshal([]byte(configmapTemplate), cm)
	if err != nil {
		return nil, err
	}

	var temp []unstructured.Unstructured
	for _, res := range prManifests.Resources() {
		if res.GetKind() != "PipelineRun" {
			temp = append(temp, res)
			continue
		}

		data, err := yaml.Marshal(res.Object)
		if err != nil {
			return nil, err
		}

		// set pipelineRun
		cm.Data["template"] = string(data)

		// set metadata
		prname := res.GetName()
		cm.SetName("pipelines-as-code-" + prname)
		cm.Labels[pacRuntimeLabel] = strings.TrimPrefix(prname, "pipelinerun-")

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
		if err != nil {
			return nil, err
		}

		temp = append(temp, unstructured.Unstructured{Object: unstrObj})
	}
	manifest, _ := mf.ManifestFrom(mf.Slice(temp))
	return &manifest, nil
}
