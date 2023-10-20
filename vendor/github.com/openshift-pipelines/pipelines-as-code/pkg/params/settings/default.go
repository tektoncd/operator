package settings

import "strconv"

func SetDefaults(config map[string]string) {
	if appName, ok := config[ApplicationNameKey]; !ok || appName == "" {
		config[ApplicationNameKey] = PACApplicationNameDefaultValue
	}

	if secretAutoCreation, ok := config[SecretAutoCreateKey]; !ok || secretAutoCreation == "" {
		config[SecretAutoCreateKey] = secretAutoCreateDefaultValue
	}

	if ghScopedToken, ok := config[SecretGhAppTokenRepoScopedKey]; !ok || ghScopedToken == "" {
		config[SecretGhAppTokenRepoScopedKey] = secretGhAppTokenRepoScopedDefaultValue
	}

	if hubURL, ok := config[HubURLKey]; !ok || hubURL == "" {
		config[HubURLKey] = HubURLDefaultValue
	}

	if hubCatalogName, ok := config[HubCatalogNameKey]; !ok || hubCatalogName == "" {
		config[HubCatalogNameKey] = hubCatalogNameDefaultValue
	}

	if remoteTasks, ok := config[RemoteTasksKey]; !ok || remoteTasks == "" {
		config[RemoteTasksKey] = remoteTasksDefaultValue
	}

	if check, ok := config[BitbucketCloudCheckSourceIPKey]; !ok || check == "" {
		config[BitbucketCloudCheckSourceIPKey] = bitbucketCloudCheckSourceIPDefaultValue
	}

	if autoConfigure, ok := config[AutoConfigureNewGitHubRepoKey]; !ok || autoConfigure == "" {
		config[AutoConfigureNewGitHubRepoKey] = AutoConfigureNewGitHubRepoDefaultValue
	}

	if errorLogSnippet, ok := config[ErrorLogSnippetKey]; !ok || errorLogSnippet == "" {
		config[ErrorLogSnippetKey] = errorLogSnippetValue
	}

	if errorDetection, ok := config[ErrorDetectionKey]; !ok || errorDetection == "" {
		config[ErrorDetectionKey] = errorDetectionValue
	}

	if errorDetectionNumberOfLines, ok := config[ErrorDetectionNumberOfLinesKey]; !ok || errorDetectionNumberOfLines == "" {
		config[ErrorDetectionNumberOfLinesKey] = strconv.Itoa(errorDetectionNumberOfLinesValue)
	}

	if errorDetectionSimpleRegexp, ok := config[ErrorDetectionSimpleRegexpKey]; !ok || errorDetectionSimpleRegexp == "" {
		config[ErrorDetectionSimpleRegexpKey] = errorDetectionSimpleRegexpValue
	}

	if v, ok := config[CustomConsoleNameKey]; !ok || v == "" {
		config[CustomConsoleNameKey] = v
	}
	if v, ok := config[CustomConsoleURLKey]; !ok || v == "" {
		config[CustomConsoleURLKey] = v
	}
	if v, ok := config[CustomConsolePRDetailKey]; !ok || v == "" {
		config[CustomConsolePRDetailKey] = v
	}
	if v, ok := config[CustomConsolePRTaskLogKey]; !ok || v == "" {
		config[CustomConsolePRTaskLogKey] = v
	}
}
