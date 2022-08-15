// Package config is a set of types for interacting with the gh configuration files.
// Note: This package is intended for use only in gh, any other use cases are subject
// to breakage and non-backwards compatible updates.
package config

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/cli/go-gh/internal/yamlmap"
)

const (
	appData       = "AppData"
	ghConfigDir   = "GH_CONFIG_DIR"
	localAppData  = "LocalAppData"
	xdgConfigHome = "XDG_CONFIG_HOME"
	xdgDataHome   = "XDG_DATA_HOME"
	xdgStateHome  = "XDG_STATE_HOME"
)

var (
	cfg  *Config
	once sync.Once
)

// Config is a in memory representation of the gh configuration files.
// It can be thought of as map where entries consist of a key that
// correspond to either a string value or a map value, allowing for
// multi-level maps.
type Config struct {
	entries *yamlmap.Map
	mu      sync.RWMutex
}

// Get a string value from a Config.
// The keys argument is a sequence of key values so that nested
// entries can be retrieved. A undefined string will be returned
// if trying to retrieve a key that corresponds to a map value.
// Returns "", KeyNotFoundError if any of the keys can not be found.
func (c *Config) Get(keys []string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m := c.entries
	for _, key := range keys {
		var err error
		m, err = m.FindEntry(key)
		if err != nil {
			return "", KeyNotFoundError{key}
		}
	}
	return m.Value, nil
}

// Keys enumerates a Config's keys.
// The keys argument is a sequence of key values so that nested
// map values can be have their keys enumerated.
// Returns nil, KeyNotFoundError if any of the keys can not be found.
func (c *Config) Keys(keys []string) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m := c.entries
	for _, key := range keys {
		var err error
		m, err = m.FindEntry(key)
		if err != nil {
			return nil, KeyNotFoundError{key}
		}
	}
	return m.Keys(), nil
}

// Remove an entry from a Config.
// The keys argument is a sequence of key values so that nested
// entries can be removed. Removing an entry that has nested
// entries removes those also.
// Returns KeyNotFoundError if any of the keys can not be found.
func (c *Config) Remove(keys []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	m := c.entries
	for i := 0; i < len(keys)-1; i++ {
		var err error
		key := keys[i]
		m, err = m.FindEntry(key)
		if err != nil {
			return KeyNotFoundError{key}
		}
	}
	err := m.RemoveEntry(keys[len(keys)-1])
	if err != nil {
		return KeyNotFoundError{keys[len(keys)-1]}
	}
	return nil
}

// Set a string value in a Config.
// The keys argument is a sequence of key values so that nested
// entries can be set. If any of the keys do not exist they will
// be created.
func (c *Config) Set(keys []string, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	m := c.entries
	for i := 0; i < len(keys)-1; i++ {
		key := keys[i]
		entry, err := m.FindEntry(key)
		if err != nil {
			entry = yamlmap.MapValue()
			m.AddEntry(key, entry)
		}
		m = entry
	}
	m.SetEntry(keys[len(keys)-1], yamlmap.StringValue(value))
}

// Read gh configuration files from the local file system and
// return a Config.
var Read = func() (*Config, error) {
	var err error
	once.Do(func() {
		cfg, err = load(generalConfigFile(), hostsConfigFile())
	})
	return cfg, err
}

// ReadFromString takes a yaml string and returns a Config.
// Note: This is only used for testing, and should not be
// relied upon in production.
func ReadFromString(str string) *Config {
	m, _ := mapFromString(str)
	if m == nil {
		m = yamlmap.MapValue()
	}
	return &Config{entries: m}
}

// Write gh configuration files to the local file system.
// It will only write gh configuration files that have been modified
// since last being read.
func Write(c *Config) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	hosts, err := c.entries.FindEntry("hosts")
	if err == nil && hosts.IsModified() {
		err := writeFile(hostsConfigFile(), []byte(hosts.String()))
		if err != nil {
			return err
		}
		hosts.SetUnmodified()
	}

	if c.entries.IsModified() {
		// Hosts gets written to a different file above so remove it
		// before writing and add it back in after writing.
		hostsMap, hostsErr := c.entries.FindEntry("hosts")
		if hostsErr == nil {
			_ = c.entries.RemoveEntry("hosts")
		}
		err := writeFile(generalConfigFile(), []byte(c.entries.String()))
		if err != nil {
			return err
		}
		c.entries.SetUnmodified()
		if hostsErr == nil {
			c.entries.AddEntry("hosts", hostsMap)
		}
	}

	return nil
}

func load(generalFilePath, hostsFilePath string) (*Config, error) {
	generalMap, err := mapFromFile(generalFilePath)
	if err != nil && !os.IsNotExist(err) {
		if errors.Is(err, yamlmap.ErrInvalidYaml) ||
			errors.Is(err, yamlmap.ErrInvalidFormat) {
			return nil, InvalidConfigFileError{Path: generalFilePath, Err: err}
		}
		return nil, err
	}

	if generalMap == nil || generalMap.Empty() {
		generalMap, _ = mapFromString(defaultGeneralEntries)
	}

	hostsMap, err := mapFromFile(hostsFilePath)
	if err != nil && !os.IsNotExist(err) {
		if errors.Is(err, yamlmap.ErrInvalidYaml) ||
			errors.Is(err, yamlmap.ErrInvalidFormat) {
			return nil, InvalidConfigFileError{Path: hostsFilePath, Err: err}
		}
		return nil, err
	}

	if hostsMap != nil && !hostsMap.Empty() {
		generalMap.AddEntry("hosts", hostsMap)
	}

	return &Config{entries: generalMap}, nil
}

func generalConfigFile() string {
	return filepath.Join(ConfigDir(), "config.yml")
}

func hostsConfigFile() string {
	return filepath.Join(ConfigDir(), "hosts.yml")
}

func mapFromFile(filename string) (*yamlmap.Map, error) {
	data, err := readFile(filename)
	if err != nil {
		return nil, err
	}
	return yamlmap.Unmarshal(data)
}

func mapFromString(str string) (*yamlmap.Map, error) {
	return yamlmap.Unmarshal([]byte(str))
}

// Config path precedence: GH_CONFIG_DIR, XDG_CONFIG_HOME, AppData (windows only), HOME.
func ConfigDir() string {
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
func StateDir() string {
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
func DataDir() string {
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

func writeFile(filename string, data []byte) error {
	err := os.MkdirAll(filepath.Dir(filename), 0771)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(data)
	return err
}

var defaultGeneralEntries = `
# What protocol to use when performing git operations. Supported values: ssh, https
git_protocol: https
# What editor gh should run when creating issues, pull requests, etc. If blank, will refer to environment.
editor:
# When to interactively prompt. This is a global config that cannot be overridden by hostname. Supported values: enabled, disabled
prompt: enabled
# A pager program to send command output to, e.g. "less". Set the value to "cat" to disable the pager.
pager:
# Aliases allow you to create nicknames for gh commands
aliases:
  co: pr checkout
# The path to a unix socket through which send HTTP connections. If blank, HTTP traffic will be handled by net/http.DefaultTransport.
http_unix_socket:
# What web browser gh should use when opening URLs. If blank, will refer to environment.
browser:
`
