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
	"time"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	"go.uber.org/zap"
	"knative.dev/pkg/injection"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/openshift/tektonconfig/extension"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
)

const (
	// RetryInterval specifies the time between two polls.
	RetryInterval = 10 * time.Second

	// RetryTimeout specifies the timeout for the function PollImmediate to
	// reach a certain status.
	RetryTimeout = 5 * time.Minute

	// DefaultCRName specifies the default targetnamespaceto be used
	// in autocreated TektonConfig instance
	DefaultCRName = "config"

	// DefaultTargetNamespace specifies the default targetnamespaceto be used
	// in autocreated TektonConfig instance
	DefaultTargetNamespace = "openshift-pipelines"
)

// NoPlatform "generates" a NilExtension
func OpenShiftExtension(ctx context.Context) common.Extension {

	logger := logging.FromContext(ctx)
	mfclient, err := mfc.NewClient(injection.GetConfig(ctx))
	if err != nil {
		logger.Fatalw("error creating client from injected config", zap.Error(err))
	}
	mflogger := zapr.NewLogger(logger.Named("manifestival").Desugar())
	manifest, err := mf.ManifestFrom(mf.Slice{}, mf.UseClient(mfclient), mf.UseLogger(mflogger))
	if err != nil {
		logger.Fatalw("error creating initial manifest", zap.Error(err))
	}
	ext := openshiftExtension{
		operatorClientSet: operatorclient.Get(ctx),
		kubeClientSet:     kubeclient.Get(ctx),
		manifest:          manifest,
	}
	// try to ensure that there is an instance of tektonConfig
	ext.ensureTektonConfigInstance(ctx)
	return ext
}

type openshiftExtension struct {
	operatorClientSet versioned.Interface
	kubeClientSet     kubernetes.Interface
	manifest          mf.Manifest
}

func (oe openshiftExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	return []mf.Transformer{}
}
func (oe openshiftExtension) PreReconcile(context.Context, v1alpha1.TektonComponent) error {
	return nil
}
func (oe openshiftExtension) PostReconcile(ctx context.Context, comp v1alpha1.TektonComponent) error {
	configInstance := comp.(*v1alpha1.TektonConfig)
	if configInstance.Spec.Profile == common.ProfileAll {
		if err := extension.CreateAddonCR(comp, oe.operatorClientSet.OperatorV1alpha1()); err != nil {
			return err
		}
	}

	//TODO: Remove this cleanup after 1.4 GA release
	// cleanup orphaned `pipeline-anyuid` rolebindings and clusterrole
	if err := extension.RbacCleanup(ctx, oe.kubeClientSet); err != nil {
		return err
	}
	return nil
}
func (oe openshiftExtension) Finalize(ctx context.Context, comp v1alpha1.TektonComponent) error {
	configInstance := comp.(*v1alpha1.TektonConfig)
	if configInstance.Spec.Profile == common.ProfileAll {
		return extension.TektonAddonCRDelete(oe.operatorClientSet.OperatorV1alpha1().TektonAddons(), common.AddonResourceName)
	}
	return nil
}

// try to ensure an instance of TektonConfig exists
// if there is an error log error,and continue (an instance of TektonConfig will
// then need to be created by the user to get OpenShift-Pipelines components installed
func (oe openshiftExtension) ensureTektonConfigInstance(ctx context.Context) {
	logger := logging.FromContext(ctx)
	logger.Debug("ensuring tektonconfig instance")

	waitErr := wait.PollImmediate(RetryInterval, RetryTimeout, func() (bool, error) {
		//note: the code in this block will be retired until
		// an error is returned, or
		// 'true' is returned, or
		// timeout
		instance, err := oe.operatorClientSet.
			OperatorV1alpha1().
			TektonConfigs().Get(context.TODO(), DefaultCRName, metav1.GetOptions{})
		if err == nil {
			if !instance.GetDeletionTimestamp().IsZero() {
				// log deleting timestamp error and retry
				logger.Errorf("deletionTimestamp is set on existing Tektonconfig instance, Name: %w", instance.GetName())
				return false, nil
			}
			return true, nil
		}
		if !apierrs.IsNotFound(err) {
			//log error and retry
			logger.Errorf("error getting Tektonconfig, Name: ", instance.GetName())
			return false, nil
		}
		err = oe.createTektonConfigInstance()
		if err != nil {
			//log error and retry
			logger.Errorf("error creating Tektonconfig instance, Name: ", instance.GetName())
			return false, nil
		}
		// even if there is no error after create,
		// loop again to ensure the create is successful with a 'get; api call
		return false, nil
	})
	if waitErr != nil {
		// log error and continue
		logger.Error("error ensuring instance of tektonconfig, check retry logs above for more details, %w", waitErr)
		logger.Info("an instance of TektonConfig need to be created by the user to get OpenShift-Pipelines components installed")
	}
}

func (oe openshiftExtension) createTektonConfigInstance() error {
	tcCR := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: common.ConfigResourceName,
		},
		Spec: v1alpha1.TektonConfigSpec{
			Profile: common.ProfileAll,
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: DefaultTargetNamespace,
			},
		},
	}
	_, err := oe.operatorClientSet.OperatorV1alpha1().
		TektonConfigs().Create(context.TODO(), tcCR, metav1.CreateOptions{})
	return err
}
