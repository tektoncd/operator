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
package common

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver"
	openshiftconfigclient "github.com/openshift/client-go/config/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	openshiftClient openshiftconfigclient.Interface
)

func GetOCPVersion(ctx context.Context) (*semver.Version, error) {

	if openshiftClient == nil {
		return nil, fmt.Errorf("OpenShift client not initialized")
	}

	// Fetch the ClusterVersion object (always named "version")
	cv, err := openshiftClient.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
	if err != nil {
		// If running on standard Kubernetes, this will return an IsNotFound error.
		// Handle gracefully if your operator supports both vanilla K8s and OCP.
		return nil, err
	}
	versionStr := cv.Status.Desired.Version
	if versionStr == "" {
		return nil, fmt.Errorf("empty OpenShift version in ClusterVersion status")
	}

	v, err := semver.NewVersion(versionStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenShift version %q: %w", versionStr, err)
	}
	return v, nil
}

func SetOpenshiftClient(client openshiftconfigclient.Interface) {
	openshiftClient = client
}
