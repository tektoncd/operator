package common

import (
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func InjectOperandNameLabelPreserveExisting(operandName string) mf.Transformer {
	preserveExisting := true
	return injectOperandNameLabel(operandName, preserveExisting)
}

func InjectOperandNameLabelOverwriteExisting(operandName string) mf.Transformer {
	preserveExisting := false
	return injectOperandNameLabel(operandName, preserveExisting)
}

func injectOperandNameLabel(operandName string, preserveExisting bool) mf.Transformer {
	l := labels.Set{
		v1alpha1.LabelOperandName: operandName,
	}

	if preserveExisting {
		return InjectLabelPreserveExisting(l)
	}
	return InjectLabelOverwriteExisting(l)
}

func InjectLabelPreserveExisting(newLabels labels.Set, skipChecks ...mf.Predicate) mf.Transformer {
	preserverExisting := true
	return injectLabel(newLabels, preserverExisting, skipChecks...)
}

func InjectLabelOverwriteExisting(newLabels labels.Set, skipChecks ...mf.Predicate) mf.Transformer {
	preserverExisting := false
	return injectLabel(newLabels, preserverExisting, skipChecks...)
}

func injectLabel(newLabels labels.Set, preserverExisting bool, skipChecks ...mf.Predicate) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		for _, skipCheck := range skipChecks {
			if skipCheck(u) {
				return nil
			}
		}
		resourceLabels := u.GetLabels()
		if resourceLabels == nil {
			resourceLabels = map[string]string{}
		}
		for key, val := range newLabels {
			if !replaceAllowed(preserverExisting, resourceLabels, key) {
				continue
			}
			resourceLabels[key] = val
		}
		u.SetLabels(resourceLabels)
		return nil
	}
}

func replaceAllowed(preserveExisting bool, existingLabels map[string]string, key string) bool {
	if !preserveExisting {
		return true
	}
	_, ok := existingLabels[key]

	// if key exists (ok = true) then donot allow replace, hence return false
	// else if key not exists (ok = false) the allow replace, hence return true
	// ie, return !ok
	return !ok
}
