/*
Copyright 2024 The Tekton Authors

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
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/go-logr/zapr"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	// manifests console plugin yaml directory location
	consolePluginReconcileYamlDirectory = "static/tekton-config/00-console-plugin"
	// installerSet label value
	consolePluginReconcileLabelCreatedByValue = "tekton-config-console-plugin-manifests"
	// pipelines console plugin environment variable key
	PipelinesConsolePluginImageEnvironmentKey       = "IMAGE_PIPELINES_CONSOLE_PLUGIN"
	PipelinesConsolePluginImageEnvironmentKeyLegacy = "IMAGE_PIPELINES_CONSOLE_PLUGIN_LEGACY"
	// pipelines console plugin container name, used to replace the image from the environment
	PipelinesConsolePluginContainerName = "pipelines-console-plugin"
)

var (
	// label filter to set/get installerSet specific to this reconciler
	consolePluginReconcileInstallerSetLabel = metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.InstallerSetType: v1alpha1.ConfigResourceName,
			v1alpha1.CreatedByKey:     consolePluginReconcileLabelCreatedByValue,
		},
	}
)

type consolePluginReconciler struct {
	logger                      *zap.SugaredLogger
	operatorClientSet           versioned.Interface
	syncOnce                    sync.Once
	resourcesYamlDirectory      string
	operatorVersion             string
	pipelinesConsolePluginImage string
	manifest                    mf.Manifest
	// tlsConfig holds the centrally resolved TLS profile (set on every reconcile).
	// nil means central TLS is disabled; the nginx.conf is left unmodified.
	tlsConfig *occommon.TLSEnvVars
}

// SetTLSConfig stores the resolved central TLS configuration for use during the
// next reconcile cycle. Call this before reconcile() on every reconcile loop.
func (cpr *consolePluginReconciler) SetTLSConfig(tlsEnvVars *occommon.TLSEnvVars) {
	cpr.tlsConfig = tlsEnvVars
}

// reconcile steps
// 1. get console plugin manifests from kodata
// 2. verify the existing installerSet hash value
// 3. if there is a mismatch or the installerSet not available, (re)create it
func (cpr *consolePluginReconciler) reconcile(ctx context.Context, tektonConfigCR *v1alpha1.TektonConfig) error {

	cpr.updateOnce(ctx)

	// verify he availability of the installerSet
	labelSelector, err := common.LabelSelector(consolePluginReconcileInstallerSetLabel)
	if err != nil {
		return err
	}

	installerSetList, err := cpr.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return err
	}

	doCreateInstallerSet := false
	var deployedInstallerSet v1alpha1.TektonInstallerSet

	if len(installerSetList.Items) > 1 {
		for _, installerSet := range installerSetList.Items {
			err = cpr.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().Delete(ctx, installerSet.GetName(), metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
		doCreateInstallerSet = true
	} else if len(installerSetList.Items) == 1 {
		deployedInstallerSet = installerSetList.Items[0]
	} else {
		doCreateInstallerSet = true
	}

	// clone the manifest
	manifest := cpr.manifest.Append()
	// apply transformations
	if err := cpr.transform(ctx, &manifest, tektonConfigCR); err != nil {
		tektonConfigCR.Status.MarkNotReady(fmt.Sprintf("transformation failed: %s", err.Error()))
		return err
	}

	// get expected hash value of the manifests
	expectedHash, err := cpr.getHash(manifest.Resources())
	if err != nil {
		return err
	}

	if !doCreateInstallerSet {
		// compute hash from the deployed installerSet
		deployedHash, err := cpr.getHash(deployedInstallerSet.Spec.Manifests)
		if err != nil {
			return err
		}

		releaseVersion := deployedInstallerSet.GetLabels()[v1alpha1.ReleaseVersionKey]
		// delete the existing installerSet,
		// if hash mismatch or version mismatch
		if expectedHash != deployedHash || cpr.operatorVersion != releaseVersion {
			if err := cpr.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().Delete(ctx, deployedInstallerSet.GetName(), metav1.DeleteOptions{}); err != nil {
				return err
			}
			doCreateInstallerSet = true
		}
	}

	if doCreateInstallerSet {
		return cpr.createInstallerSet(ctx, &manifest, tektonConfigCR)
	}

	return nil
}

func (cpr *consolePluginReconciler) updateOnce(ctx context.Context) {
	// reads all yaml files from the directory, it is an expensive process to access disk on each reconcile call.
	// hence fetch only once at startup, it helps not to degrade the performance of the reconcile loop
	// also it not necessary to read the files frequently, as the files are shipped along the container and never change
	cpr.syncOnce.Do(func() {
		// fetch manifest from disk
		manifest, err := mf.NewManifest(cpr.resourcesYamlDirectory, mf.UseLogger(zapr.NewLogger(cpr.logger.Desugar())))
		if err != nil {
			cpr.logger.Fatal("error getting manifests",
				"manifestsLocation", cpr.resourcesYamlDirectory,
				err,
			)
		}
		cpr.manifest = manifest

		// update pipelines console image details

		// Below logic is to pick Console Plugin Image based on the OCP Version.
		// OCP versions older than 4.22 uses the legacy Console Plugin.
		var envKey string
		ocpVersion, err := occommon.GetOCPVersion(ctx)
		if err != nil {
			cpr.logger.Errorf("error getting OCP version: %q", err)
		} else if ocpVersion.Major() == 4 && ocpVersion.Minor() < 22 {
			cpr.logger.Infof("Using Legacy Console Plugin on OCP : %v", ocpVersion)
			envKey = PipelinesConsolePluginImageEnvironmentKeyLegacy
		} else {
			envKey = PipelinesConsolePluginImageEnvironmentKey
		}

		consoleImage, found := os.LookupEnv(envKey)
		if found {
			cpr.pipelinesConsolePluginImage = consoleImage
			cpr.logger.Infow("pipelines console plugin image found from environment",
				"image", consoleImage,
				"environmentVariable", envKey,
			)
		} else {
			cpr.logger.Warnw("pipelines console plugin image not found from environment, continuing with the default image from the manifest",
				"environmentVariable", envKey,
			)
		}

	})
}

func (cpr *consolePluginReconciler) createInstallerSet(ctx context.Context, manifest *mf.Manifest, tektonConfigCR *v1alpha1.TektonConfig) error {
	// setup installerSet
	ownerRef := *metav1.NewControllerRef(tektonConfigCR, tektonConfigCR.GetGroupVersionKind())
	installerSet := &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "tekton-config-console-plugin-manifests-",
			Labels:       consolePluginReconcileInstallerSetLabel.MatchLabels,
			Annotations: map[string]string{
				v1alpha1.TargetNamespaceKey: tektonConfigCR.Spec.TargetNamespace,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}
	// update operator version
	installerSet.Labels[v1alpha1.ReleaseVersionKey] = cpr.operatorVersion

	// creates installerSet in the cluster
	_, err := cpr.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().Create(ctx, installerSet, metav1.CreateOptions{})
	if err != nil {
		cpr.logger.Error("error on creating installerset", err)
	}
	return err
}

// apply transformations
func (cpr *consolePluginReconciler) transform(ctx context.Context, manifest *mf.Manifest, tektonConfigCR *v1alpha1.TektonConfig) error {
	// load required transformers
	transformers := []mf.Transformer{
		// updates "metadata.namespace" to targetNamespace
		common.ReplaceNamespace(tektonConfigCR.Spec.TargetNamespace),
		cpr.transformerConsolePlugin(tektonConfigCR.Spec.TargetNamespace),
		// Add nginx TLS configuration transformer
		cpr.transformerNginxTLS(),
		common.AddConfiguration(tektonConfigCR.Spec.Config),
	}

	if cpr.pipelinesConsolePluginImage != "" {
		// updates deployments container image
		transformers = append(transformers, common.DeploymentImages(map[string]string{
			// on the transformer, in the container name, the '-' replaced with '_'
			strings.ReplaceAll(PipelinesConsolePluginContainerName, "-", "_"): cpr.pipelinesConsolePluginImage,
		}))
	}

	// perform transformation
	return common.Transform(ctx, manifest, tektonConfigCR, transformers...)
}

func (cpr *consolePluginReconciler) getHash(resources []unstructured.Unstructured) (string, error) {
	return hash.Compute(resources)
}

func (cpr *consolePluginReconciler) transformerConsolePlugin(targetNamespace string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "ConsolePlugin" {
			return nil
		}

		return unstructured.SetNestedField(u.Object, targetNamespace, "spec", "backend", "service", "namespace")
	}
}

// transformerNginxTLS updates the nginx.conf ConfigMap with TLS directives
func (cpr *consolePluginReconciler) transformerNginxTLS() mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "ConfigMap" || u.GetName() != "pipelines-console-plugin" {
			return nil
		}

		// Get the current nginx.conf
		data, found, err := unstructured.NestedString(u.Object, "data", "nginx.conf")
		if err != nil || !found {
			return err
		}

		// Generate the updated nginx.conf with TLS directives
		updatedConf := cpr.generateNginxConfWithTLS(data)

		// Set the updated nginx.conf back
		return unstructured.SetNestedField(u.Object, updatedConf, "data", "nginx.conf")
	}
}

// generateNginxConfWithTLS injects TLS directives into nginx configuration.
// Directives are always produced (at minimum ssl_protocols) so this function
// never returns the unmodified base configuration.
func (cpr *consolePluginReconciler) generateNginxConfWithTLS(baseConf string) string {
	tlsDirectives := cpr.buildNginxTLSDirectives()

	// Inject TLS directives into the server block
	// Find "server {" and inject after it
	lines := strings.Split(baseConf, "\n")
	var result strings.Builder

	for _, line := range lines {
		result.WriteString(line)
		result.WriteString("\n")

		// After "server {", inject TLS directives
		if strings.Contains(line, "server {") {
			// Add TLS directives with proper indentation
			result.WriteString(tlsDirectives)
		}
	}

	return result.String()
}

// defaultTLS13Ciphersuites is the set of TLS 1.3 ciphersuites used in the
// ssl_conf_command Ciphersuites directive when no explicit cluster TLS profile
// is present.  TLS_CHACHA20_POLY1305_SHA256 is intentionally omitted:
// OpenSSL rejects the entire Ciphersuites directive when it contains a cipher
// that its active provider does not support (e.g. the FIPS provider), which
// disables TLS 1.3 on the nginx listener entirely.  TLS_AES_128_CCM_SHA256
// (nginx's built-in default) is also excluded as it is not part of the
// OpenShift Intermediate/Default profile.
const defaultTLS13Ciphersuites = "TLS_AES_256_GCM_SHA384:TLS_AES_128_GCM_SHA256"

// buildNginxTLSDirectives generates nginx TLS directives from the centrally resolved
// TLS profile. When no explicit profile is configured (cluster uses the "Default"
// profile), secure Intermediate-equivalent defaults are applied so that nginx never
// falls back to its built-in cipher and protocol set.
func (cpr *consolePluginReconciler) buildNginxTLSDirectives() string {
	var directives strings.Builder

	// ssl_protocols – derived from the minimum TLS version in the APIServer profile.
	// Fall back to "1.2" (Intermediate) when no central TLS config is present, which
	// is the OpenShift default for clusters without an explicit tlsSecurityProfile.
	minVersion := "1.2"
	if cpr.tlsConfig != nil && cpr.tlsConfig.MinVersion != "" {
		minVersion = cpr.tlsConfig.MinVersion
	}
	protocols := convertTLSVersionToNginx(minVersion)
	directives.WriteString(fmt.Sprintf("    ssl_protocols %s;\n", protocols))

	// ssl_ciphers – translate IANA cipher names from the cluster profile to the
	// OpenSSL names required by nginx. Only TLS 1.2 ciphers are emitted here;
	// TLS 1.3 ciphersuites are controlled separately via ssl_conf_command Ciphersuites.
	if cpr.tlsConfig != nil && cpr.tlsConfig.CipherSuites != "" {
		opensslCiphers := ianaToOpenSSLCiphers(cpr.tlsConfig.CipherSuites)
		if opensslCiphers != "" {
			directives.WriteString(fmt.Sprintf("    ssl_ciphers %s;\n", opensslCiphers))
			directives.WriteString("    ssl_prefer_server_ciphers on;\n")
		}
	}

	// ssl_conf_command Ciphersuites – explicitly restrict TLS 1.3 ciphersuites to
	// those allowed by the cluster profile. nginx's built-in TLS 1.3 defaults
	// include TLS_AES_128_CCM_SHA256 which is NOT part of OpenShift's
	// Intermediate/Default profile, so we must enumerate the allowed set explicitly.
	// ssl_ciphers only controls TLS 1.2; TLS 1.3 suites require this separate directive.
	tls13Ciphers := defaultTLS13Ciphersuites
	if cpr.tlsConfig != nil && cpr.tlsConfig.CipherSuites != "" {
		if extracted := ianaTLS13Ciphersuites(cpr.tlsConfig.CipherSuites); extracted != "" {
			tls13Ciphers = extracted
		}
	}
	directives.WriteString(fmt.Sprintf("    ssl_conf_command Ciphersuites %s;\n", tls13Ciphers))

	// ssl_ecdh_curve – advertise TLS key-exchange groups for the nginx listener.
	//
	// The OpenShift TLS FAQ mandates that every TLS 1.3 server negotiate ML-KEM
	// if the client supports it (quantum-safe key encapsulation is a mandatory
	// requirement).  nginx/OpenSSL requires an explicit ssl_ecdh_curve directive
	// to advertise post-quantum groups; without it only classical curves are
	// offered and the TLS scanner reports pqc_capable=false.
	//
	// Group list is currently hardcoded because library-go's
	// ObserveTLSSecurityProfile does not yet expose the groups/curve-preferences
	// field from the APIServer TLS profile (openshift/library-go#2347, open).
	// Once that lands we will switch to dynamic propagation from the APIServer
	// profile and remove this function.
	//
	// X25519MLKEM768 is not FIPS-approved: OpenSSL's FIPS provider rejects it
	// with a fatal error, crashing nginx.  We therefore exclude it on FIPS nodes
	// while keeping it for all other clusters to satisfy the mandatory ML-KEM
	// requirement.
	tlsGroups := tlsECDHGroups()
	directives.WriteString(fmt.Sprintf("    ssl_ecdh_curve %s;\n", tlsGroups))

	return directives.String()
}

// tlsECDHGroups returns the colon-separated list of TLS key-exchange groups to
// emit in the nginx ssl_ecdh_curve directive.
//
// On FIPS-enabled nodes X25519MLKEM768 is excluded because it is not
// FIPS-approved and causes OpenSSL to crash nginx with:
//
//	nginx: [emerg] SSL_CONF_cmd("Groups","X25519MLKEM768:…") failed
//
// On all other nodes X25519MLKEM768 is included first so that TLS 1.3 clients
// that support ML-KEM perform a post-quantum key exchange (pqc_capable=true).
func tlsECDHGroups() string {
	if isFIPSEnabled() {
		// On FIPS nodes only NIST-approved curves are permitted by OpenSSL's
		// FIPS provider. Both X25519MLKEM768 and X25519 are rejected; only the
		// NIST prime curves (P-256, P-384, P-521) are FIPS 140-approved.
		// Matches the curve list used by cluster-ingress-operator on FIPS.
		return "P-256:P-384:P-521"
	}
	// Matches cluster-ingress-operator's non-FIPS default.
	return "X25519MLKEM768:X25519:P-256:P-384:P-521"
}

// fipsEnabledPath is the kernel file that reports FIPS 140 mode status.
// Overridable in tests via a temp file.
var fipsEnabledPath = "/proc/sys/crypto/fips_enabled"

// isFIPSEnabled reports whether the host kernel has FIPS 140 mode active.
// It reads /proc/sys/crypto/fips_enabled which is provided by the Linux kernel
// and is accessible from inside containers (containers share the host kernel).
// The value is "1" when FIPS mode is on, "0" otherwise.
func isFIPSEnabled() bool {
	data, err := os.ReadFile(fipsEnabledPath)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == "1"
}

// convertTLSVersionToNginx converts the Go crypto/tls minimum version string
// ("1.2" or "1.3", as stored in TLSEnvVars.MinVersion) to the corresponding
// nginx ssl_protocols value.
func convertTLSVersionToNginx(minVersion string) string {
	switch minVersion {
	case "1.3":
		return "TLSv1.3"
	case "1.2":
		return "TLSv1.2 TLSv1.3"
	case "1.1":
		return "TLSv1.1 TLSv1.2 TLSv1.3"
	case "1.0":
		return "TLSv1 TLSv1.1 TLSv1.2 TLSv1.3"
	default:
		return "TLSv1.2 TLSv1.3"
	}
}

// ianaToOpenSSLCiphers translates a comma-separated list of IANA TLS cipher suite
// names to the colon-separated OpenSSL names required by nginx's ssl_ciphers
// directive.
//
// The mapping is derived by inverting the openSSLToIANACiphersMap defined in
// vendor/github.com/openshift/library-go/pkg/crypto/crypto.go, which is the
// canonical source of truth for OpenShift TLS profile cipher names.
//
// TLS 1.3 ciphers (TLS_AES_* / TLS_CHACHA20_*) are omitted here because they
// must not appear in ssl_ciphers; they are handled separately via
// ssl_conf_command Ciphersuites in buildNginxTLSDirectives.
func ianaToOpenSSLCiphers(ianaCiphers string) string {
	// Inverted from library-go's openSSLToIANACiphersMap (unexported).
	// Keep in sync with:
	// vendor/github.com/openshift/library-go/pkg/crypto/crypto.go
	ianaToOpenSSL := map[string]string{
		// TLS 1.2 — explicit nginx ssl_ciphers configuration required.
		"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256":       "ECDHE-ECDSA-AES128-GCM-SHA256",
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256":         "ECDHE-RSA-AES128-GCM-SHA256",
		"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384":       "ECDHE-ECDSA-AES256-GCM-SHA384",
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384":         "ECDHE-RSA-AES256-GCM-SHA384",
		"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256": "ECDHE-ECDSA-CHACHA20-POLY1305",
		"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256":   "ECDHE-RSA-CHACHA20-POLY1305",
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256":       "ECDHE-ECDSA-AES128-SHA256",
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256":         "ECDHE-RSA-AES128-SHA256",
		"TLS_RSA_WITH_AES_128_GCM_SHA256":               "AES128-GCM-SHA256",
		"TLS_RSA_WITH_AES_256_GCM_SHA384":               "AES256-GCM-SHA384",
		"TLS_RSA_WITH_AES_128_CBC_SHA256":               "AES128-SHA256",
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA":          "ECDHE-ECDSA-AES128-SHA",
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA":            "ECDHE-RSA-AES128-SHA",
		"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA":          "ECDHE-ECDSA-AES256-SHA",
		"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA":            "ECDHE-RSA-AES256-SHA",
		"TLS_RSA_WITH_AES_128_CBC_SHA":                  "AES128-SHA",
		"TLS_RSA_WITH_AES_256_CBC_SHA":                  "AES256-SHA",
		"TLS_RSA_WITH_3DES_EDE_CBC_SHA":                 "DES-CBC3-SHA",
		"TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA":           "ECDHE-RSA-DES-CBC3-SHA",
		// TLS 1.3 — handled via ssl_conf_command Ciphersuites, not ssl_ciphers.
		"TLS_AES_128_GCM_SHA256":       "",
		"TLS_AES_256_GCM_SHA384":       "",
		"TLS_CHACHA20_POLY1305_SHA256": "",
	}

	var opensslNames []string
	for _, iana := range strings.Split(ianaCiphers, ",") {
		iana = strings.TrimSpace(iana)
		if iana == "" {
			continue
		}
		openssl, known := ianaToOpenSSL[iana]
		if !known {
			// Unknown cipher — pass through unchanged; nginx will surface any error.
			opensslNames = append(opensslNames, iana)
			continue
		}
		if openssl != "" {
			opensslNames = append(opensslNames, openssl)
		}
		// empty string → TLS 1.3 cipher, handled by ianaTLS13Ciphersuites.
	}
	return strings.Join(opensslNames, ":")
}

// ianaTLS13Ciphersuites extracts TLS 1.3 AES-GCM ciphersuite names from a
// comma-separated IANA cipher list and returns them colon-separated for
// nginx's ssl_conf_command Ciphersuites directive.
//
// Only TLS_AES_* ciphers are included.  TLS_CHACHA20_POLY1305_SHA256 is
// intentionally excluded: OpenSSL rejects the entire ssl_conf_command
// Ciphersuites directive when it contains a cipher that its active provider
// does not support, which would silently disable TLS 1.3 on the nginx
// listener.  AES-GCM suites are universally supported and sufficient.
func ianaTLS13Ciphersuites(ianaCiphers string) string {
	var tls13 []string
	for _, cipher := range strings.Split(ianaCiphers, ",") {
		cipher = strings.TrimSpace(cipher)
		if strings.HasPrefix(cipher, "TLS_AES_") {
			tls13 = append(tls13, cipher)
		}
	}
	return strings.Join(tls13, ":")
}
