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

package tektonscheduler

import (
	"context"
	"strings"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	certManagerAnnotation = "cert-manager.io/inject-ca-from"
)

func filterAndTransform(extension common.Extension) client.FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		schedulerCR := comp.(*v1alpha1.TektonScheduler)

		imagesRaw := common.ToLowerCaseKeys(common.ImagesFromEnv(common.SchedulerImagePrefix))
		schedulerImages := common.ImageRegistryDomainOverride(imagesRaw)
		extra := []mf.Transformer{
			common.InjectOperandNameLabelOverwriteExisting(v1alpha1.TektonSchedulerResourceName),
			common.DeploymentImages(schedulerImages),
			common.AddDeploymentRestrictedPSA(),
			common.AddConfigMapValues(v1alpha1.SchedulerConfigMapName, schedulerCR.Spec.SchedulerConfig),
			CertificateTransformer(schedulerCR.GetSpec().GetTargetNamespace()),
			MutatingWebhookConfigurationTransformer(ctx, schedulerCR.GetSpec().GetTargetNamespace()),
		}
		extra = append(extra, extension.Transformers(schedulerCR)...)
		err := common.Transform(ctx, manifest, schedulerCR, extra...)
		if err != nil {
			return &mf.Manifest{}, err
		}

		// additional options transformer
		// always execute as last transformer, so that the values in options will be final update values on the manifests
		if err := common.ExecuteAdditionalOptionsTransformer(ctx, manifest, schedulerCR.Spec.GetTargetNamespace(), schedulerCR.Spec.Options); err != nil {
			return &mf.Manifest{}, err
		}

		// Now Remove the TargetNamespace from manifest as same is not owned by Scheduler.
		filteredManifest := manifest.Filter(mf.Not(mf.ByKind("Namespace")), mf.Not(mf.ByName(schedulerCR.GetSpec().GetTargetNamespace())))

		return &filteredManifest, nil
	}
}

func CertificateTransformer(targetNamespace string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Certificate" {
			return nil
		}

		cm := &certv1.Certificate{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, cm)
		if err != nil {
			return err
		}

		// Update DNS entries
		dnsNames := cm.Spec.DNSNames
		for i, v := range dnsNames {
			dnsTokens := strings.Split(v, ".")
			if len(dnsTokens) < 2 {
				continue
			}
			dnsTokens[1] = targetNamespace // ReplaceNameSpace
			dnsNames[i] = strings.Join(dnsTokens, ".")
		}

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

func MutatingWebhookConfigurationTransformer(ctx context.Context, targetNamespace string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "MutatingWebhookConfiguration" {
			annotations := u.GetAnnotations()
			ann := annotations[certManagerAnnotation]
			if ann != "" {
				tokens := strings.Split(ann, "/")
				if len(tokens) >= 2 {
					tokens[0] = targetNamespace
					annotations[certManagerAnnotation] = strings.Join(tokens, "/")

				}
			}
			u.SetAnnotations(annotations)
		}
		return nil
	}
}
