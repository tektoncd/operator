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

package v1alpha1

import (
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
)

// TestPrune_PrunePerResourceJSONRoundTrip guards against the "prune-per-resource"
// field being dropped when explicitly set to false. The admission webhook computes
// a round-trip patch by marshaling a freshly unmarshaled copy of the request object
// and diffing it against the original bytes; a `false` value on a field tagged
// `omitempty` is indistinguishable from an unset field once marshaled, so the
// webhook would emit a patch removing the key from the stored object.
func TestPrune_PrunePerResourceJSONRoundTrip(t *testing.T) {
	raw := []byte(`{"disabled":false,"prune-per-resource":false}`)

	var p Prune
	err := json.Unmarshal(raw, &p)
	assert.NilError(t, err)
	assert.Equal(t, p.PrunePerResource, false)

	out, err := json.Marshal(p)
	assert.NilError(t, err)

	var roundTripped map[string]interface{}
	err = json.Unmarshal(out, &roundTripped)
	assert.NilError(t, err)

	_, present := roundTripped["prune-per-resource"]
	assert.Assert(t, present, "prune-per-resource key was dropped from marshaled JSON when false: %s", string(out))
}
