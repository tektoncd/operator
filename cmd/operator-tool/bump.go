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
	"sort"
)

func bump(filename string, bugfix bool) error {
	newComponents := map[string]component{}
	components, err := readCompoments(filename)
	if err != nil {
		return err
	}
	for name, component := range components {
		newComponent, err := bumpComponent(name, component, bugfix)
		if err != nil {
			return err
		}
		newComponents[name] = newComponent
	}
	return writeComponents(filename, newComponents)
}

func bumpComponent(name string, c component, bugfix bool) (component, error) {
	newVersion := c.Version
	newerVersions, err := checkComponentNewerVersions(c, bugfix)
	if err != nil {
		return component{}, err
	}
	if len(newerVersions) > 0 {
		// Get the latest one
		sort.Sort(newerVersions) // sort just in case
		newVersion = "v" + newerVersions[len(newerVersions)-1].String()
	}
	return component{
		Github:  c.Github,
		Version: newVersion,
	}, nil
}
