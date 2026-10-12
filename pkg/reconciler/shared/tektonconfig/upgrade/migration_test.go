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

package upgrade

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrationRegistryMetadata enforces that every migration entry in the
// pre/post upgrade registries is fully documented before it is merged.
// This is the addition gate: you cannot add a migration without setting all
// required fields.
func TestMigrationRegistryMetadata(t *testing.T) {
	all := allRegisteredMigrations()
	seen := map[string]struct{}{}

	for _, m := range all {
		require.NotEmptyf(t, m.Name, "migration Name is required")
		require.NotEmptyf(t, m.Introduced, "Introduced is required for %q", m.Name)
		require.NotEmptyf(t, m.DeletableFrom, "DeletableFrom is required for %q — set to Introduced + 2 minors (or higher for ELC)", m.Name)
		require.NotEmptyf(t, m.Description, "Description is required for %q", m.Name)
		require.NotNilf(t, m.Fn, "Fn is required for %q", m.Name)

		_, dup := seen[m.Name]
		assert.Falsef(t, dup, "duplicate migration name %q", m.Name)
		seen[m.Name] = struct{}{}
	}
}

func allRegisteredMigrations() []Migration {
	all := make([]Migration, 0, len(preUpgradeMigrations)+len(postUpgradeMigrations))
	all = append(all, preUpgradeMigrations...)
	all = append(all, postUpgradeMigrations...)
	return all
}
