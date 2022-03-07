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
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"golang.org/x/mod/semver"
)

const (
	// KoEnvKey is the key of the environment variable to specify the path to the ko data directory
	KoEnvKey = "KO_DATA_PATH"
	// COMMA is the character comma
	COMMA = ","
)

var cache = map[string]mf.Manifest{}

// TargetVersion returns the version of the manifest to be installed
// per the spec in the component. If spec.version is empty, the latest
// version known to the operator is returned.
func TargetVersion(instance v1alpha1.TektonComponent) string {
	return latestRelease(instance)
}

// TargetManifest returns the manifest for the TargetVersion
func TargetManifest(instance v1alpha1.TektonComponent) (mf.Manifest, error) {
	return Fetch(manifestPath(TargetVersion(instance), instance))
}

// InstalledManifest returns the version currently installed, which is
// harder than it sounds, since status.version isn't set until the
// target version is successfully installed, which can take some time.
// So we return the target manifest if status.version is empty.
func InstalledManifest(instance v1alpha1.TektonComponent) (mf.Manifest, error) {
	current := instance.GetStatus().GetVersion()
	if len(instance.GetStatus().GetManifests()) == 0 && current == "" {
		return TargetManifest(instance)
	}
	return Fetch(installedManifestPath(current, instance))
}

func Fetch(path string) (mf.Manifest, error) {
	if m, ok := cache[path]; ok {
		return m, nil
	}
	result, err := mf.NewManifest(path)
	if err == nil {
		cache[path] = result
	}
	return result, err
}

func ComponentDir(instance v1alpha1.TektonComponent) string {
	koDataDir := os.Getenv(KoEnvKey)
	switch ins := instance.(type) {
	case *v1alpha1.TektonPipeline:
		return filepath.Join(koDataDir, "tekton-pipeline")
	case *v1alpha1.TektonTrigger:
		return filepath.Join(koDataDir, "tekton-trigger")
	case *v1alpha1.TektonDashboard:
		if ins.Spec.Readonly {
			return filepath.Join(koDataDir, "tekton-dashboard/tekton-dashboard-readonly")
		}
		return filepath.Join(koDataDir, "tekton-dashboard/tekton-dashboard-fullaccess")
	case *v1alpha1.TektonAddon:
		return filepath.Join(koDataDir, "tekton-addon")
	case *v1alpha1.TektonConfig:
		return filepath.Join(koDataDir, "tekton-config")
	case *v1alpha1.TektonResult:
		return filepath.Join(koDataDir, "tekton-results")
	case *v1alpha1.TektonHub:
		return filepath.Join(koDataDir, "tekton-hub")
	case *v1alpha1.TektonChain:
		return filepath.Join(koDataDir, "tekton-chains")
	}
	return ""
}

func manifestPath(version string, instance v1alpha1.TektonComponent) string {
	if !semver.IsValid(sanitizeSemver(version)) {
		return ""
	}

	localPath := filepath.Join(ComponentDir(instance), version)
	if _, err := os.Stat(localPath); !os.IsNotExist(err) {
		return localPath
	}

	return ""
}

func installedManifestPath(version string, instance v1alpha1.TektonComponent) string {
	if manifests := instance.GetStatus().GetManifests(); len(manifests) != 0 {
		return strings.Join(manifests, COMMA)
	}

	localPath := filepath.Join(ComponentDir(instance), version)
	if _, err := os.Stat(localPath); !os.IsNotExist(err) {
		return localPath
	}

	return ""
}

// sanitizeSemver always adds `v` in front of the version.
// x.y.z is the standard format we use as the semantic version for Knative. The letter `v` is added for
// comparison purpose.
func sanitizeSemver(version string) string {
	return fmt.Sprintf("v%s", version)
}

// allReleases returns the all the available release versions
// available under kodata directory for Knative component.
func allReleases(instance v1alpha1.TektonComponent) ([]string, error) {
	// List all the directories available under kodata
	pathname := ComponentDir(instance)
	fileList, err := ioutil.ReadDir(pathname)
	if err != nil {
		return nil, err
	}

	releaseTags := make([]string, 0, len(fileList))
	for _, file := range fileList {
		name := path.Join(pathname, file.Name())
		pathDirOrFile, err := os.Stat(name)
		if err != nil {
			return nil, err
		}
		if pathDirOrFile.IsDir() {
			releaseTags = append(releaseTags, file.Name())
		}
	}
	if len(releaseTags) == 0 {
		return nil, fmt.Errorf("unable to find any version number for %v", instance)
	}

	// This function makes sure the versions are sorted in a descending order.
	sort.Slice(releaseTags, func(i, j int) bool {
		// The index i is the one after the index j. If i is more recent than j, return true to swap.
		return semver.Compare(sanitizeSemver(releaseTags[i]), sanitizeSemver(releaseTags[j])) == 1
	})

	return releaseTags, nil
}

// latestRelease returns the latest release tag available under kodata directory for Knative component.
func latestRelease(instance v1alpha1.TektonComponent) string {
	vers, err := allReleases(instance)
	if err != nil {
		panic(err)
	}
	// The versions are in a descending order, so the first one will be the latest version.
	return vers[0]
}

func AppendManifest(manifest *mf.Manifest, yamlLocation string) error {
	m, err := mf.ManifestFrom(mf.Recursive(yamlLocation))
	if err != nil {
		return err
	}
	*manifest = manifest.Append(m)
	return nil
}
