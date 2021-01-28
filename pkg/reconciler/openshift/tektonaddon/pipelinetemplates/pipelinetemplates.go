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

package tektonaddon

import (
	mf "github.com/manifestival/manifestival"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"path"
)

type generateDeployTask func(map[string]interface{}) map[string]interface{}
type taskGenerator interface {
	generate(pipeline unstructured.Unstructured, usingPipelineResource bool) (unstructured.Unstructured, error)
}

type pipeline struct {
	environment string
	nameSuffix  string
	generateDeployTask
}

type RuntimeSpec struct {
	Runtime           string
	Version           string
	MinorVersion      string
	SupportedVersions string
	Default           string
}

const (
	LabelPipelineEnvironmentType        = "pipeline.openshift.io/type"
	LabelPipelineStrategy               = "pipeline.openshift.io/strategy"
	LabelPipelineRuntime                = "pipeline.openshift.io/runtime"
	AnnotationPreserveNS                = "operator.tekton.dev/preserve-namespace"
	AnnotationPipelineSupportedVersions = "pipeline.openshift.io/supported-versions"
)

var (
	Runtimes = map[string]RuntimeSpec{
		"s2i-dotnet-3": {Runtime: "dotnet", MinorVersion: "$(params.MINOR_VERSION)", SupportedVersions: "[3.1,3.0]", Default: "1"},
		"s2i-go":       {Runtime: "golang"},
		"s2i-java-8":   {Runtime: "java", SupportedVersions: "[8]"},
		"s2i-java-11":  {Runtime: "java", SupportedVersions: "[11]"},
		"s2i-nodejs":   {Runtime: "nodejs", Version: "$(params.MAJOR_VERSION)", SupportedVersions: "[10,12]", Default: "12"},
		"s2i-perl":     {Runtime: "perl", MinorVersion: "$(params.MINOR_VERSION)", SupportedVersions: "[5.30,5.26]", Default: "30"},
		"s2i-php":      {Runtime: "php", MinorVersion: "$(params.MINOR_VERSION)", SupportedVersions: "[7.2,7.3]", Default: "3"},
		"s2i-python-3": {Runtime: "python", MinorVersion: "$(params.MINOR_VERSION)", SupportedVersions: "[3.8,3.6-ubi8]", Default: "8"},
		"s2i-ruby":     {Runtime: "ruby", MinorVersion: "$(params.MINOR_VERSION)", SupportedVersions: "[2.7,2.6,2.5]", Default: "7"},
		"buildah":      {},
	}
)

func GeneratePipelineTemplates(templatePath string, manifest *mf.Manifest) error {
	var pipelines []unstructured.Unstructured
	usingPipelineResource := true

	workspacedTemplate, err := mf.NewManifest(path.Join(templatePath, "pipeline_using_workspace.yaml"))
	if err != nil {
		return err
	}

	workspacedTaskGenerators := []taskGenerator{
		&pipeline{environment: "openshift", nameSuffix: "", generateDeployTask: openshiftDeployTask},
		&pipeline{environment: "kubernetes", nameSuffix: "-deployment", generateDeployTask: kubernetesDeployTask},
		&pipeline{environment: "knative", nameSuffix: "-knative", generateDeployTask: knativeDeployTask},
	}
	wps, err := generateBasePipeline(workspacedTemplate, workspacedTaskGenerators, !usingPipelineResource)
	if err != nil {
		return err
	}
	pipelines = append(pipelines, wps...)

	resourcedTemplate, err := mf.NewManifest(path.Join(templatePath, "pipeline_using_resource.yaml"))
	if err != nil {
		return err
	}

	resourcedTaskGenerators := []taskGenerator{
		&pipeline{environment: "openshift", nameSuffix: "", generateDeployTask: openshiftDeployTask},
		&pipeline{environment: "kubernetes", nameSuffix: "-deployment", generateDeployTask: kubernetesDeployTask},
		&pipeline{environment: "knative", nameSuffix: "-knative", generateDeployTask: knativeResourcedDeployTask},
	}
	rps, err := generateBasePipeline(resourcedTemplate, resourcedTaskGenerators, usingPipelineResource)
	if err != nil {
		return err
	}
	pipelines = append(pipelines, rps...)
	generatedPipelines, err := mf.ManifestFrom(mf.Slice(pipelines), mf.UseClient(manifest.Client))
	if err != nil {
		return err
	}

	*manifest = manifest.Append(generatedPipelines)
	return nil
}

func (p *pipeline) generate(pipeline unstructured.Unstructured, usingPipelineResource bool) (unstructured.Unstructured, error) {
	newTempRes := unstructured.Unstructured{}
	pipeline.DeepCopyInto(&newTempRes)
	labels := newTempRes.GetLabels()
	labels[LabelPipelineEnvironmentType] = p.environment
	newTempRes.SetLabels(labels)
	updatedName := newTempRes.GetName()
	updatedName += p.nameSuffix
	taskDeploy, found, err := unstructured.NestedFieldNoCopy(newTempRes.Object, "spec", "tasks")
	if !found || err != nil {
		return unstructured.Unstructured{}, err
	}

	var index = 2
	if usingPipelineResource {
		index = 1
		updatedName += "-pr"
	}
	newTempRes.SetName(updatedName)

	p.generateDeployTask(taskDeploy.([]interface{})[index].(map[string]interface{}))
	return newTempRes, nil
}

func openshiftDeployTask(deployTask map[string]interface{}) map[string]interface{} {
	deployTask["taskRef"] = map[string]interface{}{"name": "openshift-client", "kind": "ClusterTask"}
	deployTask["runAfter"] = []interface{}{"build"}
	deployTask["params"] = []interface{}{
		map[string]interface{}{"name": "ARGS", "value": []interface{}{"rollout", "status", "dc/$(params.APP_NAME)"}},
	}
	return deployTask
}

func kubernetesDeployTask(deployTask map[string]interface{}) map[string]interface{} {
	deployTask["taskRef"] = map[string]interface{}{"name": "openshift-client", "kind": "ClusterTask"}
	deployTask["runAfter"] = []interface{}{"build"}
	deployTask["params"] = []interface{}{
		map[string]interface{}{"name": "SCRIPT", "value": "kubectl $@"},
		map[string]interface{}{"name": "ARGS", "value": []interface{}{"rollout", "status", "deploy/$(params.APP_NAME)"}},
	}
	return deployTask
}

func knativeDeployTask(deployTask map[string]interface{}) map[string]interface{} {
	deployTask["name"] = "kn-service-create"
	deployTask["taskRef"] = map[string]interface{}{"name": "kn", "kind": "ClusterTask"}
	deployTask["runAfter"] = []interface{}{"build"}
	deployTask["params"] = []interface{}{
		map[string]interface{}{"name": "ARGS", "value": []interface{}{"service", "create", "$(params.APP_NAME)", "--image=$(params.IMAGE_NAME)", "--force"}},
	}
	return deployTask
}

func knativeResourcedDeployTask(deployTask map[string]interface{}) map[string]interface{} {
	deployTask["name"] = "kn-service-create"
	deployTask["taskRef"] = map[string]interface{}{"name": "kn", "kind": "ClusterTask"}
	deployTask["runAfter"] = []interface{}{"build"}
	deployTask["resources"] = map[string]interface{}{
		"inputs": []interface{}{map[string]interface{}{"name": "image", "resource": "app-image", "from": []interface{}{"build"}}},
	}
	deployTask["params"] = []interface{}{
		map[string]interface{}{"name": "ARGS", "value": []interface{}{"service", "create", "$(params.APP_NAME)", "--image=$(resources.inputs.image.url)", "--force"}},
	}
	return deployTask
}

func generateBasePipeline(template mf.Manifest, taskGenerators []taskGenerator, usingPipelineResource bool) ([]unstructured.Unstructured, error) {
	var pipelines []unstructured.Unstructured

	for name, spec := range Runtimes {
		contextParamName := "PATH_CONTEXT"
		newTempRes := unstructured.Unstructured{}
		template.Resources()[0].DeepCopyInto(&newTempRes)
		labels := map[string]string{}
		annotations := map[string]string{}
		if name == "buildah" {
			labels[LabelPipelineStrategy] = "docker"
			contextParamName = "CONTEXT"
		} else {
			labels[LabelPipelineRuntime] = spec.Runtime
		}

		annotations[AnnotationPreserveNS] = "true"
		if spec.SupportedVersions != "" {
			annotations[AnnotationPipelineSupportedVersions] = spec.SupportedVersions
		}
		newTempRes.SetAnnotations(annotations)
		newTempRes.SetLabels(labels)
		newTempRes.SetName(name)
		pipelineParams, found, err := unstructured.NestedFieldNoCopy(newTempRes.Object, "spec", "params")
		if !found || err != nil {
			return nil, err
		}

		tasks, found, err := unstructured.NestedFieldNoCopy(newTempRes.Object, "spec", "tasks")
		if !found || err != nil {
			return nil, err
		}

		taskName := name
		var index = 1
		if usingPipelineResource {
			index = 0
			taskName += "-pr"
		}

		taskBuild := tasks.([]interface{})[index].(map[string]interface{})
		taskBuild["taskRef"] = map[string]interface{}{"name": taskName, "kind": "ClusterTask"}
		taskParams, found, err := unstructured.NestedFieldNoCopy(taskBuild, "params")
		if !found || err != nil {
			return nil, err
		}

		taskParams = append(taskParams.([]interface{}), map[string]interface{}{"name": contextParamName, "value": "$(params.PATH_CONTEXT)"})

		if spec.Version != "" {
			taskParams = append(taskParams.([]interface{}), map[string]interface{}{"name": "VERSION", "value": spec.Version})
			pipelineParams = append(pipelineParams.([]interface{}), map[string]interface{}{"name": "MAJOR_VERSION", "type": "string", "default": spec.Default})
		}
		if spec.MinorVersion != "" {
			taskParams = append(taskParams.([]interface{}), map[string]interface{}{"name": "MINOR_VERSION", "value": spec.MinorVersion})
			pipelineParams = append(pipelineParams.([]interface{}), map[string]interface{}{"name": "MINOR_VERSION", "type": "string", "default": spec.Default})
		}

		if err := unstructured.SetNestedField(newTempRes.Object, pipelineParams, "spec", "params"); err != nil {
			return nil, err
		}

		if err := unstructured.SetNestedField(taskBuild, taskParams, "params"); err != nil {
			return nil, nil
		}

		//adding the deploy task
		for _, tg := range taskGenerators {
			p, err := tg.generate(newTempRes, usingPipelineResource)
			if err != nil {
				return nil, err
			}
			pipelines = append(pipelines, p)
		}
	}
	return pipelines, nil
}
