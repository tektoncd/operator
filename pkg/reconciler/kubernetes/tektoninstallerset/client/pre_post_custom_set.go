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
	"strings"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"knative.dev/pkg/logging"
)

func (i *InstallerSetClient) PostSet(ctx context.Context, comp v1alpha1.TektonComponent, manifest *mf.Manifest, filterAndTransform FilterAndTransform) error {
	return i.createSet(ctx, comp, InstallerTypePost, manifest, filterAndTransform)
}

func (i *InstallerSetClient) PreSet(ctx context.Context, comp v1alpha1.TektonComponent, manifest *mf.Manifest, filterAndTransform FilterAndTransform) error {
	return i.createSet(ctx, comp, InstallerTypePre, manifest, filterAndTransform)
}

func (i *InstallerSetClient) CustomSet(ctx context.Context, comp v1alpha1.TektonComponent, customName string, manifest *mf.Manifest, filterAndTransform FilterAndTransform) error {
	setType := InstallerTypeCustom + "-" + strings.ToLower(customName)
	return i.createSet(ctx, comp, setType, manifest, filterAndTransform)
}

func (i *InstallerSetClient) createSet(ctx context.Context, comp v1alpha1.TektonComponent, setType string, manifest *mf.Manifest, filterAndTransform FilterAndTransform) error {
	logger := logging.FromContext(ctx)

	sets, err := i.checkSet(ctx, comp, setType)
	if err == nil {
		logger.Infof("%v/%v: found %v installer sets", i.resourceKind, setType, len(sets))
	}

	switch err {
	case ErrNotFound:
		logger.Infof("%v/%v: installer set not found, creating", i.resourceKind, setType)
		sets, err = i.create(ctx, comp, manifest, filterAndTransform, setType)
		if err != nil {
			logger.Errorf("%v/%v: failed to create installer set: %v", i.resourceKind, setType, err)
			return err
		}

	case ErrInvalidState, ErrNsDifferent, ErrVersionDifferent:
		logger.Infof("%v/%v: installer set not in valid state : %v, cleaning up!", i.resourceKind, setType, err)
		if err := i.CleanupSet(ctx, setType); err != nil {
			logger.Errorf("%v/%v: failed to cleanup installer set: %v", i.resourceKind, setType, err)
			return nil
		}
		logger.Infof("%v/%v: returning, will create installer sets in further reconcile", i.resourceKind, setType)
		return v1alpha1.REQUEUE_EVENT_AFTER

	case ErrUpdateRequired:
		logger.Infof("%v/%v: updating installer set", i.resourceKind, setType)
		sets, err = i.update(ctx, comp, sets, manifest, filterAndTransform, setType)
		if err != nil {
			logger.Errorf("%v/%v: update failed : %v", i.resourceKind, setType, err)
			return err
		}
	case ErrSetsInDeletionState:
		logger.Infof("%v/%v: %v", i.resourceKind, setType, err)
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	if err := i.statusCheck(logger, setType, sets); err != nil {
		return err
	}
	return nil
}
