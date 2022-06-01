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
	"fmt"

	mf "github.com/manifestival/manifestival"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ComponentInstaller interface {
	GetManifest(ctx context.Context) (*mf.Manifest, error)
	GetLabels(ctx context.Context) map[string]string
	GetAnnotations(ctx context.Context) map[string]string
	GetOwnerReferences(ctx context.Context) []metav1.OwnerReference
}

// TektonInstallerSet Meta
type tisMeta struct {
	Labels          map[string]string
	Annotations     map[string]string
	OwnerReferences []metav1.OwnerReference
	Name            string
	GenerateName    string
}

func newTisMeta() *tisMeta {
	return &tisMeta{}
}

func newTisMetaWithName(name string) *tisMeta {
	tis := newTisMeta()
	tis.Name = name

	return tis
}

func newTisMetaWithGenerateName(namePrefix string) *tisMeta {
	tis := newTisMeta()
	tis.GenerateName = fmt.Sprintf("%s-", namePrefix)

	return tis
}

func (tis *tisMeta) config(ctx context.Context, ci ComponentInstaller) {
	tis.Labels = ci.GetLabels(ctx)
	tis.Annotations = ci.GetAnnotations(ctx)
	tis.OwnerReferences = ci.GetOwnerReferences(ctx)
}
