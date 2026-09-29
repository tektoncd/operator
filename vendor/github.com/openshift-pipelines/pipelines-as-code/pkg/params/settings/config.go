package settings

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/configutil"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/vcshost"
	"go.uber.org/zap"
)

const (
	PACApplicationNameDefaultValue = "Pipelines as Code CI"

	HubURLKey                  = "hub-url"
	HubCatalogNameKey          = "hub-catalog-name"
	ArtifactHubURLDefaultValue = "https://artifacthub.io"

	CustomConsoleNameKey         = "custom-console-name"
	CustomConsoleURLKey          = "custom-console-url"
	CustomConsolePRDetailKey     = "custom-console-url-pr-details"
	CustomConsolePRTaskLogKey    = "custom-console-url-pr-tasklog"
	CustomConsoleNamespaceURLKey = "custom-console-url-namespace"

	SecretGhAppTokenRepoScopedKey = "secret-github-app-token-scoped" //nolint: gosec

	// TrustedProviderHostnamesKey is the ConfigMap key holding the comma
	// separated list of hosted VCS hostnames this controller is allowed to send
	// credentials to.
	TrustedProviderHostnamesKey = "trusted-provider-hostnames"

	// DefaultAPIRetryMaxAttempts is the fallback number of attempts (initial
	// request included) used when api-retry-max-attempts is unset or invalid.
	DefaultAPIRetryMaxAttempts = 4
	// DefaultAPIRetryMaxWaitSeconds is the fallback cap on the wait between
	// attempts used when api-retry-max-wait-seconds is unset or invalid.
	DefaultAPIRetryMaxWaitSeconds = 120
)

var (
	TknBinaryName       = `tkn`
	TknBinaryURL        = `https://tekton.dev/docs/cli/#installation`
	hubCatalogNameRegex = regexp.MustCompile(`^catalog-(\d+)-`)
)

type HubCatalog struct {
	Index string
	Name  string
	URL   string
}

// if there is a change performed on the default value,
// update the same on "config/302-pac-configmap.yaml".
type Settings struct {
	ApplicationName                     string `default:"Pipelines as Code CI" json:"application-name"`
	HubCatalogs                         *sync.Map
	RemoteTasks                         bool   `default:"true"                                 json:"remote-tasks"`
	MaxKeepRunsUpperLimit               int    `json:"max-keep-run-upper-limit"`
	DefaultMaxKeepRuns                  int    `json:"default-max-keep-runs"`
	BitbucketCloudCheckSourceIP         bool   `default:"true"                                 json:"bitbucket-cloud-check-source-ip"`
	BitbucketCloudAdditionalSourceIP    string `json:"bitbucket-cloud-additional-source-ip"`
	TektonDashboardURL                  string `json:"tekton-dashboard-url"`
	AutoConfigureNewGitHubRepo          bool   `default:"false"                                json:"auto-configure-new-github-repo"`
	AutoConfigureRepoNamespaceTemplate  string `json:"auto-configure-repo-namespace-template"`
	AutoConfigureRepoRepositoryTemplate string `json:"auto-configure-repo-repository-template"`

	// TrustedProviderHostnames is the comma separated allowlist of hosted VCS
	// hostnames this controller may send credentials to.
	//
	// It is here so that the value is validated on every ConfigMap change and
	// surfaced with the rest of the settings. The security gate in
	// pkg/hostpolicy deliberately does NOT read this field: it reads the
	// ConfigMap live, because the informer cache may lag behind an administrator
	// narrowing the allowlist, and because trust on first use has to update the
	// learned-host annotation under a read-modify-write it can retry on conflict.
	TrustedProviderHostnames string `json:"trusted-provider-hostnames"`

	SecretAutoCreation               bool   `default:"true"                             json:"secret-auto-create"`
	SecretGHAppRepoScoped            bool   `default:"true"                             json:"secret-github-app-token-scoped"`
	SecretGhAppTokenScopedExtraRepos string `json:"secret-github-app-scope-extra-repos"`

	ErrorLogSnippet              bool   `default:"true"                                                                          json:"error-log-snippet"`
	ErrorLogSnippetNumberOfLines int    `default:"3"                                                                             json:"error-log-snippet-number-of-lines"`
	ErrorDetection               bool   `default:"true"                                                                          json:"error-detection-from-container-logs"`
	ErrorDetectionNumberOfLines  int    `default:"50"                                                                            json:"error-detection-max-number-of-lines"`
	ErrorDetectionSimpleRegexp   string `default:"^(?P<filename>[^:]*):(?P<line>[0-9]+):(?P<column>[0-9]+)?([ ]*)?(?P<error>.*)" json:"error-detection-simple-regexp"`

	EnableCancelInProgressOnPullRequests bool `json:"enable-cancel-in-progress-on-pull-requests"`
	EnableCancelInProgressOnPush         bool `json:"enable-cancel-in-progress-on-push"`

	SkipPushEventForPRCommits bool `json:"skip-push-event-for-pr-commits" default:"true"` // nolint:tagalign

	CustomConsoleName         string `json:"custom-console-name"`
	CustomConsoleURL          string `json:"custom-console-url"`
	CustomConsolePRdetail     string `json:"custom-console-url-pr-details"`
	CustomConsolePRTaskLog    string `json:"custom-console-url-pr-tasklog"`
	CustomConsoleNamespaceURL string `json:"custom-console-url-namespace"`

	RememberOKToTest   bool `json:"remember-ok-to-test"`
	RequireOkToTestSHA bool `json:"require-ok-to-test-sha"`

	// Retry Git provider API requests on rate limits and transient errors.
	// Disabled by default.
	EnableAPIRetry         bool `default:"false" json:"enable-api-retry"`
	APIRetryMaxAttempts    int  `default:"4"     json:"api-retry-max-attempts"`
	APIRetryMaxWaitSeconds int  `default:"120"   json:"api-retry-max-wait-seconds"`

	// Tracing label names. Defaults in config/302-pac-configmap.yaml.
	TracingLabelAction      string `json:"tracing-label-action"`
	TracingLabelApplication string `json:"tracing-label-application"`
	TracingLabelComponent   string `json:"tracing-label-component"`
}

func (s *Settings) DeepCopy(out *Settings) {
	*out = *s
}

func DefaultSettings() Settings {
	newSettings := &Settings{}
	hubCatalog := &sync.Map{}
	hubCatalog.Store("default", HubCatalog{
		Index: "default",
		URL:   ArtifactHubURLDefaultValue,
	})
	newSettings.HubCatalogs = hubCatalog

	_ = configutil.ValidateAndAssignValues(nil, map[string]string{}, newSettings, map[string]func(string) error{}, false)

	return *newSettings
}

func DefaultValidators() map[string]func(string) error {
	return map[string]func(string) error{
		"ErrorDetectionSimpleRegexp": isValidRegex,
		"TektonDashboardURL":         isValidURL,
		"CustomConsoleURL":           isValidURL,
		"CustomConsolePRTaskLog":     startWithHTTPorHTTPS,
		"CustomConsolePRDetail":      startWithHTTPorHTTPS,
		"TrustedProviderHostnames":   isValidTrustedProviderHostnames,
	}
}

// isValidTrustedProviderHostnames validates the hosted VCS allowlist. An empty
// value is accepted and means the allowlist has not been configured yet.
func isValidTrustedProviderHostnames(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	if _, err := vcshost.ParseAllowlist(raw); err != nil {
		return fmt.Errorf("invalid value for %s: %w", TrustedProviderHostnamesKey, err)
	}
	return nil
}

func SyncConfig(logger *zap.SugaredLogger, setting *Settings, config map[string]string, validators map[string]func(string) error) error {
	setting.HubCatalogs = getHubCatalogs(logger, setting.HubCatalogs, config)

	err := configutil.ValidateAndAssignValues(logger, config, setting, validators, true)
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
