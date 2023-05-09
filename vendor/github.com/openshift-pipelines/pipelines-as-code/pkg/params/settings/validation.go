package settings

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

func Validate(config map[string]string) error {
	if secretAutoCreation, ok := config[SecretAutoCreateKey]; ok && secretAutoCreation != "" {
		if !isValidBool(secretAutoCreation) {
			return fmt.Errorf("invalid value for key %v, acceptable values: true or false", SecretAutoCreateKey)
		}
	}

	if remoteTask, ok := config[RemoteTasksKey]; ok && remoteTask != "" {
		if !isValidBool(remoteTask) {
			return fmt.Errorf("invalid value for key %v, acceptable values: true or false", RemoteTasksKey)
		}
	}

	if check, ok := config[BitbucketCloudCheckSourceIPKey]; ok && check != "" {
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

	if check, ok := config[AutoConfigureNewGitHubRepoKey]; ok && check != "" {
		if !isValidBool(check) {
			return fmt.Errorf("invalid value for key %v, acceptable values: true or false", AutoConfigureNewGitHubRepoKey)
		}
	}

	if dashboardURL, ok := config[TektonDashboardURLKey]; ok && dashboardURL != "" {
		if _, err := url.ParseRequestURI(dashboardURL); err != nil {
			return fmt.Errorf("invalid value for key %v, invalid url: %w", TektonDashboardURLKey, err)
		}
	}

	if check, ok := config[ErrorDetectionKey]; ok && check != "" {
		if !isValidBool(check) {
			return fmt.Errorf("invalid value for key %v, acceptable values: true or false", ErrorDetectionKey)
		}
	}

	if errorDetectionSimpleRegexp, ok := config[ErrorDetectionSimpleRegexpKey]; ok && config[ErrorDetectionSimpleRegexpKey] != "" {
		if _, err := regexp.Compile(errorDetectionSimpleRegexp); err != nil {
			return fmt.Errorf("cannot use %v as regexp for error detection: %w", config[ErrorDetectionSimpleRegexpKey], err)
		}
	}

	if v, ok := config[CustomConsoleURLKey]; ok && v != "" {
		if _, err := url.ParseRequestURI(v); err != nil {
			return fmt.Errorf("invalid value for key %v, invalid url: %w", CustomConsoleURLKey, err)
		}
	}

	if v, ok := config[CustomConsolePRTaskLogKey]; ok && v != "" {
		// check if custom console start with http:// or https://
		if strings.HasPrefix(v, "http://") || !strings.HasPrefix(v, "https://") {
			return fmt.Errorf("invalid value for key %v, must start with http:// or https://", CustomConsolePRTaskLogKey)
		}
	}

	if v, ok := config[CustomConsolePRDetailKey]; ok && v != "" {
		if strings.HasPrefix(v, "http://") || !strings.HasPrefix(v, "https://") {
			return fmt.Errorf("invalid value for key %v, must start with http:// or https://", CustomConsolePRTaskLogKey)
		}
	}
	return nil
}

func isValidBool(value string) bool {
	return value == "true" || value == "false"
}
