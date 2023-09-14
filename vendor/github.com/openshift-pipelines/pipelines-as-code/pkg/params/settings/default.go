package settings

import (
	"fmt"
	"net/url"
	"strconv"
	"sync"

	"go.uber.org/zap"
)

func getHubCatalogs(logger *zap.SugaredLogger, catalogs *sync.Map, config map[string]string) *sync.Map {
	if catalogs == nil {
		catalogs = &sync.Map{}
	}
	if hubURL, ok := config[HubURLKey]; !ok || hubURL == "" {
		config[HubURLKey] = HubURLDefaultValue
		logger.Infof("CONFIG: using default hub url %s", HubURLDefaultValue)
	}

	if hubCatalogName, ok := config[HubCatalogNameKey]; !ok || hubCatalogName == "" {
		config[HubCatalogNameKey] = HubCatalogNameDefaultValue
	}
	catalogs.Store("default", HubCatalog{
		ID:   "default",
		Name: config[HubCatalogNameKey],
		URL:  config[HubURLKey],
	})

	for k := range config {
		m := hubCatalogNameRegex.FindStringSubmatch(k)
		if len(m) > 0 {
			id := m[1]
			cPrefix := fmt.Sprintf("catalog-%s", id)
			skip := false
			for _, kk := range []string{"id", "name", "url"} {
				cKey := fmt.Sprintf("%s-%s", cPrefix, kk)
				// check if key exist in config
				if _, ok := config[cKey]; !ok {
					logger.Warnf("CONFIG: hub %v should have the key %s, skipping catalog configuration", id, cKey)
					skip = true
					break
				} else if config[cKey] == "" {
					logger.Warnf("CONFIG: hub %v catalog configuration is empty, skipping catalog configuration", id)
					skip = true
					break
				}
			}
			if !skip {
				catalogID := config[fmt.Sprintf("%s-id", cPrefix)]
				if catalogID == "http" || catalogID == "https" {
					logger.Warnf("CONFIG: custom hub catalog name cannot be %s, skipping catalog configuration", catalogID)
					break
				}
				catalogURL := config[fmt.Sprintf("%s-url", cPrefix)]
				u, err := url.Parse(catalogURL)
				if err != nil || u.Scheme == "" || u.Host == "" {
					logger.Warnf("CONFIG: custom hub %s, catalog url %s is not valid, skipping catalog configuration", catalogID, catalogURL)
					break
				}
				catalogName := config[fmt.Sprintf("%s-name", cPrefix)]
				value, ok := catalogs.Load(catalogID)
				if ok {
					catalogValues, ok := value.(HubCatalog)
					if ok && (catalogValues.Name == catalogName) && (catalogValues.URL == catalogURL) {
						break
					}
				}
				logger.Infof("CONFIG: setting custom hub %s, catalog %s", catalogID, catalogURL)
				catalogs.Store(catalogID, HubCatalog{
					ID:   catalogID,
					Name: catalogName,
					URL:  catalogURL,
				})
			}
		}
	}
	return catalogs
}

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

	if rememberOKToTest, ok := config[RememberOKToTestKey]; !ok || rememberOKToTest == "" {
		config[RememberOKToTestKey] = rememberOKToTestValue
	}
}
