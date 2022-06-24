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

package tektonaddon

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/client-go/route/clientset/versioned/scheme"
	"github.com/tektoncd/operator/pkg/reconciler/openshift"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const pacRuntimeLabel = "pipelinesascode.openshift.io/runtime"

var configmapTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: pipelines-as-code-template
  labels:
    app.kubernetes.io/part-of: pipelines-as-code
data:
  template: ""`

var pacLS = metav1.LabelSelector{
	MatchLabels: map[string]string{
		v1alpha1.InstallerSetType: PACInstallerSet,
	},
}

func (r *Reconciler) EnsurePipelinesAsCode(ctx context.Context, ta *v1alpha1.TektonAddon) error {

	pacLabelSelector, err := common.LabelSelector(pacLS)
	if err != nil {
		return err
	}

	if *ta.Spec.EnablePAC {
		exist, err := checkIfInstallerSetExist(ctx, r.operatorClientSet, r.operatorVersion, pacLabelSelector)
		if err != nil {
			return err
		}
		if !exist {
			return r.ensurePAC(ctx, ta)
		}
		return r.updateControllerURL(ta)

	} else {
		// if disabled then delete the installer Set if exist
		if err := r.deleteInstallerSet(ctx, pacLabelSelector); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reconciler) updateControllerURL(ta *v1alpha1.TektonAddon) error {
	var err error
	pacManifest := mf.Manifest{
		Client: r.manifest.Client,
	}

	koDataDir := os.Getenv(common.KoEnvKey)
	pacLocation := filepath.Join(koDataDir, "tekton-addon", "pipelines-as-code")
	if err := common.AppendManifest(&pacManifest, pacLocation); err != nil {
		return err
	}
	pacManifest, err = pacManifest.Transform(mf.InjectNamespace(ta.Spec.TargetNamespace))
	if err != nil {
		return err
	}

	route, err := getControllerRouteHost(&pacManifest)
	if err != nil {
		return err
	}
	if route == "" {
		return v1alpha1.RECONCILE_AGAIN_ERR
	}

	if err := updateInfoConfigMap(route, &pacManifest); err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) ensurePAC(ctx context.Context, ta *v1alpha1.TektonAddon) error {
	pacManifest := mf.Manifest{}

	// core manifest
	koDataDir := os.Getenv(common.KoEnvKey)
	pacLocation := filepath.Join(koDataDir, "tekton-addon", "pipelines-as-code")
	if err := common.AppendManifest(&pacManifest, pacLocation); err != nil {
		return err
	}

	if err := fetchPRTemplates(&pacManifest); err != nil {
		return err
	}

	// installerSet adds it's owner as namespace's owner
	// so deleting tekton addon deletes target namespace too
	// to skip it we filter out namespace
	pacManifest = pacManifest.Filter(mf.Not(mf.ByKind("Namespace")))

	images := common.ToLowerCaseKeys(common.ImagesFromEnv(common.PacImagePrefix))
	// Run transformers
	tfs := []mf.Transformer{
		common.InjectOperandNameLabelOverwriteExisting(openshift.OperandOpenShiftPipelineAsCode),
		common.DeploymentImages(images),
		common.AddConfiguration(ta.Spec.Config),
	}

	if err := r.addonTransform(ctx, &pacManifest, ta, tfs...); err != nil {
		return err
	}

	if err := createInstallerSet(ctx, r.operatorClientSet, ta, pacManifest, r.operatorVersion,
		PACInstallerSet, "addon-pac"); err != nil {
		return err
	}

	return nil
}

func fetchPRTemplates(manifest *mf.Manifest) error {
	prManifests := mf.Manifest{}
	koDataDir := os.Getenv(common.KoEnvKey)
	templateLocation := filepath.Join(koDataDir, "tekton-addon", "pipelines-as-code-templates")
	if err := common.AppendManifest(&prManifests, templateLocation); err != nil {
		return err
	}

	cmManifest, err := pipelineRunToConfigMapConverter(&prManifests)
	if err != nil {
		return err
	}
	*manifest = manifest.Append(*cmManifest)
	return nil
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

func getControllerRouteHost(manifest *mf.Manifest) (string, error) {
	var hostUrl string
	for _, r := range manifest.Filter(mf.ByKind("Route")).Resources() {
		u, err := manifest.Client.Get(&r)
		if err != nil {
			return "", err
		}
		if u.GetName() == "pipelines-as-code-controller" {
			route := &routev1.Route{}
			if err := scheme.Scheme.Convert(u, route, nil); err != nil {
				return "", err
			}
			hostUrl = route.Spec.Host
		}
	}
	return hostUrl, nil
}

func updateInfoConfigMap(route string, pacManifest *mf.Manifest) error {
	for _, r := range pacManifest.Filter(mf.ByKind("ConfigMap")).Resources() {
		if r.GetName() != "pipelines-as-code-info" {
			continue
		}
		u, err := pacManifest.Client.Get(&r)
		if err != nil {
			return err
		}
		cm := &v1.ConfigMap{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, cm)
		if err != nil {
			return err
		}

		// set controller url
		if cm.Data["controller-url"] != "" {
			return nil
		}

		cm.Data["controller-url"] = "https://" + route

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)

		err = pacManifest.Client.Update(u)
		if err != nil {
			return err
		}
	}
	return nil
}
