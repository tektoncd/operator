/*
Copyright 2023 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" B]>SIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tektonresult

import (
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
)

const (
	statefulSetDB     = "tekton-results-postgres"
	servicePostgresDB = "tekton-results-postgres-service"
)

func (r *Reconciler) filterExternalDB(tr *v1alpha1.TektonResult) {
	if tr.Spec.IsExternalDB {
		r.manifest = r.manifest.Filter(mf.Not(mf.All(mf.ByKind("StatefulSet"), mf.ByName(statefulSetDB))))
		r.manifest = r.manifest.Filter(mf.Not(mf.All(mf.ByKind("ConfigMap"), mf.ByName(configPostgresDB))))
		r.manifest = r.manifest.Filter(mf.Not(mf.All(mf.ByKind("Service"), mf.ByName(servicePostgresDB))))
	}
}
