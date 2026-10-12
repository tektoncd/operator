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

// Migration is a pre- or post-upgrade function with explicit documentation.
//
// # Adding a migration
//
// Append an entry to preUpgradeMigrations or postUpgradeMigrations with all
// four fields set. TestMigrationRegistryMetadata will fail the build otherwise.
//
// # Removing a migration
//
// A migration may be deleted from the registry once DeletableFrom has shipped
// (i.e. the operator version has reached or passed DeletableFrom). Deletion
// requires reviewer sign-off confirming that every supported upgrade source
// has already had the migration applied.
type Migration struct {
	// Name uniquely identifies the migration. Never reuse a name.
	Name string

	// Introduced is the operator version (e.g. "0.81.0") that first shipped
	// this migration.
	Introduced string

	// DeletableFrom is the first operator version at which this migration may
	// safely be removed. Typically Introduced + 2 minors for normal upgrades;
	// set higher when ELC / skip-level support is required.
	// Example: Introduced "0.81.0" → DeletableFrom "0.83.0".
	DeletableFrom string

	// Description briefly states what the migration does and why.
	Description string

	// Fn is the migration implementation. Must be idempotent.
	Fn upgradeFunc
}

func migrationFns(migrations []Migration) []upgradeFunc {
	fns := make([]upgradeFunc, 0, len(migrations))
	for _, m := range migrations {
		fns = append(fns, m.Fn)
	}
	return fns
}
