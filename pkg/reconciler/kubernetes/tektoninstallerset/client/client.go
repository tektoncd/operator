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

package client

import (
	"context"
	"fmt"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientSet "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
)

const (
	InstallerSubTypeStatic     = "static"
	InstallerSubTypeDeployment = "deployment"

	InstallerTypeMain   = "main"
	InstallerTypePre    = "pre"
	InstallerTypePost   = "post"
	InstallerTypeCustom = "custom"
)

var (
	ErrInvalidState        = fmt.Errorf("installer sets in invalid state")
	ErrNotFound            = fmt.Errorf("installer sets not found")
	ErrVersionDifferent    = fmt.Errorf("installer sets release version doesn't match")
	ErrNsDifferent         = fmt.Errorf("installer sets target namespace doesn't match")
	ErrUpdateRequired      = fmt.Errorf("installer sets needs to be updated")
	ErrSetsInDeletionState = fmt.Errorf("installer sets are in deletion state, will come back")
)

type FilterAndTransform func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error)

type InstallerSetClient struct {
	clientSet        clientSet.TektonInstallerSetInterface
	releaseVersion   string
	componentVersion string
	resourceKind     string
	metrics          Metrics
}

func NewInstallerSetClient(clientSet clientSet.TektonInstallerSetInterface, releaseVersion, componentVersion string, resourceKind string, metrics Metrics) *InstallerSetClient {
	return &InstallerSetClient{
		clientSet:        clientSet,
		releaseVersion:   releaseVersion,
		resourceKind:     resourceKind,
		metrics:          metrics,
		componentVersion: componentVersion,
	}
}
