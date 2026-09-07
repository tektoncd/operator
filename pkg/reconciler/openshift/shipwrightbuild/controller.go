/*
Copyright 2026 The Tekton Authors

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

// Package shipwrightbuild installs the Shipwright Build whose desired state is expressed through the
// ShipwrightBuild CRD.

package shipwrightbuild

import (
	"context"

	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/shipwrightbuild"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return shipwrightbuild.NewExtendedController(NewOpenShiftExtension)(ctx, cmw)
}
