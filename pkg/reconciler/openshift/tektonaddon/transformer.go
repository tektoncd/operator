/*
Copyright 2021 The Tekton Authors

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
	"fmt"

	mf "github.com/manifestival/manifestival"
	console "github.com/openshift/api/console/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func replaceKind(fromKind, toKind string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		kind := u.GetKind()
		if kind != fromKind {
			return nil
		}
		err := unstructured.SetNestedField(u.Object, toKind, "kind")
		if err != nil {
			return fmt.Errorf(
				"failed to change resource Name:%s, KIND from %s to %s, %s",
				u.GetName(),
				fromKind,
				toKind,
				err,
			)
		}
		return nil
	}
}

//injectLabel adds label key:value to a resource
// overwritePolicy (Retain/Overwrite) decides whehther to overwrite an already existing label
// []kinds specify the Kinds on which the label should be applied
// if len(kinds) = 0, label will be apllied to all/any resources irrespective of its Kind
func injectLabel(key, value string, overwritePolicy int, kinds ...string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		kind := u.GetKind()
		if len(kinds) != 0 && !itemInSlice(kind, kinds) {
			return nil
		}
		labels, found, err := unstructured.NestedStringMap(u.Object, "metadata", "labels")
		if err != nil {
			return fmt.Errorf("could not find labels set, %q", err)
		}
		if overwritePolicy == retain && found {
			if _, ok := labels[key]; ok {
				return nil
			}
		}
		if !found {
			labels = map[string]string{}
		}
		labels[key] = value
		err = unstructured.SetNestedStringMap(u.Object, labels, "metadata", "labels")
		if err != nil {
			return fmt.Errorf("error updating labels for %s:%s, %s", kind, u.GetName(), err)
		}
		return nil
	}
}

func itemInSlice(item string, items []string) bool {
	for _, v := range items {
		if v == item {
			return true
		}
	}
	return false
}

func getlinks(baseURL string) []console.CLIDownloadLink {
	platforms := []struct {
		key   string
		value string
	}{
		{"Linux", "tkn/tkn-linux-amd64-0.21.0.tar.gz"},
		{"IBM Power", "tkn/tkn-linux-ppc64le-0.21.0.tar.gz"},
		{"IBM Z", "tkn/tkn-linux-s390x-0.21.0.tar.gz"},
		{"Mac", "tkn/tkn-macos-amd64-0.21.0.tar.gz"},
		{"Windows", "tkn/tkn-windows-amd64-0.21.0.zip"},
	}
	links := []console.CLIDownloadLink{}
	for _, platform := range platforms {
		links = append(links, console.CLIDownloadLink{
			Href: getURL(baseURL, platform.value),
			Text: fmt.Sprintf("Download tkn for %s", platform.key),
		})
	}
	return links
}

func getURL(baseURL string, path string) string {
	return fmt.Sprintf("https://%s/%s", baseURL, path)
}

func replaceURLCCD(baseURL string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "ConsoleCLIDownload" {
			return nil
		}
		ccd := &console.ConsoleCLIDownload{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, ccd)
		if err != nil {
			return err
		}
		ccd.Spec.Links = getlinks(baseURL)
		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ccd)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)
		return nil
	}
}
