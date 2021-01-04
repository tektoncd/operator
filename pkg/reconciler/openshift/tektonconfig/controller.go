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

package tektonconfig

import (
	"context"
	"log"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	k8s_ctrl "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektonconfig"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
)

// NewController initializes the controller and is called by the generated code
// Registers eventhandlers to enqueue events
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	ctrl := k8s_ctrl.NewExtendedController(OpenShiftExtension)(ctx, cmw)
	createCR(ctx)
	return ctrl
}

func createCR(ctx context.Context) {
	c := operatorclient.Get(ctx).OperatorV1alpha1()
	tcCR := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: common.ConfigResourceName,
		},
		Spec: v1alpha1.TektonConfigSpec{
			Profile: common.ProfileAll,
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: "openshift-pipelines",
			},
		},
	}
	if _, err := c.TektonConfigs().Create(context.TODO(), tcCR, metav1.CreateOptions{}); err != nil {
		if !errors.IsAlreadyExists(err) {
			log.Panic("Failed to autocreate TektonConfig with error: ", err)
		}
	}
}
