package common

import (
	"errors"
	"os"
)

const namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

func GetCurrentNamespace() (string, error) {
	bytes, err := os.ReadFile(namespaceFile)
	if err != nil {
		return "", errors.New("not able to read  namespace file: " + namespaceFile)
	}
	namespace := string(bytes)
	return namespace, nil
}
