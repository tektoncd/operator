package tektonpipeline

import (
	"time"
)

const (
	// APIRetry defines the frequency at which we check for updates against the
	// k8s api when waiting for a specific condition to be true.
	APIRetry = time.Second * 5

	// APITimeout defines the amount of time we should spend querying the k8s api
	// when waiting for a specific condition to be true.
	APITimeout = time.Minute * 5

	// TestOperatorNS specifies the namespace of the operator being tested
	TestOperatorNS = "default"

	// TestOperatorName specifies the name of the operator being tested
	TestOperatorName = "tekton-operator"
)
