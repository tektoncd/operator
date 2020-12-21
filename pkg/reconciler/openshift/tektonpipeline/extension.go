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

package tektonpipeline

import (
	"context"
	"strings"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// DefaultSA is the default service account
	DefaultSA = "pipeline"
	// DefaultDisableAffinityAssistant is default value of disable affinity assistant flag
	DefaultDisableAffinityAssistant = "true"
	AnnotationPreserveNS            = "operator.tekton.dev/preserve-namespace"
	AnnotationPreserveRBSubjectNS   = "operator.tekton.dev/preserve-rb-subject-namespace"
)

// NoPlatform "generates" a NilExtension
func OpenShiftExtension(context.Context) common.Extension {
	return openshiftExtension{}
}

type openshiftExtension struct{}

func (oe openshiftExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	images := common.ToLowerCaseKeys(common.ImagesFromEnv(common.PipelinesImagePrefix))
	return []mf.Transformer{
		common.DeploymentImages(images),
		injectDefaultSA(DefaultSA),
		setDisableAffinityAssistant(DefaultDisableAffinityAssistant),
		injectNamespaceRoleBindingConditional(AnnotationPreserveNS,
			AnnotationPreserveRBSubjectNS, comp.GetSpec().GetTargetNamespace()),
	}
}
func (oe openshiftExtension) PreReconcile(context.Context, v1alpha1.TektonComponent) error {
	return nil
}
func (oe openshiftExtension) PostReconcile(context.Context, v1alpha1.TektonComponent) error {
	return nil
}
func (oe openshiftExtension) Finalize(context.Context, v1alpha1.TektonComponent) error {
	return nil
}

// injectDefaultSA adds default service account into config-defaults configMap
func injectDefaultSA(defaultSA string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if strings.ToLower(u.GetKind()) != "configmap" {
			return nil
		}
		if u.GetName() != "config-defaults" {
			return nil
		}

		cm := &corev1.ConfigMap{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, cm)
		if err != nil {
			return err
		}

		cm.Data["default-service-account"] = defaultSA
		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
		if err != nil {
			return err
		}

		u.SetUnstructuredContent(unstrObj)
		return nil
	}
}

// setDisableAffinityAssistant set value of disable-affinity-assistant into feature-flags configMap
func setDisableAffinityAssistant(disableAffinityAssistant string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if strings.ToLower(u.GetKind()) != "configmap" {
			return nil
		}
		if u.GetName() != "feature-flags" {
			return nil
		}

		cm := &corev1.ConfigMap{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, cm)
		if err != nil {
			return err
		}

		cm.Data["disable-affinity-assistant"] = disableAffinityAssistant
		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
		if err != nil {
			return err
		}

		u.SetUnstructuredContent(unstrObj)
		return nil
	}
}

func injectNamespaceRoleBindingConditional(preserveNS, preserveRBSubjectNS, targetNamespace string) mf.Transformer {
	tf := injectNamespaceRoleBindingSubjects(targetNamespace)

	return func(u *unstructured.Unstructured) error {
		annotations := u.GetAnnotations()
		val, ok := annotations[preserveNS]
		if !(ok && val == "true") {
			u.SetNamespace(targetNamespace)
		}
		val, ok = annotations[preserveRBSubjectNS]
		if ok && val == "true" {
			return nil
		}
		return tf(u)
	}
}

func injectNamespaceRoleBindingSubjects(targetNamespace string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		kind := strings.ToLower(u.GetKind())
		if kind != "rolebinding" {
			return nil
		}
		subjects, found, err := unstructured.NestedFieldNoCopy(u.Object, "subjects")
		if !found || err != nil {
			return err
		}
		for _, subject := range subjects.([]interface{}) {
			m := subject.(map[string]interface{})
			if _, ok := m["namespace"]; ok {
				m["namespace"] = targetNamespace
			}
		}
		return nil
	}
}
