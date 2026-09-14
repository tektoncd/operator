package api

import (
	"github.com/cli/go-gh/v2/pkg/auth"
	"github.com/cli/go-gh/v2/pkg/config"
)

const (
	hostsKey   = "hosts"
	apiHostKey = "api_host"
)

// apiHost returns the api_host value configured for host in hosts.yml.
// The boolean reports whether a non-empty value was found. The value is not validated.
func apiHost(host string) (string, bool) {
	cfg, err := config.Read(nil)
	if err != nil || cfg == nil {
		return "", false
	}

	normalizedHost := auth.NormalizeHostname(host)
	configuredAPIHost, err := cfg.Get([]string{hostsKey, normalizedHost, apiHostKey})
	if err != nil || configuredAPIHost == "" {
		return "", false
	}
	return configuredAPIHost, true
}
