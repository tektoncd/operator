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
	"fmt"

	"go.uber.org/zap"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apiextensions/storageversion"
)

// performs crd storage version upgrade
// lists all the resources and,
// keeps only one storage version on the crd
func MigrateStorageVersion(ctx context.Context, logger *zap.SugaredLogger, migrator *storageversion.Migrator, crdGroups []string) error {
	logger.Infof("migrating %d group resources", len(crdGroups))

	for _, crdGroupString := range crdGroups {
		crdGroup := schema.ParseGroupResource(crdGroupString)
		if crdGroup.Empty() {
			logger.Errorf("unable to parse group version: %s", crdGroupString)
			return fmt.Errorf("unable to parse group version: %s", crdGroupString)
		}
		logger.Info("migrating group resource ", crdGroup)
		if err := migrator.Migrate(ctx, crdGroup); err != nil {
			if apierrs.IsNotFound(err) {
				logger.Infow("ignoring resource migration - unable to fetch a crd",
					"crd", crdGroup, err,
				)
				continue
			}
			logger.Errorw("failed to migrate: ", err)
			return err
		}
	}

	return nil
}
