/*
Copyright 2020 The Tekton Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    https://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helpers

import (
	// Mysteriously required to support GCP auth (required by k8s libs).
	// Apparently just importing it is enough. @_@ side effects @_@.
	// https://github.com/kubernetes/client-go/issues/242
	"testing"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	pkgTest "knative.dev/pkg/test"
)

// Setup creates the client objects needed in the e2e tests.
func Setup(t *testing.T) *Clients {
	clients, err := NewClients(
		pkgTest.Flags.Kubeconfig,
		pkgTest.Flags.Cluster)
	if err != nil {
		t.Fatalf("Couldn't initialize clients: %v", err)
	}
	return clients
}
