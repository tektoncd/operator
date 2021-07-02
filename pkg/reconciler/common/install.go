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
	"fmt"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"knative.dev/pkg/logging"
)

var (
	namespace             mf.Predicate = mf.ByKind("Namespace")
	role                  mf.Predicate = mf.Any(mf.ByKind("ClusterRole"), mf.ByKind("Role"))
	rolebinding           mf.Predicate = mf.Any(mf.ByKind("ClusterRoleBinding"), mf.ByKind("RoleBinding"))
	consoleCLIDownload    mf.Predicate = mf.Any(mf.ByKind("ConsoleCLIDownload"))
	clusterTriggerBinding mf.Predicate = mf.Any(mf.ByKind("ClusterTriggerBinding"))
	deployment            mf.Predicate = mf.Any(mf.ByKind("Deployment"))
)

// Install applies the manifest resources for the given version and updates the given
// status accordingly.
func Install(ctx context.Context, manifest *mf.Manifest, instance v1alpha1.TektonComponent) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Installing manifest")
	status := instance.GetStatus()
	// The Operator needs a higher level of permissions if it 'bind's non-existent roles.
	// To avoid this, we strictly order the manifest application as (Cluster)Roles, then
	// (Cluster)RoleBindings, then the rest of the manifest.
	if err := manifest.Filter(namespace).Apply(); err != nil {
		status.MarkInstallFailed(err.Error())
		return fmt.Errorf("failed to apply namespaces: %w", err)
	}
	if err := manifest.Filter(role).Apply(); err != nil {
		status.MarkInstallFailed(err.Error())
		return fmt.Errorf("failed to apply (cluster)roles: %w", err)
	}
	if err := manifest.Filter(rolebinding).Apply(); err != nil {
		status.MarkInstallFailed(err.Error())
		return fmt.Errorf("failed to apply (cluster)rolebindings: %w", err)
	}
	if err := manifest.Filter(consoleCLIDownload).Apply(); err != nil {
		status.MarkInstallFailed(err.Error())
		return fmt.Errorf("failed to apply consoleCLIdownload: %w", err)
	}
	if err := manifest.Filter(clusterTriggerBinding).Apply(); err != nil {
		status.MarkInstallFailed(err.Error())
		return fmt.Errorf("failed to apply clusterTriggerBinding: %w", err)
	}
	if err := manifest.Filter(mf.Not(mf.Any(role, rolebinding))).Apply(); err != nil {
		status.MarkInstallFailed(err.Error())
		return fmt.Errorf("failed to apply non rbac manifest: %w", err)
	}
	status.MarkInstallSucceeded()
	status.SetVersion(TargetVersion(instance))
	return nil
}

// Uninstall removes all resources
func Uninstall(ctx context.Context, manifest *mf.Manifest, instance v1alpha1.TektonComponent) error {
	if err := manifest.Filter(mf.Not(mf.Any(role, rolebinding, deployment))).Delete(); err != nil {
		return fmt.Errorf("failed to remove non-crd/non-rbac resources: %w", err)
	}
	// delete deployment separately (after delete call to CRDs)
	// to improve the chance of the finalizers to be handled before the controllers are deleted
	// ref: https://github.com/tektoncd/triggers/issues/775#issuecomment-700271144
	if err := manifest.Filter(mf.Any(deployment)).Delete(); err != nil {
		return fmt.Errorf("failed to remove deployments: %w", err)
	}
	// Delete Roles last, as they may be useful for human operators to clean up.
	if err := manifest.Filter(mf.Any(role, rolebinding)).Delete(); err != nil {
		return fmt.Errorf("failed to remove rbac: %w", err)
	}
	return nil
}
