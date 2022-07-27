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
	"context"
	"fmt"
	"sort"

	"github.com/Masterminds/semver"
	"github.com/cli/go-gh"
	"golang.org/x/sync/errgroup"
)

func check(filename string, bugfix bool) error {
	components, err := readCompoments(filename)
	if err != nil {
		return err
	}
	g, ctx := errgroup.WithContext(context.Background())
	for name, component := range components {
		// Force scope
		name := name
		component := component

		g.Go(func() error {
			return checkComponent(ctx, name, component, bugfix)
		})
	}
	return g.Wait()
}

func checkComponent(ctx context.Context, name string, component component, bugfix bool) error {
	newerVersion, err := checkComponentNewerVersions(component, bugfix)
	if err != nil {
		return err
	}
	if len(newerVersion) > 0 {
		fmt.Printf("%s: %v\n", name, newerVersion)
	}

	return nil
}

func checkComponentNewerVersions(component component, bugfix bool) (semver.Collection, error) {
	sVersions, err := fetchVersions(component.Github)
	if err != nil {
		return nil, err
	}
	currentVersion, err := semver.NewVersion(component.Version)
	if err != nil {
		return nil, err
	}
	newerVersion, err := getNewerVersion(currentVersion, sVersions, bugfix)
	if err != nil {
		return nil, err
	}
	return newerVersion, nil
}

func fetchVersions(github string) (semver.Collection, error) {
	client, err := gh.RESTClient(nil)
	if err != nil {
		return nil, err
	}
	versions := []struct {
		Name    string
		TagName string `json:"tag_name"`
	}{}
	err = client.Get(fmt.Sprintf("repos/%s/releases", github), &versions)
	if err != nil {
		return nil, err
	}
	sVersions := semver.Collection([]*semver.Version{})
	for _, v := range versions {
		sVersion, err := semver.NewVersion(v.TagName)
		if err != nil {
			return nil, err
		}
		sVersions = append(sVersions, sVersion)
	}
	sort.Sort(sVersions)
	return sVersions, nil
}

func getNewerVersion(currentVersion *semver.Version, versions []*semver.Version, bugfix bool) (semver.Collection, error) {
	constraint := fmt.Sprintf("> %s", currentVersion)
	if bugfix {
		nextMinorVersion := currentVersion.IncMinor()
		constraint = fmt.Sprintf("> %s, < %s", currentVersion, nextMinorVersion.String())
	}
	c, err := semver.NewConstraint(constraint)
	if err != nil {
		return nil, err
	}
	newerVersion := semver.Collection([]*semver.Version{})
	for _, sv := range versions {
		if c.Check(sv) {
			newerVersion = append(newerVersion, sv)
		}
	}
	return newerVersion, nil
}
