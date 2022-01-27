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

package common

import (
	"context"

	"github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
)

var webhookNames = map[string]string{
	v1alpha1.PipelineResourceName: "config.webhook.pipeline.tekton.dev",
	v1alpha1.TriggerResourceName:  "config.webhook.triggers.tekton.dev",
}

var webhookServiceNames = map[string]string{
	v1alpha1.PipelineResourceName: "tekton-pipelines-webhook",
	v1alpha1.TriggerResourceName:  "tekton-triggers-webhook",
}

func PreemptDeadlock(ctx context.Context, m *manifestival.Manifest, kc kubernetes.Interface, component string) error {

	// check if there are pod endpoints populated for webhhook service
	webhookServiceName := webhookServiceNames[component]
	ok, err := isWebhookEndpointsActive(m, kc, webhookServiceName)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	// if endpoints are empty, set webhook definition rules
	// to the initial state where the webhook pod can refill the rules when it comes up
	webhookName := webhookNames[component]
	err = removeValidatingWebhookRules(m, kc, webhookName)
	if err != nil {
		return err
	}
	return nil
}

// isWebhookEndpointsActive checks if the there are valid Endpoint resources associated with a webhook service
func isWebhookEndpointsActive(m *manifestival.Manifest, kc kubernetes.Interface, svcName string) (bool, error) {
	svcResource := m.Filter(manifestival.ByKind("Service"), manifestival.ByName(svcName))
	targetNamespace := svcResource.Resources()[0].GetNamespace()
	endPoint, err := kc.CoreV1().Endpoints(targetNamespace).Get(context.TODO(), svcName, v1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if endPoint.Subsets == nil || len(endPoint.Subsets) == 0 {
		return false, nil
	}
	return true, nil
}

// removeValidatingWebhookRules remove "rules" from config.webhook.** webhook definiton(s)
func removeValidatingWebhookRules(m *manifestival.Manifest, kc kubernetes.Interface, webhookName string) error {
	cmValidationWebHookManifest := m.Filter(manifestival.ByName(webhookName))
	cmValidationWebHookManifest, err := cmValidationWebHookManifest.Transform(removeWebhooks)
	if err != nil {
		return err
	}
	return cmValidationWebHookManifest.Apply()
}

// removeWebhooks is a Transformer function which clears our webhooks[...].rules
func removeWebhooks(u *unstructured.Unstructured) error {
	unstructured.RemoveNestedField(u.Object, "webhooks")
	return nil
}
