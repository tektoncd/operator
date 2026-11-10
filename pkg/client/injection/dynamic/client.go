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

package dynamic

import (
	"context"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/injection"
)

type clientKey struct{}

func init() {
	injection.Default.RegisterClient(withClientFromConfig)
}

func withClientFromConfig(ctx context.Context, config *rest.Config) context.Context {
	if Get(ctx) != nil {
		return ctx
	}
	return WithClient(ctx, dynamic.NewForConfigOrDie(config))
}

// WithClient adds the shared dynamic client to an admission or test context.
func WithClient(ctx context.Context, client dynamic.Interface) context.Context {
	return context.WithValue(ctx, clientKey{}, client)
}

// Get returns the injected dynamic client, or nil outside an injected context.
func Get(ctx context.Context) dynamic.Interface {
	client, _ := ctx.Value(clientKey{}).(dynamic.Interface)
	return client
}
