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

package common

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	mf "github.com/manifestival/manifestival"
)

// FetchVersionFromCRD finds the component version from the crd labels, it looks for the
// label on the first crd it finds after filtering
// It will return error if crds are not found in manifest or label is not found
func FetchVersionFromCRD(manifest mf.Manifest, releaseLabel string) (string, error) {
	crds := manifest.Filter(mf.CRDs)
	if len(crds.Resources()) == 0 {
		return "", fmt.Errorf("failed to find crds to get release version")
	}

	crd := crds.Resources()[0]
	version, ok := crd.GetLabels()[releaseLabel]
	if !ok {
		return version, fmt.Errorf("failed to find release label on crd")
	}

	return version, nil
}

// ComputeHashOf generates an unique hash/string for the
// object pass to it.
func ComputeHashOf(obj interface{}) (string, error) {
	h := sha256.New()
	d, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	h.Write(d)
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
