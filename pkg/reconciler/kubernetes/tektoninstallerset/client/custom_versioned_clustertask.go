/*
Copyright 2022 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    hcompp://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"context"
	"strings"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
)

var (
	versionedClusterTaskInstallerSet = "VersionedClusterTask"
)

// VersionedClusterTaskSet this is an exception case where we create one installer set for one minor version
// not for patch version and we don't remove older installer sets on upgrade, hence keeping it different
// from custom set otherwise code becomes unnecessarily complex to handle this case
func (i *InstallerSetClient) VersionedClusterTaskSet(ctx context.Context, comp v1alpha1.TektonComponent, customName string, manifest *mf.Manifest, filterAndTransform FilterAndTransform) error {
	logger := logging.FromContext(ctx)

	setType := InstallerTypeCustom + "-" + strings.ToLower(versionedClusterTaskInstallerSet)
	versionedClusterTaskLS := v1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.InstallerSetType:       setType,
			v1alpha1.ReleaseMinorVersionKey: getPatchVersionTrimmed(i.releaseVersion),
		},
	}
	versionedClusterTaskLabelSelector, err := common.LabelSelector(versionedClusterTaskLS)
	if err != nil {
		return err
	}
	is, err := i.clientSet.List(ctx, v1.ListOptions{LabelSelector: versionedClusterTaskLabelSelector})
	if err != nil {
		return err
	}

	if len(is.Items) == 0 {
		vctSet, err := i.makeInstallerSet(ctx, comp, manifest, filterAndTransform, "addon-versioned-clustertasks", setType)
		if err != nil {
			return err
		}
		vctSet.Labels[v1alpha1.ReleaseMinorVersionKey] = getPatchVersionTrimmed(i.releaseVersion)
		vctSet.GenerateName = "addon-versioned-clustertasks" + "-" + getPatchVersionTrimmed(i.releaseVersion) + "-"

		_, err = i.clientSet.Create(ctx, vctSet, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	if err := i.statusCheck(logger, setType, is.Items); err != nil {
		return err
	}
	return nil
}

func getPatchVersionTrimmed(version string) string {
	endIndex := strings.LastIndex(version, ".")
	if endIndex != -1 {
		version = version[:endIndex]
	}
	return version
}
