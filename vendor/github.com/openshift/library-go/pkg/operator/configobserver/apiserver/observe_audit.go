package apiserver

import (
	"fmt"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	"github.com/openshift/library-go/pkg/operator/configobserver"
	"github.com/openshift/library-go/pkg/operator/events"
)

// AuditPolicyPathGetterFunc allows the observer to be agnostic of the source of audit profile(s).
// The function returns the path to the audit policy file (associated with the
// given profile) in the static manifest folder.
type AuditPolicyPathGetterFunc func(profile string) (string, error)

// NewAuditObserver returns an ObserveConfigFunc that observes the audit field of the APIServer resource
// and sets the apiServerArguments:audit-policy-file field for the apiserver appropriately.
func NewAuditObserver(pathGetter AuditPolicyPathGetterFunc) configobserver.ObserveConfigFunc {
	var (
		apiServerArgumentsAuditPath = []string{"apiServerArguments", "audit-policy-file"}
	)

	return func(genericListers configobserver.Listers, recorder events.Recorder, existingConfig map[string]interface{}) (observed map[string]interface{}, _ []error) {
		defer func() {
			observed = configobserver.Pruned(observed, apiServerArgumentsAuditPath)
		}()

		errs := []error{}

		// if the function encounters an error it returns existing/current config, which means that
		// some other entity (default config in bindata ) must ensure to default the configuration.
		// otherwise, the apiserver won't have a path to audit policy file and it will fail to start.
		listers := genericListers.(APIServerLister)
		apiServer, err := listers.APIServerLister().Get("cluster")
		if err != nil {
			if k8serrors.IsNotFound(err) {
				klog.Warningf("apiserver.config.openshift.io/cluster: not found")

				return existingConfig, errs
			}

			return existingConfig, append(errs, err)
		}

		desiredProfile := string(apiServer.Spec.Audit.Profile)
		if len(desiredProfile) == 0 {
			// The specified Profile is empty, so let the defaulting layer choose a default for us.
			return map[string]interface{}{}, errs
		}

		desiredAuditPolicyPath, err := pathGetter(desiredProfile)
		if err != nil {
			return existingConfig, append(errs, fmt.Errorf("audit profile is not valid name=%s", desiredProfile))
		}

		currentAuditPolicyPath, err := getCurrentPolicyPath(existingConfig, apiServerArgumentsAuditPath...)
		if err != nil {
			return existingConfig, append(errs, fmt.Errorf("audit profile is not valid name=%s", desiredProfile))
		}
		if desiredAuditPolicyPath == currentAuditPolicyPath {
			return existingConfig, errs
		}

		// we have a change of audit policy here!
		observedConfig := map[string]interface{}{}
		if err := unstructured.SetNestedStringSlice(observedConfig, []string{desiredAuditPolicyPath}, apiServerArgumentsAuditPath...); err != nil {
			return existingConfig, append(errs, fmt.Errorf("failed to set desired audit profile in observed config name=%s", desiredProfile))
		}

		recorder.Eventf("ObserveAPIServerArgumentsAudit", "audit policy has been set to profile=%s", desiredProfile)
		return observedConfig, errs
	}
}

func getCurrentPolicyPath(existing map[string]interface{}, fields ...string) (string, error) {
	current, _, err := unstructured.NestedStringSlice(existing, fields...)
	if err != nil {
		return "", err
	}
	if len(current) == 0 {
		return "", nil
	}

	return current[0], nil
}
