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
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"knative.dev/pkg/logging"
)

// Stage represents a step in the reconcile process
type Stage func(context.Context, *mf.Manifest, v1alpha1.TektonComponent) error

// Stages are a list of steps
type Stages []Stage

// Execute each stage in sequence until one returns an error
func (stages Stages) Execute(ctx context.Context, manifest *mf.Manifest, instance v1alpha1.TektonComponent) error {
	for _, stage := range stages {
		if err := stage(ctx, manifest, instance); err != nil {
			return err
		}
	}
	return nil
}

// NoOp does nothing
func NoOp(context.Context, *mf.Manifest, v1alpha1.TektonComponent) error {
	return nil
}

// AppendTarget mutates the passed manifest by appending one
// appropriate for the passed TektonComponent
func AppendTarget(ctx context.Context, manifest *mf.Manifest, instance v1alpha1.TektonComponent) error {
	m, err := TargetManifest(instance)
	if err != nil {
		return err
	}
	*manifest = manifest.Append(m)
	return nil
}

// ManifestFetcher returns a manifest appropriate for the instance
type ManifestFetcher func(ctx context.Context, instance v1alpha1.TektonComponent) (*mf.Manifest, error)

// DeleteObsoleteResources returns a Stage after calculating the
// installed manifest from the instance. This is meant to be called
// *before* executing the reconciliation stages so that the proper
// manifest is captured in a closure before any stage might mutate the
// instance status, e.g. Install.
func DeleteObsoleteResources(ctx context.Context, instance v1alpha1.TektonComponent, fetch ManifestFetcher) Stage {
	if TargetVersion(instance) == instance.GetStatus().GetVersion() {
		return NoOp
	}
	logger := logging.FromContext(ctx)
	installed, err := fetch(ctx, instance)
	if err != nil {
		logger.Error("Unable to obtain the installed manifest; obsolete resources may linger", err)
		return NoOp
	}
	return func(_ context.Context, manifest *mf.Manifest, _ v1alpha1.TektonComponent) error {
		return installed.Filter(mf.NoCRDs, mf.Not(mf.In(*manifest))).Delete()
	}
}
