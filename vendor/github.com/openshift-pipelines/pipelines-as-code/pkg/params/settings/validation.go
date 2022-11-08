package settings

import (
	"fmt"
	"net/url"
	"strconv"
)

func Validate(config map[string]string) error {
	if secretAutoCreation, ok := config[SecretAutoCreateKey]; ok {
		if !isValidBool(secretAutoCreation) {
			return fmt.Errorf("invalid value for key %v, acceptable values: true or false", SecretAutoCreateKey)
		}
	}

	if remoteTask, ok := config[RemoteTasksKey]; ok {
		if !isValidBool(remoteTask) {
			return fmt.Errorf("invalid value for key %v, acceptable values: true or false", RemoteTasksKey)
		}
	}

	if check, ok := config[BitbucketCloudCheckSourceIPKey]; ok {
		if !isValidBool(check) {
			return fmt.Errorf("invalid value for key %v, acceptable values: true or false", BitbucketCloudCheckSourceIPKey)
		}
	}

	if runs, ok := config[MaxKeepRunUpperLimitKey]; ok && runs != "" {
		_, err := strconv.Atoi(runs)
		if err != nil {
			return fmt.Errorf("failed to convert %v value to int: %w", MaxKeepRunUpperLimitKey, err)
		}
	}

	if runs, ok := config[DefaultMaxKeepRunsKey]; ok && runs != "" {
		_, err := strconv.Atoi(runs)
		if err != nil {
			return fmt.Errorf("failed to convert %v value to int: %w", DefaultMaxKeepRunsKey, err)
		}
	}

	if check, ok := config[AutoConfigureNewGitHubRepoKey]; ok {
		if !isValidBool(check) {
			return fmt.Errorf("invalid value for key %v, acceptable values: true or false", AutoConfigureNewGitHubRepoKey)
		}
	}

	if dashboardURL, ok := config[TektonDashboardURLKey]; ok {
		if _, err := url.ParseRequestURI(dashboardURL); err != nil {
			return fmt.Errorf("invalid value for key %v, invalid url: %w", TektonDashboardURLKey, err)
		}
	}
	return nil
}

func isValidBool(value string) bool {
	return value == "true" || value == "false"
}
