package apiserver

import (
	"fmt"
	"reflect"
	"slices"

	"k8s.io/klog/v2"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/configobserver"
	"github.com/openshift/library-go/pkg/operator/events"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ObserveTLSSecurityProfile observes APIServer.Spec.TLSSecurityProfile field and sets
// the ServingInfo.MinTLSVersion, ServingInfo.CipherSuites fields of observed config
func ObserveTLSSecurityProfile(genericListers configobserver.Listers, recorder events.Recorder, existingConfig map[string]interface{}) (map[string]interface{}, []error) {
	return innerTLSSecurityProfileObservations(genericListers, recorder, existingConfig, []string{"servingInfo", "minTLSVersion"}, []string{"servingInfo", "cipherSuites"}, nil)
}

// ObserveTLSSecurityProfileWithPaths is like ObserveTLSSecurityProfile, but accepts
// custom paths for ServingInfo.MinTLSVersion and ServingInfo.CipherSuites fields of observed config.
func ObserveTLSSecurityProfileWithPaths(genericListers configobserver.Listers, recorder events.Recorder, existingConfig map[string]interface{}, minTLSVersionPath, cipherSuitesPath []string) (map[string]interface{}, []error) {
	return innerTLSSecurityProfileObservations(genericListers, recorder, existingConfig, minTLSVersionPath, cipherSuitesPath, nil)
}

// ObserveTLSSecurityProfileWithGroupPaths is like ObserveTLSSecurityProfileWithPaths but also
// observes the TLS group (curve) preferences at groupsPath. Group names are stored as []string
// matching the TLSGroup constants from openshift/api and can be passed directly to operands
// that accept group names as CLI arguments.
func ObserveTLSSecurityProfileWithGroupPaths(genericListers configobserver.Listers, recorder events.Recorder, existingConfig map[string]interface{}, minTLSVersionPath, cipherSuitesPath, groupsPath []string) (map[string]interface{}, []error) {
	return innerTLSSecurityProfileObservations(genericListers, recorder, existingConfig, minTLSVersionPath, cipherSuitesPath, groupsPath)
}

// ObserveTLSSecurityProfileToArguments observes APIServer.Spec.TLSSecurityProfile field and sets
// the tls-min-version and tls-cipher-suites fileds of observedConfig.apiServerArguments
func ObserveTLSSecurityProfileToArguments(genericListers configobserver.Listers, recorder events.Recorder, existingConfig map[string]interface{}) (map[string]interface{}, []error) {
	return innerTLSSecurityProfileObservations(genericListers, recorder, existingConfig, []string{"apiServerArguments", "tls-min-version"}, []string{"apiServerArguments", "tls-cipher-suites"}, nil)
}

// innerTLSSecurityProfileObservations is the shared implementation for all Observe* functions.
// When groupsPath is non-nil, TLS group preferences are also observed and stored at that path.
func innerTLSSecurityProfileObservations(genericListers configobserver.Listers, recorder events.Recorder, existingConfig map[string]interface{}, minTLSVersionPath, cipherSuitesPath, groupsPath []string) (ret map[string]interface{}, _ []error) {
	defer func() {
		paths := [][]string{minTLSVersionPath, cipherSuitesPath}
		if len(groupsPath) > 0 {
			paths = append(paths, groupsPath)
		}
		ret = configobserver.Pruned(ret, paths...)
	}()

	listers := genericListers.(APIServerLister)
	errs := []error{}

	currentMinTLSVersion, _, versionErr := unstructured.NestedString(existingConfig, minTLSVersionPath...)
	if versionErr != nil {
		errs = append(errs, fmt.Errorf("failed to retrieve spec.servingInfo.minTLSVersion: %v", versionErr))
		// keep going on read error from existing config
	}

	currentCipherSuites, _, suitesErr := unstructured.NestedStringSlice(existingConfig, cipherSuitesPath...)
	if suitesErr != nil {
		errs = append(errs, fmt.Errorf("failed to retrieve spec.servingInfo.cipherSuites: %v", suitesErr))
		// keep going on read error from existing config
	}

	var currentGroups []string
	if len(groupsPath) > 0 {
		var groupsErr error
		currentGroups, _, groupsErr = unstructured.NestedStringSlice(existingConfig, groupsPath...)
		if groupsErr != nil {
			errs = append(errs, fmt.Errorf("failed to retrieve groups: %v", groupsErr))
		}
	}

	apiServer, err := listers.APIServerLister().Get("cluster")
	if errors.IsNotFound(err) {
		klog.Warningf("apiserver.config.openshift.io/cluster: not found")
		apiServer = &configv1.APIServer{}
	} else if err != nil {
		return existingConfig, append(errs, err)
	}

	// Resolve the effective profile spec once and share it between the cipher and
	// group extractors so the two cannot resolve different profiles.
	profileSpec := resolveProfileSpec(apiServer.Spec.TLSSecurityProfile)

	observedConfig := map[string]interface{}{}
	observedMinTLSVersion, observedCipherSuites := getSecurityProfileCiphers(profileSpec)
	if err = unstructured.SetNestedField(observedConfig, observedMinTLSVersion, minTLSVersionPath...); err != nil {
		return existingConfig, append(errs, err)
	}
	if err = unstructured.SetNestedStringSlice(observedConfig, observedCipherSuites, cipherSuitesPath...); err != nil {
		return existingConfig, append(errs, err)
	}

	if observedMinTLSVersion != currentMinTLSVersion {
		recorder.Eventf("ObserveTLSSecurityProfile", "minTLSVersion changed to %s", observedMinTLSVersion)
	}
	if !reflect.DeepEqual(observedCipherSuites, currentCipherSuites) {
		recorder.Eventf("ObserveTLSSecurityProfile", "cipherSuites changed to %q", observedCipherSuites)
	}

	if len(groupsPath) > 0 {
		observedGroups := getSecurityProfileGroups(profileSpec)
		// Store the groups (already FIPS-filtered) verbatim, including an empty list.
		// An empty result (e.g. all groups dropped under FIPS) lets TLS fail rather
		// than negotiating unrequested curves.
		if err = unstructured.SetNestedStringSlice(observedConfig, observedGroups, groupsPath...); err != nil {
			return existingConfig, append(errs, err)
		}
		// slices.Equal treats nil and empty as equal, so a first reconcile
		// (currentGroups nil) against an empty observation is not a change.
		if !slices.Equal(observedGroups, currentGroups) {
			recorder.Eventf("ObserveTLSSecurityProfile", "groups changed to %q", observedGroups)
		}
	}

	return observedConfig, errs
}

// resolveProfileSpec returns the effective TLSProfileSpec for a
// TLSSecurityProfile. Named profiles (Old/Intermediate/Modern) resolve from the
// well-known openshift/api table; a Custom profile uses its inline spec. When
// the profile is nil (e.g. APIServer/cluster not found) or is Custom-typed
// without an actual custom spec, it falls back to the Intermediate default.
//
// It is resolved once per reconcile in innerTLSSecurityProfileObservations and
// shared by the cipher and group extractors so the two cannot drift.
func resolveProfileSpec(profile *configv1.TLSSecurityProfile) *configv1.TLSProfileSpec {
	profileType := crypto.DefaultTLSProfileType
	if profile != nil {
		profileType = profile.Type
	}

	var profileSpec *configv1.TLSProfileSpec
	if profileType == configv1.TLSProfileCustomType {
		if profile.Custom != nil {
			profileSpec = &profile.Custom.TLSProfileSpec
		}
	} else {
		profileSpec = configv1.TLSProfiles[profileType]
	}

	// nothing found / custom type set but no actual custom spec / empty type
	if profileSpec == nil {
		profileSpec = configv1.TLSProfiles[crypto.DefaultTLSProfileType]
	}
	return profileSpec
}

// getSecurityProfileCiphers extracts the minimum TLS version and cipher suites
// from the resolved profile spec, remapping ciphers to the IANA names used by the
// Kube ServingInfo config.
func getSecurityProfileCiphers(profileSpec *configv1.TLSProfileSpec) (string, []string) {
	// need to remap all Ciphers to their respective IANA names used by Go
	return string(profileSpec.MinTLSVersion), crypto.OpenSSLToIANACipherSuites(profileSpec.Ciphers)
}

// getSecurityProfileGroups returns the TLS group preference names for the resolved
// profile spec as []string. The strings match the TLSGroup constants from
// openshift/api and can be passed directly to operands that accept group names as
// CLI arguments.
//
// When the Go runtime is in FIPS mode, non-FIPS-approved groups are dropped
// before conversion so consuming components receive an already-safe list.
// See tls_fips.go for the allowlist and the removal TODO (Go 1.27 without OpenSSL FIPS).
func getSecurityProfileGroups(profileSpec *configv1.TLSProfileSpec) []string {
	src := profileSpec.Groups
	if isFIPSEnabled() {
		src = fipsApprovedTLSGroups(src)
	}
	groups := make([]string, 0, len(src))
	for _, g := range src {
		groups = append(groups, string(g))
	}
	return groups
}
