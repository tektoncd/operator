/*
Copyright 2022 The Tekton Authors

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

package tektoninstallerset

import (
	"context"

	versionedClients "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
)

var (
	tisClient versionedClients.TektonInstallerSetInterface
)

func InitTektonInstallerSetClient(ctx context.Context) {
	tisClient = operatorclient.Get(ctx).OperatorV1alpha1().TektonInstallerSets()
}

func getTektonInstallerSetClient() versionedClients.TektonInstallerSetInterface {
	return tisClient
}
