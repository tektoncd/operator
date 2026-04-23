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
	PipelinesConsolePluginImageEnvironmentKey = "IMAGE_PIPELINES_CONSOLE_PLUGIN"
	// pipelines console plugin container name, used to replace the image from the environment
	PipelinesConsolePluginContainerName = "pipelines-console-plugin"
	// TLS configuration environment variables
	TLSMinVersionEnvKey       = "TLS_MIN_VERSION"
	TLSCipherSuitesEnvKey     = "TLS_CIPHER_SUITES"
	TLSCurvePreferencesEnvKey = "TLS_CURVE_PREFERENCES"
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
	// TLS configuration
	tlsMinVersion       string
	tlsCipherSuites     string
	tlsCurvePreferences string
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
		consoleImage, found := os.LookupEnv(PipelinesConsolePluginImageEnvironmentKey)
		if found {
			cpr.pipelinesConsolePluginImage = consoleImage
			cpr.logger.Debugw("pipelines console plugin image found from environment",
				"image", consoleImage,
				"environmentVariable", PipelinesConsolePluginImageEnvironmentKey,
			)
		} else {
			cpr.logger.Warnw("pipelines console plugin image not found from environment, continuing with the default image from the manifest",
				"environmentVariable", PipelinesConsolePluginImageEnvironmentKey,
			)
		}

		// Read TLS configuration from environment
		cpr.tlsMinVersion = os.Getenv(TLSMinVersionEnvKey)
		cpr.tlsCipherSuites = os.Getenv(TLSCipherSuitesEnvKey)
		cpr.tlsCurvePreferences = os.Getenv(TLSCurvePreferencesEnvKey)

		// Apply fail-safe defaults if not provided
		if cpr.tlsMinVersion == "" {
			cpr.tlsMinVersion = "VersionTLS12" // Safe default
			cpr.logger.Warnw("TLS min version not configured, using safe default",
				"default", "TLSv1.2",
				"environmentVariable", TLSMinVersionEnvKey,
			)
		}

		cpr.logger.Debugw("TLS configuration loaded",
			"minVersion", cpr.tlsMinVersion,
			"hasCipherSuites", cpr.tlsCipherSuites != "",
			"hasCurvePreferences", cpr.tlsCurvePreferences != "",
		)
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

// generateNginxConfWithTLS injects TLS directives into nginx configuration
func (cpr *consolePluginReconciler) generateNginxConfWithTLS(baseConf string) string {
	// Build TLS directives
	tlsDirectives := cpr.buildNginxTLSDirectives()

	// If no TLS directives to add, return original
	if tlsDirectives == "" {
		return baseConf
	}

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

// buildNginxTLSDirectives generates nginx TLS directives from environment variables
func (cpr *consolePluginReconciler) buildNginxTLSDirectives() string {
	var directives strings.Builder

	// Convert Go TLS version to nginx ssl_protocols
	if cpr.tlsMinVersion != "" {
		protocols := cpr.convertTLSVersionToNginx(cpr.tlsMinVersion)
		directives.WriteString(fmt.Sprintf("        ssl_protocols %s;\n", protocols))
	}

	// NOTE: Cipher suites are intentionally NOT configured here.
	// TLS 1.3 has secure cipher suites by default, and configuring them
	// requires different nginx directives (ssl_conf_command vs ssl_ciphers).
	// Relying on nginx's secure defaults is simpler and less error-prone.
	if cpr.tlsCipherSuites != "" {
		cpr.logger.Debugw("TLS cipher suites provided but not applied (using nginx defaults)",
			"reason", "TLS 1.3 uses secure defaults, avoids ssl_ciphers/ssl_conf_command complexity",
		)
	}

	// Add ECDH curves if provided
	if cpr.tlsCurvePreferences != "" {
		// Convert comma-separated to colon-separated
		curves := strings.ReplaceAll(cpr.tlsCurvePreferences, ",", ":")
		directives.WriteString(fmt.Sprintf("        ssl_ecdh_curve %s;\n", curves))
	}

	return directives.String()
}

// convertTLSVersionToNginx converts Go TLS version names to nginx protocol names
func (cpr *consolePluginReconciler) convertTLSVersionToNginx(minVersion string) string {
	// Map Go TLS constants to nginx protocols
	switch minVersion {
	case "VersionTLS13":
		return "TLSv1.3"
	case "VersionTLS12":
		return "TLSv1.2 TLSv1.3"
	case "VersionTLS11":
		return "TLSv1.1 TLSv1.2 TLSv1.3"
	case "VersionTLS10":
		return "TLSv1 TLSv1.1 TLSv1.2 TLSv1.3"
	default:
		// Fail-safe: use modern defaults
		cpr.logger.Warnw("Unknown TLS version, using safe default",
			"providedVersion", minVersion,
			"defaultTo", "TLSv1.2 TLSv1.3",
		)
		return "TLSv1.2 TLSv1.3"
	}
}
