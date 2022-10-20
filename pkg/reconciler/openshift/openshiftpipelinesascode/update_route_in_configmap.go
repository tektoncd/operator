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
	"fmt"

	mf "github.com/manifestival/manifestival"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/client-go/route/clientset/versioned/scheme"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	controllerRouteName = "pipelines-as-code-controller"
	infoConfigMapName   = "pipelines-as-code-info"
)

func updateControllerRouteInConfigMap(pacManifest *mf.Manifest, targetNs string) error {
	controllerRoute, err := getControllerRouteHost(pacManifest, targetNs)
	if err != nil {
		return fmt.Errorf("failed to get controller route: %v", err)
	}
	if controllerRoute == "" {
		return fmt.Errorf("failed to get host in route, will try again")
	}
	if err := updateInfoConfigMap(controllerRoute, pacManifest, targetNs); err != nil {
		return err
	}
	return nil
}

func getControllerRouteHost(manifest *mf.Manifest, targetNs string) (string, error) {
	var hostUrl string
	for _, r := range manifest.Filter(mf.ByKind("Route")).Resources() {
		r.SetNamespace(targetNs)
		u, err := manifest.Client.Get(&r)
		if err != nil {
			return "", err
		}
		if u.GetName() == controllerRouteName {
			route := &routev1.Route{}
			if err := scheme.Scheme.Convert(u, route, nil); err != nil {
				return "", err
			}
			hostUrl = route.Spec.Host
		}
	}
	return hostUrl, nil
}

func updateInfoConfigMap(route string, pacManifest *mf.Manifest, targetNs string) error {
	for _, r := range pacManifest.Filter(mf.ByKind("ConfigMap")).Resources() {
		if r.GetName() != infoConfigMapName {
			continue
		}
		r.SetNamespace(targetNs)
		u, err := pacManifest.Client.Get(&r)
		if err != nil {
			return err
		}
		cm := &v1.ConfigMap{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, cm)
		if err != nil {
			return err
		}
		routeURL := "https://" + route

		// set controller url if not the same
		if cm.Data["controller-url"] == routeURL {
			return nil
		}
		cm.Data["controller-url"] = routeURL

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
