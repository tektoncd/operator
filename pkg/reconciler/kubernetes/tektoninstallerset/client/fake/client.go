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

package fake

import (
	"context"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	v1alpha12 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

type fakeClient struct {
	resource map[string]*v1alpha1.TektonInstallerSet
}

func NewFakeISClient() v1alpha12.TektonInstallerSetInterface {
	return &fakeClient{
		resource: map[string]*v1alpha1.TektonInstallerSet{},
	}
}

func (f fakeClient) Create(ctx context.Context, tektonInstallerSet *v1alpha1.TektonInstallerSet, opts metav1.CreateOptions) (*v1alpha1.TektonInstallerSet, error) {
	tektonInstallerSet.SetName(tektonInstallerSet.GenerateName + "test")
	if _, ok := f.resource[tektonInstallerSet.GetName()]; ok {
		return nil, errors.NewAlreadyExists(schema.GroupResource{
			Group:    v1alpha1.GroupName,
			Resource: v1alpha1.KindTektonInstallerSet,
		}, tektonInstallerSet.GetName())
	}
	f.resource[tektonInstallerSet.GetName()] = tektonInstallerSet
	return tektonInstallerSet, nil
}

func (f fakeClient) Update(ctx context.Context, tektonInstallerSet *v1alpha1.TektonInstallerSet, opts metav1.UpdateOptions) (*v1alpha1.TektonInstallerSet, error) {
	//TODO implement me
	panic("implement me")
}

func (f fakeClient) UpdateStatus(ctx context.Context, tektonInstallerSet *v1alpha1.TektonInstallerSet, opts metav1.UpdateOptions) (*v1alpha1.TektonInstallerSet, error) {
	//TODO implement me
	panic("implement me")
}

func (f fakeClient) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	//TODO implement me
	panic("implement me")
}

func (f fakeClient) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	//TODO implement me
	panic("implement me")
}

func (f fakeClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1alpha1.TektonInstallerSet, error) {
	//TODO implement me
	panic("implement me")
}

func (f fakeClient) List(ctx context.Context, opts metav1.ListOptions) (*v1alpha1.TektonInstallerSetList, error) {
	//TODO implement me
	panic("implement me")
}

func (f fakeClient) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	//TODO implement me
	panic("implement me")
}

func (f fakeClient) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1alpha1.TektonInstallerSet, err error) {
	//TODO implement me
	panic("implement me")
}
