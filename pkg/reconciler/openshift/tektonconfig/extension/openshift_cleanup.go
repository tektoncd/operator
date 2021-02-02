package extension

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/tektoncd/operator/pkg/reconciler/common"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"knative.dev/pkg/logging"
)

const (
	TP12SeamlessPrefix = "IMAGE_TP12_SEAMLESS_"
)

func AppendCleanupTarget(ctx context.Context, manifest *mf.Manifest, instance v1alpha1.TektonComponent) error {
	manifestPath := filepath.Join(common.ComponentDir(instance), "99-clean-up")
	m, err := common.Fetch(manifestPath)
	if err != nil {
		return err
	}
	*manifest = manifest.Append((m))
	return nil
}

func CleanupTransforms(ctx context.Context, manifest *mf.Manifest, instance v1alpha1.TektonComponent) error {
	images := common.ToLowerCaseKeys(common.ImagesFromEnv(TP12SeamlessPrefix))
	tf := common.JobImages(images)
	return common.Transform(ctx, manifest, instance, tf)
}

func RunCleanup(ctx context.Context, manifest *mf.Manifest, instance v1alpha1.TektonComponent) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Running Cleanup Jobs on OpenShift")
	status := instance.GetStatus()
	if err := manifest.Apply(); err != nil {
		status.MarkInstallFailed(err.Error())
		return fmt.Errorf("failed to apply cleanup job: %w", err)
	}
	return nil
}

func CheckCleanup(ctx context.Context, manifest *mf.Manifest, instance v1alpha1.TektonComponent) error {
	return nil
}
