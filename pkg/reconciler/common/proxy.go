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
package common

import (
	"os"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ApplyProxySettings is a transformer that propagate any proxy environment variables
// set on the operator deployment to the underlying deployment.
func ApplyProxySettings(u *unstructured.Unstructured) error {
	if u.GetKind() != "Deployment" {
		// Don't do anything on something else than Deployment
		return nil
	}

	var proxyEnv = []corev1.EnvVar{{
		Name:  "HTTPS_PROXY",
		Value: os.Getenv("HTTPS_PROXY"),
	}, {
		Name:  "HTTP_PROXY",
		Value: os.Getenv("HTTP_PROXY"),
	}, {
		Name:  "NO_PROXY",
		Value: os.Getenv("NO_PROXY"),
	}}

	m := u.Object
	containers, found, err := unstructured.NestedSlice(m, "spec", "template", "spec", "containers")
	if err != nil {
		return err
	}
	if !found {
		// No containers in the deployment, it is weird but let's not fail
		return nil
	}
	for _, c := range containers {
		envs, err := extractEnvs(c.(map[string]interface{}))
		if err != nil {
			return err
		}
		for _, e := range proxyEnv {
			if e.Value == "" {
				// Remove existing envvar if they are not set.
				// This probably means the proxy configuration has been removed
				delete(envs, e.Name)
				continue
			}
			envs[e.Name] = e.Value
		}
		if len(envs) == 0 {
			unstructured.RemoveNestedField(c.(map[string]interface{}), "env")
		} else if err := unstructured.SetNestedSlice(c.(map[string]interface{}), toUnstructured(envs), "env"); err != nil {
			return err
		}
	}
	if err := unstructured.SetNestedField(m, containers, "spec", "template", "spec", "containers"); err != nil {
		return err
	}

	u.SetUnstructuredContent(m)
	return nil
}

func extractEnvs(uc map[string]interface{}) (map[string]interface{}, error) {
	currentEnv, found, err := unstructured.NestedSlice(uc, "env")
	if err != nil {
		return nil, err
	}
	if !found {
		return map[string]interface{}{}, nil
	}
	envs := make(map[string]interface{}, len(currentEnv))
	for _, e := range currentEnv {
		em := e.(map[string]interface{})
		envs[em["name"].(string)] = em

	}
	return envs, nil
}

func toUnstructured(envs map[string]interface{}) []interface{} {
	newEnv := []interface{}{}
	for n, v := range envs {
		switch va := v.(type) {
		case map[string]interface{}:
			newEnv = append(newEnv, va)
		case map[string]string:
			newEnv = append(newEnv, va)
		default:
			newEnv = append(newEnv, map[string]interface{}{
				"name":  n,
				"value": va,
			})
		}
	}
	sort.Slice(newEnv, func(i, j int) bool {
		return newEnv[i].(map[string]interface{})["name"].(string) < newEnv[j].(map[string]interface{})["name"].(string)
	})
	return newEnv
}
