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
	"testing"

	"gotest.tools/v3/assert"
	"k8s.io/client-go/rest"
)

func TestClientReusedAcrossInjectionAndAdmission(t *testing.T) {
	assert.Assert(t, Get(context.Background()) == nil)
	ctx := withClientFromConfig(context.Background(), &rest.Config{Host: "https://example.com"})
	client := Get(ctx)
	assert.Assert(t, client != nil)

	// Startup enables injection more than once; do not recreate the client.
	reinjected := withClientFromConfig(ctx, &rest.Config{Host: "https://example.com"})
	assert.Assert(t, Get(reinjected) == client)

	requestCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	requestCtx = WithClient(requestCtx, client)
	assert.Assert(t, Get(requestCtx) == client)
	cancel()
	assert.ErrorIs(t, requestCtx.Err(), context.Canceled)
}
