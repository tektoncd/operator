package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cli/go-gh/internal/set"
	"gopkg.in/yaml.v3"
)

const (
	appData               = "AppData"
	defaultHost           = "github.com"
	ghConfigDir           = "GH_CONFIG_DIR"
	ghEnterpriseToken     = "GH_ENTERPRISE_TOKEN"
	ghHost                = "GH_HOST"
	ghToken               = "GH_TOKEN"
	githubEnterpriseToken = "GITHUB_ENTERPRISE_TOKEN"
	githubToken           = "GITHUB_TOKEN"
	localAppData          = "LocalAppData"
	oauthToken            = "oauth_token"
	xdgConfigHome         = "XDG_CONFIG_HOME"
	xdgDataHome           = "XDG_DATA_HOME"
	xdgStateHome          = "XDG_STATE_HOME"
)

type Config interface {
	Get(key string) (string, error)
	GetForHost(host string, key string) (string, error)
	Host() string
	Hosts() []string
	AuthToken(host string) (string, error)
}

type config struct {
	global configMap
	hosts  configMap
}

func (c config) Get(key string) (string, error) {
	return c.global.getStringValue(key)
}

func (c config) GetForHost(host, key string) (string, error) {
	hostEntry, err := c.hosts.findEntry(host)
	if err != nil {
		return "", err
	}
	hostMap := configMap{Root: hostEntry.ValueNode}
	return hostMap.getStringValue(key)
}

func (c config) Host() string {
	if host := os.Getenv(ghHost); host != "" {
		return host
	}
	entries := c.hosts.keys()
	if len(entries) == 1 {
		return entries[0]
	}
	return defaultHost
}

func (c config) Hosts() []string {
	hosts := set.NewStringSet()
	if host := os.Getenv(ghHost); host != "" {
		hosts.Add(host)
	}
	entries := c.hosts.keys()
	hosts.AddValues(entries)
	return hosts.ToSlice()
}

func (c config) AuthToken(host string) (string, error) {
	hostname := normalizeHostname(host)
	if isEnterprise(hostname) {
		if token := os.Getenv(ghEnterpriseToken); token != "" {
			return token, nil
		}
		if token := os.Getenv(githubEnterpriseToken); token != "" {
			return token, nil
		}
		if token, err := c.GetForHost(hostname, oauthToken); err == nil {
			return token, nil
		}
		return "", NotFoundError{errors.New("not found")}
	}

	if token := os.Getenv(ghToken); token != "" {
		return token, nil
	}
	if token := os.Getenv(githubToken); token != "" {
		return token, nil
	}
	if token, err := c.GetForHost(hostname, oauthToken); err == nil {
		return token, nil
	}
	return "", NotFoundError{errors.New("not found")}
}

func isEnterprise(host string) bool {
	return host != defaultHost
}

func normalizeHostname(host string) string {
	hostname := strings.ToLower(host)
	if strings.HasSuffix(hostname, "."+defaultHost) {
		return defaultHost
	}
	return hostname
}

func FromString(str string) (Config, error) {
	root, err := parseData([]byte(str))
	if err != nil {
		return nil, err
	}
	cfg := config{}
	globalMap := configMap{Root: root}
	cfg.global = globalMap
	hostsEntry, err := globalMap.findEntry("hosts")
	if err == nil {
		cfg.hosts = configMap{Root: hostsEntry.ValueNode}
	}
	return cfg, nil
}

func defaultConfig() Config {
	return config{global: configMap{Root: defaultGlobal().Content[0]}}
}

func Load() (Config, error) {
	return load(configFile(), hostsConfigFile())
}

func load(globalFilePath, hostsFilePath string) (Config, error) {
	var readErr error
	var parseErr error
	globalData, readErr := readFile(globalFilePath)
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
		return nil, readErr
	}

	// Use defaultGlobal node if globalFile does not exist or is empty.
	global := defaultGlobal().Content[0]
	if len(globalData) > 0 {
		global, parseErr = parseData(globalData)
	}
	if parseErr != nil {
		return nil, parseErr
	}

	hostsData, readErr := readFile(hostsFilePath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return nil, readErr
	}

	// Use nil if hostsFile does not exist or is empty.
	var hosts *yaml.Node
	if len(hostsData) > 0 {
		hosts, parseErr = parseData(hostsData)
	}
	if parseErr != nil {
		return nil, parseErr
	}

	cfg := config{
		global: configMap{Root: global},
		hosts:  configMap{Root: hosts},
	}

	return cfg, nil
}

// Config path precedence: GH_CONFIG_DIR, XDG_CONFIG_HOME, AppData (windows only), HOME.
func configDir() string {
	var path string
	if a := os.Getenv(ghConfigDir); a != "" {
		path = a
	} else if b := os.Getenv(xdgConfigHome); b != "" {
		path = filepath.Join(b, "gh")
	} else if c := os.Getenv(appData); runtime.GOOS == "windows" && c != "" {
		path = filepath.Join(c, "GitHub CLI")
	} else {
		d, _ := os.UserHomeDir()
		path = filepath.Join(d, ".config", "gh")
	}
	return path
}

// State path precedence: XDG_STATE_HOME, LocalAppData (windows only), HOME.
func stateDir() string {
	var path string
	if a := os.Getenv(xdgStateHome); a != "" {
		path = filepath.Join(a, "gh")
	} else if b := os.Getenv(localAppData); runtime.GOOS == "windows" && b != "" {
		path = filepath.Join(b, "GitHub CLI")
	} else {
		c, _ := os.UserHomeDir()
		path = filepath.Join(c, ".local", "state", "gh")
	}
	return path
}

// Data path precedence: XDG_DATA_HOME, LocalAppData (windows only), HOME.
func dataDir() string {
	var path string
	if a := os.Getenv(xdgDataHome); a != "" {
		path = filepath.Join(a, "gh")
	} else if b := os.Getenv(localAppData); runtime.GOOS == "windows" && b != "" {
		path = filepath.Join(b, "GitHub CLI")
	} else {
		c, _ := os.UserHomeDir()
		path = filepath.Join(c, ".local", "share", "gh")
	}
	return path
}

func configFile() string {
	return filepath.Join(configDir(), "config.yml")
}

func hostsConfigFile() string {
	return filepath.Join(configDir(), "hosts.yml")
}

func readFile(filename string) ([]byte, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func parseData(data []byte) (*yaml.Node, error) {
	var root yaml.Node
	err := yaml.Unmarshal(data, &root)
	if err != nil {
		return nil, fmt.Errorf("invalid config file: %w", err)
	}
	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("invalid config file")
	}
	return root.Content[0], nil
}

func defaultGlobal() *yaml.Node {
	return &yaml.Node{
		Kind: yaml.DocumentNode,
		Content: []*yaml.Node{
			{
				Kind: yaml.MappingNode,
				Content: []*yaml.Node{
					{
						HeadComment: "What protocol to use when performing git operations. Supported values: ssh, https",
						Kind:        yaml.ScalarNode,
						Value:       "git_protocol",
					},
					{
						Kind:  yaml.ScalarNode,
						Value: "https",
					},
					{
						HeadComment: "What editor gh should run when creating issues, pull requests, etc. If blank, will refer to environment.",
						Kind:        yaml.ScalarNode,
						Value:       "editor",
					},
					{
						Kind:  yaml.ScalarNode,
						Value: "",
					},
					{
						HeadComment: "When to interactively prompt. This is a global config that cannot be overridden by hostname. Supported values: enabled, disabled",
						Kind:        yaml.ScalarNode,
						Value:       "prompt",
					},
					{
						Kind:  yaml.ScalarNode,
						Value: "enabled",
					},
					{
						HeadComment: "A pager program to send command output to, e.g. \"less\". Set the value to \"cat\" to disable the pager.",
						Kind:        yaml.ScalarNode,
						Value:       "pager",
					},
					{
						Kind:  yaml.ScalarNode,
						Value: "",
					},
					{
						HeadComment: "Aliases allow you to create nicknames for gh commands",
						Kind:        yaml.ScalarNode,
						Value:       "aliases",
					},
					{
						Kind: yaml.MappingNode,
						Content: []*yaml.Node{
							{
								Kind:  yaml.ScalarNode,
								Value: "co",
							},
							{
								Kind:  yaml.ScalarNode,
								Value: "pr checkout",
							},
						},
					},
					{
						HeadComment: "The path to a unix socket through which send HTTP connections. If blank, HTTP traffic will be handled by net/http.DefaultTransport.",
						Kind:        yaml.ScalarNode,
						Value:       "http_unix_socket",
					},
					{
						Kind:  yaml.ScalarNode,
						Value: "",
					},
					{
						HeadComment: "What web browser gh should use when opening URLs. If blank, will refer to environment.",
						Kind:        yaml.ScalarNode,
						Value:       "browser",
					},
					{
						Kind:  yaml.ScalarNode,
						Value: "",
					},
				},
			},
		},
	}
}
