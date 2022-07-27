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
package main

import (
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

type component struct {
	Github  string `json:"github"`
	Version string `json:"version"`
}

func readCompoments(filename string) (map[string]component, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	components := map[string]component{}
	if err := yaml.Unmarshal(data, &components); err != nil {
		return nil, err
	}
	return components, nil
}

func writeComponents(filename string, components map[string]component) error {
	data, err := yaml.Marshal(components)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

func componentVersion(filename string, args []string) error {
	if len(args) == 0 || len(args) > 1 {
		return fmt.Errorf("Need one and only one argument, the component name")
	}
	component := args[0]
	components, err := readCompoments(filename)
	if err != nil {
		return err
	}
	c, ok := components[component]
	if !ok {
		return fmt.Errorf("Component %s not found", component)
	}
	fmt.Print(c.Version)
	return nil
}
