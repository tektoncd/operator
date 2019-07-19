package config

import (
	"time"
)

const (
	// APIRetry defines the frequency at which we check for updates against the
	// k8s api when waiting for a specific condition to be true.
	APIRetry = time.Second * 5

	// APITimeout defines the amount of time we should spend querying the k8s api
	// when waiting for a specific condition to be true.
	APITimeout = time.Minute * 60

	// CleanupRetry is the interval at which test framework attempts cleanup
	CleanupRetry = time.Second * 10

	// CleanupTimeout is the wait time for test framework cleanup
	CleanupTimeout = time.Second * 180

	// TestOperatorName specifies the name of the operator being tested
	TestOperatorName = "openshift-pipelines-operator"
)
