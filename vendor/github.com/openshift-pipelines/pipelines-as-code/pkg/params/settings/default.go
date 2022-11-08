package settings

func SetDefaults(config map[string]string) {
	if appName, ok := config[ApplicationNameKey]; !ok || appName == "" {
		config[ApplicationNameKey] = PACApplicationNameDefaultValue
	}

	if secretAutoCreation, ok := config[SecretAutoCreateKey]; !ok || secretAutoCreation == "" {
		config[SecretAutoCreateKey] = secretAutoCreateDefaultValue
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
}
