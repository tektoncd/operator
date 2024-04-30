package settings

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/configutil"
	"go.uber.org/zap"
)

const (
	PACApplicationNameDefaultValue = "Pipelines as Code CI"

	HubURLKey                  = "hub-url"
	HubCatalogNameKey          = "hub-catalog-name"
	HubURLDefaultValue         = "https://api.hub.tekton.dev/v1"
	HubCatalogNameDefaultValue = "tekton"

	CustomConsoleNameKey         = "custom-console-name"
	CustomConsoleURLKey          = "custom-console-url"
	CustomConsolePRDetailKey     = "custom-console-url-pr-details"
	CustomConsolePRTaskLogKey    = "custom-console-url-pr-tasklog"
	CustomConsoleNamespaceURLKey = "custom-console-url-namespace"

	SecretGhAppTokenRepoScopedKey = "secret-github-app-token-scoped" //nolint: gosec
)

var (
	TknBinaryName       = `tkn`
	TknBinaryURL        = `https://tekton.dev/docs/cli/#installation`
	hubCatalogNameRegex = regexp.MustCompile(`^catalog-(\d+)-`)
)

type HubCatalog struct {
	ID   string
	Name string
	URL  string
}

type Settings struct {
	ApplicationName                    string `default:"Pipelines as Code CI" json:"application-name"`
	HubCatalogs                        *sync.Map
	RemoteTasks                        bool   `default:"true"                                json:"remote-tasks"`
	MaxKeepRunsUpperLimit              int    `json:"max-keep-run-upper-limit"`
	DefaultMaxKeepRuns                 int    `json:"default-max-keep-runs"`
	BitbucketCloudCheckSourceIP        bool   `default:"true"                                json:"bitbucket-cloud-check-source-ip"`
	BitbucketCloudAdditionalSourceIP   string `json:"bitbucket-cloud-additional-source-ip"`
	TektonDashboardURL                 string `json:"tekton-dashboard-url"`
	AutoConfigureNewGitHubRepo         bool   `default:"false"                               json:"auto-configure-new-github-repo"`
	AutoConfigureRepoNamespaceTemplate string `json:"auto-configure-repo-namespace-template"`

	SecretAutoCreation               bool   `default:"true"                             json:"secret-auto-create"`
	SecretGHAppRepoScoped            bool   `default:"true"                             json:"secret-github-app-token-scoped"`
	SecretGhAppTokenScopedExtraRepos string `json:"secret-github-app-scope-extra-repos"`

	ErrorLogSnippet             bool   `default:"true"                                                                          json:"error-log-snippet"`
	ErrorDetection              bool   `default:"true"                                                                          json:"error-detection-from-container-logs"`
	ErrorDetectionNumberOfLines int    `default:"50"                                                                            json:"error-detection-max-number-of-lines"`
	ErrorDetectionSimpleRegexp  string `default:"^(?P<filename>[^:]*):(?P<line>[0-9]+):(?P<column>[0-9]+):([ ]*)?(?P<error>.*)" json:"error-detection-simple-regexp"`

	CustomConsoleName         string `json:"custom-console-name"`
	CustomConsoleURL          string `json:"custom-console-url"`
	CustomConsolePRdetail     string `json:"custom-console-url-pr-details"`
	CustomConsolePRTaskLog    string `json:"custom-console-url-pr-tasklog"`
	CustomConsoleNamespaceURL string `json:"custom-console-url-namespace"`

	RememberOKToTest bool `default:"true" json:"remember-ok-to-test"`
}

func (s *Settings) DeepCopy(out *Settings) {
	*out = *s
}

func DefaultSettings() Settings {
	newSettings := &Settings{}
	hubCatalog := &sync.Map{}
	hubCatalog.Store("default", HubCatalog{
		ID:   "default",
		Name: HubCatalogNameDefaultValue,
		URL:  HubURLDefaultValue,
	})
	newSettings.HubCatalogs = hubCatalog

	_ = configutil.ValidateAndAssignValues(nil, map[string]string{}, newSettings, map[string]func(string) error{
		"ErrorDetectionSimpleRegexp": isValidRegex,
		"TektonDashboardURL":         isValidURL,
		"CustomConsoleURL":           isValidURL,
		"CustomConsolePRTaskLog":     startWithHTTPorHTTPS,
		"CustomConsolePRDetail":      startWithHTTPorHTTPS,
	}, false)

	return *newSettings
}

func SyncConfig(logger *zap.SugaredLogger, setting *Settings, config map[string]string) error {
	setting.HubCatalogs = getHubCatalogs(logger, setting.HubCatalogs, config)

	err := configutil.ValidateAndAssignValues(logger, config, setting, map[string]func(string) error{
		"ErrorDetectionSimpleRegexp": isValidRegex,
		"TektonDashboardURL":         isValidURL,
		"CustomConsoleURL":           isValidURL,
		"CustomConsolePRTaskLog":     startWithHTTPorHTTPS,
		"CustomConsolePRDetail":      startWithHTTPorHTTPS,
	}, true)
	if err != nil {
		return fmt.Errorf("failed to validate and assign values: %w", err)
	}

	value, _ := setting.HubCatalogs.Load("default")
	catalogDefault, ok := value.(HubCatalog)
	if ok {
		if catalogDefault.URL != config[HubURLKey] {
			logger.Infof("CONFIG: hub URL set to %v", config[HubURLKey])
			catalogDefault.URL = config[HubURLKey]
		}
		if catalogDefault.Name != config[HubCatalogNameKey] {
			logger.Infof("CONFIG: hub catalog name set to %v", config[HubCatalogNameKey])
			catalogDefault.Name = config[HubCatalogNameKey]
		}
	}
	setting.HubCatalogs.Store("default", catalogDefault)
	// TODO: detect changes in extra hub catalogs

	return nil
}

func isValidURL(rawURL string) error {
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return fmt.Errorf("invalid value for URL, error: %w", err)
	}
	return nil
}

func isValidRegex(regex string) error {
	if _, err := regexp.Compile(regex); err != nil {
		return fmt.Errorf("invalid regex: %w", err)
	}
	return nil
}

func startWithHTTPorHTTPS(url string) error {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("invalid value, must start with http:// or https://")
	}
	return nil
}
