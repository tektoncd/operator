package settings

import (
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

const (
	ApplicationNameKey                    = "application-name"
	SecretAutoCreateKey                   = "secret-auto-create"
	HubURLKey                             = "hub-url"
	HubCatalogNameKey                     = "hub-catalog-name"
	MaxKeepRunUpperLimitKey               = "max-keep-run-upper-limit"
	DefaultMaxKeepRunsKey                 = "default-max-keep-runs"
	RemoteTasksKey                        = "remote-tasks"
	BitbucketCloudCheckSourceIPKey        = "bitbucket-cloud-check-source-ip"
	BitbucketCloudAdditionalSourceIPKey   = "bitbucket-cloud-additional-source-ip"
	TektonDashboardURLKey                 = "tekton-dashboard-url"
	AutoConfigureNewGitHubRepoKey         = "auto-configure-new-github-repo"
	AutoConfigureRepoNamespaceTemplateKey = "auto-configure-repo-namespace-template"

	secretAutoCreateDefaultValue            = "true"
	remoteTasksDefaultValue                 = "true"
	bitbucketCloudCheckSourceIPDefaultValue = "true"
	PACApplicationNameDefaultValue          = "Pipelines as Code CI"
	HubURLDefaultValue                      = "https://api.hub.tekton.dev/v1"
	hubCatalogNameDefaultValue              = "tekton"
	AutoConfigureNewGitHubRepoDefaultValue  = "false"
)

type Settings struct {
	ApplicationName                    string
	SecretAutoCreation                 bool
	HubURL                             string
	HubCatalogName                     string
	RemoteTasks                        bool
	MaxKeepRunsUpperLimit              int
	DefaultMaxKeepRuns                 int
	BitbucketCloudCheckSourceIP        bool
	BitbucketCloudAdditionalSourceIP   string
	TektonDashboardURL                 string
	AutoConfigureNewGitHubRepo         bool
	AutoConfigureRepoNamespaceTemplate string
}

func ConfigToSettings(logger *zap.SugaredLogger, setting *Settings, config map[string]string) error {
	// pass through defaulting
	SetDefaults(config)

	// validate fields
	if err := Validate(config); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	if setting.ApplicationName != config[ApplicationNameKey] {
		logger.Infof("CONFIG: application name updated to %v", config[ApplicationNameKey])
		setting.ApplicationName = config[ApplicationNameKey]
	}
	secretAutoCreate := StringToBool(config[SecretAutoCreateKey])
	if setting.SecretAutoCreation != secretAutoCreate {
		logger.Infof("CONFIG: secret auto create updated to %v", secretAutoCreate)
		setting.SecretAutoCreation = secretAutoCreate
	}
	if setting.HubURL != config[HubURLKey] {
		logger.Infof("CONFIG: hub URL updated to %v", config[HubURLKey])
		setting.HubURL = config[HubURLKey]
	}
	if setting.HubCatalogName != config[HubCatalogNameKey] {
		logger.Infof("CONFIG: hub catalog name updated to %v", config[HubCatalogNameKey])
		setting.HubCatalogName = config[HubCatalogNameKey]
	}
	remoteTask := StringToBool(config[RemoteTasksKey])
	if setting.RemoteTasks != remoteTask {
		logger.Infof("CONFIG: remote tasks setting updated to %v", remoteTask)
		setting.RemoteTasks = remoteTask
	}
	maxKeepRunUpperLimit, _ := strconv.Atoi(config[MaxKeepRunUpperLimitKey])
	if setting.MaxKeepRunsUpperLimit != maxKeepRunUpperLimit {
		logger.Infof("CONFIG: max keep runs upper limit updated to %v", maxKeepRunUpperLimit)
		setting.MaxKeepRunsUpperLimit = maxKeepRunUpperLimit
	}
	defaultMaxKeepRun, _ := strconv.Atoi(config[DefaultMaxKeepRunsKey])
	if setting.DefaultMaxKeepRuns != defaultMaxKeepRun {
		logger.Infof("CONFIG: default keep runs updated to %v", defaultMaxKeepRun)
		setting.DefaultMaxKeepRuns = defaultMaxKeepRun
	}
	check := StringToBool(config[BitbucketCloudCheckSourceIPKey])
	if setting.BitbucketCloudCheckSourceIP != check {
		logger.Infof("CONFIG: bitbucket cloud check source ip setting updated to %v", check)
		setting.BitbucketCloudCheckSourceIP = check
	}
	if setting.BitbucketCloudAdditionalSourceIP != config[BitbucketCloudAdditionalSourceIPKey] {
		logger.Infof("CONFIG: bitbucket cloud additional source ip updated to %v", config[BitbucketCloudAdditionalSourceIPKey])
		setting.BitbucketCloudAdditionalSourceIP = config[BitbucketCloudAdditionalSourceIPKey]
	}
	if setting.TektonDashboardURL != config[TektonDashboardURLKey] {
		logger.Infof("CONFIG: tekton dashboard url updated to %v", config[TektonDashboardURLKey])
		setting.TektonDashboardURL = config[TektonDashboardURLKey]
	}
	autoConfigure := StringToBool(config[AutoConfigureNewGitHubRepoKey])
	if setting.AutoConfigureNewGitHubRepo != autoConfigure {
		logger.Infof("CONFIG: auto configure GitHub repo setting updated to %v", autoConfigure)
		setting.AutoConfigureNewGitHubRepo = autoConfigure
	}
	if setting.AutoConfigureRepoNamespaceTemplate != config[AutoConfigureRepoNamespaceTemplateKey] {
		logger.Infof("CONFIG: auto configure repo namespace template updated to %v", config[AutoConfigureRepoNamespaceTemplateKey])
		setting.AutoConfigureRepoNamespaceTemplate = config[AutoConfigureRepoNamespaceTemplateKey]
	}
	return nil
}

func StringToBool(s string) bool {
	if strings.ToLower(s) == "true" ||
		strings.ToLower(s) == "yes" || s == "1" {
		return true
	}
	return false
}
