package settings

import (
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

const (
	ApplicationNameKey                    = "application-name"
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

	CustomConsoleNameKey      = "custom-console-name"
	CustomConsoleURLKey       = "custom-console-url"
	CustomConsolePRDetailKey  = "custom-console-url-pr-details"
	CustomConsolePRTaskLogKey = "custom-console-url-pr-tasklog"

	SecretAutoCreateKey                          = "secret-auto-create"
	secretAutoCreateDefaultValue                 = "true"
	SecretGhAppTokenRepoScopedKey                = "secret-github-app-token-scoped" //nolint: gosec
	secretGhAppTokenRepoScopedDefaultValue       = "true"
	SecretGhAppTokenScopedExtraReposKey          = "secret-github-app-scope-extra-repos" //nolint: gosec
	secretGhAppTokenScopedExtraReposDefaultValue = ""                                    //nolint: gosec

	remoteTasksDefaultValue                 = "true"
	bitbucketCloudCheckSourceIPDefaultValue = "true"
	PACApplicationNameDefaultValue          = "Pipelines as Code CI"
	HubURLDefaultValue                      = "https://api.hub.tekton.dev/v1"
	hubCatalogNameDefaultValue              = "tekton"
	AutoConfigureNewGitHubRepoDefaultValue  = "false"

	ErrorLogSnippetKey   = "error-log-snippet"
	errorLogSnippetValue = "true"

	ErrorDetectionKey   = "error-detection-from-container-logs"
	errorDetectionValue = "true"

	ErrorDetectionNumberOfLinesKey   = "error-detection-max-number-of-lines"
	errorDetectionNumberOfLinesValue = 50

	ErrorDetectionSimpleRegexpKey   = "error-detection-simple-regexp"
	errorDetectionSimpleRegexpValue = `^(?P<filename>[^:]*):(?P<line>[0-9]+):(?P<column>[0-9]+):([ ]*)?(?P<error>.*)`
)

var TknBinaryName = `tkn`

type Settings struct {
	ApplicationName                    string
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

	SecretAutoCreation               bool
	SecretGHAppRepoScoped            bool
	SecretGhAppTokenScopedExtraRepos string

	ErrorLogSnippet             bool
	ErrorDetection              bool
	ErrorDetectionNumberOfLines int
	ErrorDetectionSimpleRegexp  string

	CustomConsoleName      string
	CustomConsoleURL       string
	CustomConsolePRdetail  string
	CustomConsolePRTaskLog string
}

func ConfigToSettings(logger *zap.SugaredLogger, setting *Settings, config map[string]string) error {
	// pass through defaulting
	SetDefaults(config)

	// validate fields
	if err := Validate(config); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	if setting.ApplicationName != config[ApplicationNameKey] {
		logger.Infof("CONFIG: application name set to %v", config[ApplicationNameKey])
		setting.ApplicationName = config[ApplicationNameKey]
	}

	secretAutoCreate := StringToBool(config[SecretAutoCreateKey])
	if setting.SecretAutoCreation != secretAutoCreate {
		logger.Infof("CONFIG: secret auto create set to %v", secretAutoCreate)
		setting.SecretAutoCreation = secretAutoCreate
	}

	secretGHAppRepoScoped := StringToBool(config[SecretGhAppTokenRepoScopedKey])
	if setting.SecretGHAppRepoScoped != secretGHAppRepoScoped {
		logger.Infof("CONFIG: not scoping the token generated from gh %v", secretGHAppRepoScoped)
		setting.SecretGHAppRepoScoped = secretGHAppRepoScoped
	}

	secretGHAppScopedExtraRepos := config[SecretGhAppTokenScopedExtraReposKey]
	if setting.SecretGhAppTokenScopedExtraRepos != secretGHAppScopedExtraRepos {
		logger.Infof("CONFIG: adding extra repositories for github app token scope %v", secretGHAppRepoScoped)
		setting.SecretGhAppTokenScopedExtraRepos = secretGHAppScopedExtraRepos
	}

	if setting.HubURL != config[HubURLKey] {
		logger.Infof("CONFIG: hub URL set to %v", config[HubURLKey])
		setting.HubURL = config[HubURLKey]
	}
	if setting.HubCatalogName != config[HubCatalogNameKey] {
		logger.Infof("CONFIG: hub catalog name set to %v", config[HubCatalogNameKey])
		setting.HubCatalogName = config[HubCatalogNameKey]
	}
	remoteTask := StringToBool(config[RemoteTasksKey])
	if setting.RemoteTasks != remoteTask {
		logger.Infof("CONFIG: remote tasks setting set to %v", remoteTask)
		setting.RemoteTasks = remoteTask
	}
	maxKeepRunUpperLimit, _ := strconv.Atoi(config[MaxKeepRunUpperLimitKey])
	if setting.MaxKeepRunsUpperLimit != maxKeepRunUpperLimit {
		logger.Infof("CONFIG: max keep runs upper limit set to %v", maxKeepRunUpperLimit)
		setting.MaxKeepRunsUpperLimit = maxKeepRunUpperLimit
	}
	defaultMaxKeepRun, _ := strconv.Atoi(config[DefaultMaxKeepRunsKey])
	if setting.DefaultMaxKeepRuns != defaultMaxKeepRun {
		logger.Infof("CONFIG: default keep runs set to %v", defaultMaxKeepRun)
		setting.DefaultMaxKeepRuns = defaultMaxKeepRun
	}
	check := StringToBool(config[BitbucketCloudCheckSourceIPKey])
	if setting.BitbucketCloudCheckSourceIP != check {
		logger.Infof("CONFIG: bitbucket cloud check source ip setting set to %v", check)
		setting.BitbucketCloudCheckSourceIP = check
	}
	if setting.BitbucketCloudAdditionalSourceIP != config[BitbucketCloudAdditionalSourceIPKey] {
		logger.Infof("CONFIG: bitbucket cloud additional source ip set to %v", config[BitbucketCloudAdditionalSourceIPKey])
		setting.BitbucketCloudAdditionalSourceIP = config[BitbucketCloudAdditionalSourceIPKey]
	}
	if setting.TektonDashboardURL != config[TektonDashboardURLKey] {
		logger.Infof("CONFIG: tekton dashboard url set to %v", config[TektonDashboardURLKey])
		setting.TektonDashboardURL = config[TektonDashboardURLKey]
	}
	autoConfigure := StringToBool(config[AutoConfigureNewGitHubRepoKey])
	if setting.AutoConfigureNewGitHubRepo != autoConfigure {
		logger.Infof("CONFIG: auto configure GitHub repo setting set to %v", autoConfigure)
		setting.AutoConfigureNewGitHubRepo = autoConfigure
	}
	if setting.AutoConfigureRepoNamespaceTemplate != config[AutoConfigureRepoNamespaceTemplateKey] {
		logger.Infof("CONFIG: auto configure repo namespace template set to %v", config[AutoConfigureRepoNamespaceTemplateKey])
		setting.AutoConfigureRepoNamespaceTemplate = config[AutoConfigureRepoNamespaceTemplateKey]
	}

	errorLogSnippet := StringToBool(config[ErrorLogSnippetKey])
	if setting.ErrorLogSnippet != errorLogSnippet {
		logger.Infof("CONFIG: setting log snippet on error to %v", errorLogSnippet)
		setting.ErrorLogSnippet = errorLogSnippet
	}

	errorDetection := StringToBool(config[ErrorDetectionKey])
	if setting.ErrorDetection != errorDetection {
		logger.Infof("CONFIG: setting error detection to %v", errorDetection)
		setting.ErrorDetection = errorDetection
	}

	errorDetectNumberOfLines, _ := strconv.Atoi(config[ErrorDetectionNumberOfLinesKey])
	if setting.ErrorDetection && setting.ErrorDetectionNumberOfLines != errorDetectNumberOfLines {
		logger.Infof("CONFIG: setting error detection limit of container log to %v", errorDetectNumberOfLines)
		setting.ErrorDetectionNumberOfLines = errorDetectNumberOfLines
	}

	if setting.ErrorDetection && setting.ErrorDetectionSimpleRegexp != strings.TrimSpace(config[ErrorDetectionSimpleRegexpKey]) {
		// replace double backslash with single backslash because kube configmap is giving us things double backslashes
		logger.Infof("CONFIG: setting error detection regexp to %v", strings.TrimSpace(config[ErrorDetectionSimpleRegexpKey]))
		setting.ErrorDetectionSimpleRegexp = strings.TrimSpace(config[ErrorDetectionSimpleRegexpKey])
	}

	if setting.CustomConsoleName != config[CustomConsoleNameKey] {
		logger.Infof("CONFIG: setting custom console name to %v", config[CustomConsoleNameKey])
		setting.CustomConsoleName = config[CustomConsoleNameKey]
	}

	if setting.CustomConsoleURL != config[CustomConsoleURLKey] {
		logger.Infof("CONFIG: setting custom console url to %v", config[CustomConsoleURLKey])
		setting.CustomConsoleURL = config[CustomConsoleURLKey]
	}

	if setting.CustomConsolePRdetail != config[CustomConsolePRDetailKey] {
		logger.Infof("CONFIG: setting custom console pr detail URL to %v", config[CustomConsolePRDetailKey])
		setting.CustomConsolePRdetail = config[CustomConsolePRDetailKey]
	}

	if setting.CustomConsolePRTaskLog != config[CustomConsolePRTaskLogKey] {
		logger.Infof("CONFIG: setting custom console pr task log URL to %v", config[CustomConsolePRTaskLogKey])
		setting.CustomConsolePRTaskLog = config[CustomConsolePRTaskLogKey]
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
