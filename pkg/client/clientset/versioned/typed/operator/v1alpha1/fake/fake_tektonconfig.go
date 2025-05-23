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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorv1alpha1 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	gentype "k8s.io/client-go/gentype"
)

// fakeTektonConfigs implements TektonConfigInterface
type fakeTektonConfigs struct {
	*gentype.FakeClientWithList[*v1alpha1.TektonConfig, *v1alpha1.TektonConfigList]
	Fake *FakeOperatorV1alpha1
}

func newFakeTektonConfigs(fake *FakeOperatorV1alpha1) operatorv1alpha1.TektonConfigInterface {
	return &fakeTektonConfigs{
		gentype.NewFakeClientWithList[*v1alpha1.TektonConfig, *v1alpha1.TektonConfigList](
			fake.Fake,
			"",
			v1alpha1.SchemeGroupVersion.WithResource("tektonconfigs"),
			v1alpha1.SchemeGroupVersion.WithKind("TektonConfig"),
			func() *v1alpha1.TektonConfig { return &v1alpha1.TektonConfig{} },
			func() *v1alpha1.TektonConfigList { return &v1alpha1.TektonConfigList{} },
			func(dst, src *v1alpha1.TektonConfigList) { dst.ListMeta = src.ListMeta },
			func(list *v1alpha1.TektonConfigList) []*v1alpha1.TektonConfig {
				return gentype.ToPointerSlice(list.Items)
			},
			func(list *v1alpha1.TektonConfigList, items []*v1alpha1.TektonConfig) {
				list.Items = gentype.FromPointerSlice(items)
			},
		),
		fake,
	}
}
