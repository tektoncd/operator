// Package auth is a set of functions for retrieving authentication tokens
// and authenticated hosts.
package auth

import (
	"os"
	"strconv"
	"strings"

	"github.com/cli/go-gh/internal/set"
	"github.com/cli/go-gh/pkg/config"
)

const (
	codespaces            = "CODESPACES"
	defaultSource         = "default"
	ghEnterpriseToken     = "GH_ENTERPRISE_TOKEN"
	ghHost                = "GH_HOST"
	ghToken               = "GH_TOKEN"
	github                = "github.com"
	githubEnterpriseToken = "GITHUB_ENTERPRISE_TOKEN"
	githubToken           = "GITHUB_TOKEN"
	hostsKey              = "hosts"
	localhost             = "github.localhost"
	oauthToken            = "oauth_token"
)

// TokenForHost retrieves an authentication token and the source of
// that token for the specified host. The source can be either an
// environment variable or from the configuration file.
// Returns "", "default" if no applicable token is found.
func TokenForHost(host string) (string, string) {
	cfg, _ := config.Read()
	return tokenForHost(cfg, host)
}

func tokenForHost(cfg *config.Config, host string) (string, string) {
	host = normalizeHostname(host)
	if isEnterprise(host) {
		if token := os.Getenv(ghEnterpriseToken); token != "" {
			return token, ghEnterpriseToken
		}
		if token := os.Getenv(githubEnterpriseToken); token != "" {
			return token, githubEnterpriseToken
		}
		if isCodespaces, _ := strconv.ParseBool(os.Getenv(codespaces)); isCodespaces {
			if token := os.Getenv(githubToken); token != "" {
				return token, githubToken
			}
		}
		if cfg != nil {
			token, _ := cfg.Get([]string{hostsKey, host, oauthToken})
			return token, oauthToken
		}
	}
	if token := os.Getenv(ghToken); token != "" {
		return token, ghToken
	}
	if token := os.Getenv(githubToken); token != "" {
		return token, githubToken
	}
	if cfg != nil {
		token, _ := cfg.Get([]string{hostsKey, host, oauthToken})
		return token, oauthToken
	}
	return "", defaultSource
}

// KnownHosts retrieves a list of hosts that have corresponding
// authentication tokens, either from environment variables
// or from the configuration file.
// Returns an empty string slice if no hosts are found.
func KnownHosts() []string {
	cfg, _ := config.Read()
	return knownHosts(cfg)
}

func knownHosts(cfg *config.Config) []string {
	hosts := set.NewStringSet()
	if host := os.Getenv(ghHost); host != "" {
		hosts.Add(host)
	}
	if token, _ := tokenForHost(cfg, github); token != "" {
		hosts.Add(github)
	}
	if cfg != nil {
		keys, err := cfg.Keys([]string{hostsKey})
		if err == nil {
			hosts.AddValues(keys)
		}
	}
	return hosts.ToSlice()
}

// DefaultHost retrieves an authenticated host and the source of host.
// The source can be either an environment variable or from the
// configuration file.
// Returns "github.com", "default" if no viable host is found.
func DefaultHost() (string, string) {
	cfg, _ := config.Read()
	return defaultHost(cfg)
}

func defaultHost(cfg *config.Config) (string, string) {
	if host := os.Getenv(ghHost); host != "" {
		return host, ghHost
	}
	if cfg != nil {
		keys, err := cfg.Keys([]string{hostsKey})
		if err == nil && len(keys) == 1 {
			return keys[0], hostsKey
		}
	}
	return github, defaultSource
}

func isEnterprise(host string) bool {
	return host != github && host != localhost
}

func normalizeHostname(host string) string {
	hostname := strings.ToLower(host)
	if strings.HasSuffix(hostname, "."+github) {
		return github
	}
	if strings.HasSuffix(hostname, "."+localhost) {
		return localhost
	}
	return hostname
}
