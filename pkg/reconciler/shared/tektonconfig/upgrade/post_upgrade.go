/*
Copyright 2023 The Tekton Authors

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
	"context"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	upgrade "github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/upgrade/helper"
	"go.uber.org/zap"
	apixclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/apiextensions/storageversion"
)

// performs storage versions upgrade
// lists all the resources and keeps only one storage version
func upgradeStorageVersion(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error {
	// resources to be upgraded
	crdGroups := []string{
		"clusterinterceptors.triggers.tekton.dev",
		"clustertasks.tekton.dev",
		"clustertriggerbindings.triggers.tekton.dev",
		"customruns.tekton.dev",
		"eventlisteners.triggers.tekton.dev",
		"extensions.dashboard.tekton.dev",
		"interceptors.triggers.tekton.dev",
		"pipelineruns.tekton.dev",
		"pipelines.tekton.dev",
		"resolutionrequests.resolution.tekton.dev",
		"taskruns.tekton.dev",
		"tasks.tekton.dev",
		"triggerbindings.triggers.tekton.dev",
		"triggers.triggers.tekton.dev",
		"triggertemplates.triggers.tekton.dev",
		"verificationpolicies.tekton.dev",
	}

	if v1alpha1.IsOpenShiftPlatform() {
		crdGroups = append(crdGroups, "repositories.pipelinesascode.tekton.dev")
	}

	migrator := storageversion.NewMigrator(
		dynamic.NewForConfigOrDie(restConfig),
		apixclient.NewForConfigOrDie(restConfig),
	)

	return upgrade.MigrateStorageVersion(ctx, logger, migrator, crdGroups)
}
