package common

import (
	"fmt"
	"os"
	"strings"
)

// namespaceFile is the path to the Kubernetes service account namespace file.
const namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// GetCurrentNamespace reads the namespace from the mounted service account token.
// This is used by the ConfigMapReconciler to scope its watch to the deployment namespace.
func GetCurrentNamespace() (string, error) {
	return readNamespace(namespaceFile)
}

func readNamespace(path string) (string, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("not able to read namespace file %s: %w", path, err)
	}
	return strings.TrimSpace(string(bytes)), nil
}
