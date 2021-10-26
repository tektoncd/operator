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

package tektonhub

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/client-go/route/clientset/versioned/scheme"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

const (
	tektonHubAPIResourceKey  string = "api"
	tektonHubAuthResourceKey string = "auth"
)

func OpenShiftExtension(ctx context.Context) common.Extension {
	logger := logging.FromContext(ctx)
	mfclient, err := mfc.NewClient(injection.GetConfig(ctx))
	if err != nil {
		logger.Fatalw("error creating client from injected config", zap.Error(err))
	}
	mflogger := zapr.NewLogger(logger.Named("manifestival").Desugar())
	manifest, err := mf.ManifestFrom(mf.Slice{}, mf.UseClient(mfclient), mf.UseLogger(mflogger))
	if err != nil {
		logger.Fatalw("error creating initial manifest", zap.Error(err))
	}

	ext := openshiftExtension{
		operatorClientSet: operatorclient.Get(ctx),
		kubeClientSet:     kubeclient.Get(ctx),
		manifest:          manifest,
	}
	return ext
}

type openshiftExtension struct {
	operatorClientSet versioned.Interface
	kubeClientSet     kubernetes.Interface
	manifest          mf.Manifest
}

func (oe openshiftExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	return nil
}

func (oe openshiftExtension) PreReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {
	th := tc.(*v1alpha1.TektonHub)
	logger := logging.FromContext(ctx)
	targetNs := th.Spec.GetTargetNamespace()
	hubDir := filepath.Join(common.ComponentDir(th), common.TargetVersion(th), tektonHubAPIResourceKey)
	manifest := oe.manifest.Append()

	if err := common.AppendManifest(&manifest, hubDir); err != nil {
		return err
	}
	manifest, err := manifest.Transform(
		mf.InjectOwner(th),
		mf.InjectNamespace(targetNs),
	)
	if err != nil {
		logger.Error("failed to transform manifest")
		return err
	}

	// Just apply the routes and nothing else
	if err := manifest.Filter(mf.ByKind("Route")).Apply(); err != nil {
		return err
	}

	// Get the host of API route
	apiRoute, err := getRouteHost(&manifest, tektonHubAPIResourceKey)
	if err != nil {
		return err
	}
	th.Status.SetApiRoute(fmt.Sprintf("https://%s", apiRoute))

	// Get the host of Auth route
	authRoute, err := getRouteHost(&manifest, tektonHubAuthResourceKey)
	if err != nil {
		return err
	}
	th.Status.SetAuthRoute(fmt.Sprintf("https://%s", authRoute))

	// Update the secrets of API with the Auth Route value
	if err := oe.updateApiSecret(ctx, authRoute, targetNs); err != nil {
		return err
	}

	return nil
}

func (oe openshiftExtension) PostReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {

	return nil
}
func (oe openshiftExtension) Finalize(context.Context, v1alpha1.TektonComponent) error {
	return nil
}

// Updates the AUTH_BASE_URL in the API secret with the Auth Route value
func (oe openshiftExtension) updateApiSecret(ctx context.Context, authRoute, namespace string) error {
	secret, err := oe.kubeClientSet.CoreV1().Secrets(namespace).Get(ctx, tektonHubAPIResourceKey, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if secret.Data["AUTH_BASE_URL"] != nil && len(secret.Data["AUTH_BASE_URL"]) != 0 {
		delete(secret.Data, "AUTH_BASE_URL")
	}

	secret.StringData = make(map[string]string)
	secret.StringData["AUTH_BASE_URL"] = fmt.Sprintf("https://%s", authRoute)

	_, err = oe.kubeClientSet.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

// Get the Host value of the Route created
func getRouteHost(manifest *mf.Manifest, routeName string) (string, error) {
	var hostUrl string
	for _, r := range manifest.Filter(mf.ByKind("Route")).Resources() {
		u, err := manifest.Client.Get(&r)
		if err != nil {
			return "", err
		}
		if u.GetName() == routeName {
			route := &routev1.Route{}
			if err := scheme.Scheme.Convert(u, route, nil); err != nil {
				return "", err
			}
			hostUrl = route.Spec.Host
		}
	}
	return hostUrl, nil
}
