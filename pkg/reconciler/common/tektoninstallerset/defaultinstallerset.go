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

package tektoninstallerset

import (
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DefaultInstaller struct {
	Labels          map[string]string
	Annotations     map[string]string
	OwnerReferences []metav1.OwnerReference
	Manifest        mf.Manifest
}

// GetLabels :- Get all the labels
func (di *DefaultInstaller) GetLabels(ctx context.Context) map[string]string {
	return di.Labels
}

// GetAnnotations :- Get all the annotations
func (di *DefaultInstaller) GetAnnotations(ctx context.Context) map[string]string {
	return di.Annotations
}

// GetOwnerReferences :- Get the owner references
func (di *DefaultInstaller) GetOwnerReferences(ctx context.Context) []metav1.OwnerReference {
	return di.OwnerReferences
}

// GetManifest :- Get the manifest
func (di *DefaultInstaller) GetManifest(ctx context.Context) (*mf.Manifest, error) {
	return &di.Manifest, nil
}

func (di *DefaultInstaller) AddLabelKeyVal(key, val string) {
	di.Labels[key] = val
}

func (di *DefaultInstaller) AddLabelsFromMap(labels map[string]string) {
	for k, v := range labels {
		di.AddLabelKeyVal(k, v)
	}
}

func (di *DefaultInstaller) AddAnnotationsKeyVal(key, val string) {
	di.Annotations[key] = val
}

func (di *DefaultInstaller) AddAnnotationsFromMap(annotations map[string]string) {
	for k, v := range annotations {
		di.AddAnnotationsKeyVal(k, v)
	}
}

func (di *DefaultInstaller) AddOwnerReferences(ownerRef metav1.OwnerReference) {
	di.OwnerReferences = append(di.OwnerReferences, ownerRef)
}

func (di *DefaultInstaller) AddManifest(manifest mf.Manifest) {
	di.Manifest = di.Manifest.Append(manifest)
}

func (di *DefaultInstaller) AddTypeLabel(val string) {
	di.AddLabelKeyVal(v1alpha1.InstallerSetType, val)
}

func (di *DefaultInstaller) AddCreatedByLabel(val string) {
	di.AddLabelKeyVal(v1alpha1.CreatedByKey, val)
}

func (di *DefaultInstaller) AddReleaseVersionLabel(val string) {
	di.AddLabelKeyVal(v1alpha1.ReleaseVersionKey, val)
}

func NewDefaultInstaller() *DefaultInstaller {
	return &DefaultInstaller{
		Labels:          map[string]string{},
		Annotations:     map[string]string{},
		OwnerReferences: []metav1.OwnerReference{},
		Manifest:        mf.Manifest{},
	}
}
