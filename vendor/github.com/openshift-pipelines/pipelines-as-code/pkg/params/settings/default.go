package settings

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	hubtypes "github.com/openshift-pipelines/pipelines-as-code/pkg/hub/vars"
	"go.uber.org/zap"
)

func getHubCatalogs(logger *zap.SugaredLogger, catalogs *sync.Map, config map[string]string) *sync.Map {
	if catalogs == nil {
		catalogs = &sync.Map{}
	}
	if hubURL, ok := config[HubURLKey]; !ok || hubURL == "" {
		config[HubURLKey] = ArtifactHubURLDefaultValue
		logger.Infof("CONFIG: using default hub url %s", ArtifactHubURLDefaultValue)
	}

	if hubType, ok := config[HubCatalogTypeKey]; !ok || hubType == "" {
		config[HubCatalogTypeKey] = hubtypes.ArtifactHubType
		if config[HubURLKey] != "" {
			config[HubCatalogTypeKey] = getHubCatalogTypeViaAPI(config[HubURLKey])
		}
	} else if hubType != hubtypes.ArtifactHubType && hubType != hubtypes.TektonHubType {
		logger.Warnf("CONFIG: invalid hub type %s, defaulting to %s", hubType, hubtypes.ArtifactHubType)
		config[HubCatalogTypeKey] = hubtypes.ArtifactHubType
	}
	hc := HubCatalog{
		Index: "default",
		Name:  config[HubCatalogNameKey],
		URL:   config[HubURLKey],
		Type:  config[HubCatalogTypeKey],
	}
	catalogs.Store("default", hc)

	exists := false
	catalogs.Range(func(_, value interface{}) bool {
		if catalog, ok := value.(HubCatalog); ok && catalog.Type == hubtypes.TektonHubType {
			exists = true
			return false // Stop iteration
		}
		return true // Continue iteration
	})
	if !exists {
		catalogs.Store(hubtypes.TektonHubType, HubCatalog{
			Index: hubtypes.TektonHubType,
			Name:  TektonHubCatalogNameDefaultValue,
			URL:   TektonHubURLDefaultValue,
			Type:  hubtypes.TektonHubType,
		})
	}

	for k := range config {
		m := hubCatalogNameRegex.FindStringSubmatch(k)
		if len(m) > 0 {
			index := m[1]
			cPrefix := fmt.Sprintf("catalog-%s", index)
			skip := false
			for _, kk := range []string{"id", "name", "url"} {
				cKey := fmt.Sprintf("%s-%s", cPrefix, kk)
				// check if key exist in config
				if _, ok := config[cKey]; !ok {
					logger.Warnf("CONFIG: hub %v should have the key %s, skipping catalog configuration", index, cKey)
					skip = true
					break
				} else if config[cKey] == "" {
					logger.Warnf("CONFIG: hub %v catalog configuration have empty value for key %s, skipping catalog configuration", index, cKey)
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
				catalogType := config[fmt.Sprintf("%s-type", cPrefix)]
				if catalogType == "" {
					catalogType = hubtypes.ArtifactHubType // default to artifact hub if not specified
				}

				value, ok := catalogs.Load(catalogID)
				if ok {
					catalogValues, ok := value.(HubCatalog)
					if ok && (catalogValues.Name == catalogName) && (catalogValues.URL == catalogURL) && (catalogValues.Index == index) && (catalogValues.Type == catalogType) {
						continue
					}
				}
				logger.Infof("CONFIG: setting custom hub %s, catalog %s", catalogID, catalogURL)
				catalogs.Store(catalogID, HubCatalog{
					Index: index,
					Name:  catalogName,
					URL:   catalogURL,
					Type:  catalogType,
				})
			}
		}
	}
	return catalogs
}

func getHubCatalogTypeViaAPI(hubURL string) string {
	statsURL := fmt.Sprintf("%s/api/v1/stats", strings.TrimSuffix(hubURL, "/"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, statsURL, nil)
	if err != nil {
		return hubtypes.TektonHubType
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return hubtypes.TektonHubType
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return hubtypes.ArtifactHubType
	}

	// if the API call fails, return Tekton Hub type
	return hubtypes.TektonHubType
}
