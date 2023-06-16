// Package gh is a library for CLI Go applications to help interface with the gh CLI tool,
// and the GitHub API.
//
// Note that the examples in this package assume gh and git are installed. They do not run in
// the Go Playground used by pkg.go.dev.
package gh

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"

	iapi "github.com/cli/go-gh/internal/api"
	"github.com/cli/go-gh/internal/git"
	irepo "github.com/cli/go-gh/internal/repository"
	"github.com/cli/go-gh/pkg/api"
	"github.com/cli/go-gh/pkg/auth"
	"github.com/cli/go-gh/pkg/config"
	repo "github.com/cli/go-gh/pkg/repository"
	"github.com/cli/go-gh/pkg/ssh"
	"github.com/cli/safeexec"
)

// Exec gh command with provided arguments.
func Exec(args ...string) (stdOut, stdErr bytes.Buffer, err error) {
	path, err := path()
	if err != nil {
		err = fmt.Errorf("could not find gh executable in PATH. error: %w", err)
		return
	}
	return run(path, nil, args...)
}

func path() (string, error) {
	return safeexec.LookPath("gh")
}

func run(path string, env []string, args ...string) (stdOut, stdErr bytes.Buffer, err error) {
	cmd := exec.Command(path, args...)
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr
	if env != nil {
		cmd.Env = env
	}
	err = cmd.Run()
	if err != nil {
		err = fmt.Errorf("failed to run gh: %s. error: %w", stdErr.String(), err)
		return
	}
	return
}

// RESTClient builds a client to send requests to GitHub REST API endpoints.
// As part of the configuration a hostname, auth token, default set of headers,
// and unix domain socket are resolved from the gh environment configuration.
// These behaviors can be overridden using the opts argument.
func RESTClient(opts *api.ClientOptions) (api.RESTClient, error) {
	if opts == nil {
		opts = &api.ClientOptions{}
	}
	if optionsNeedResolution(opts) {
		err := resolveOptions(opts)
		if err != nil {
			return nil, err
		}
	}
	return iapi.NewRESTClient(opts.Host, opts), nil
}

// GQLClient builds a client to send requests to GitHub GraphQL API endpoints.
// As part of the configuration a hostname, auth token, default set of headers,
// and unix domain socket are resolved from the gh environment configuration.
// These behaviors can be overridden using the opts argument.
func GQLClient(opts *api.ClientOptions) (api.GQLClient, error) {
	if opts == nil {
		opts = &api.ClientOptions{}
	}
	if optionsNeedResolution(opts) {
		err := resolveOptions(opts)
		if err != nil {
			return nil, err
		}
	}
	return iapi.NewGQLClient(opts.Host, opts), nil
}

// HTTPClient builds a client that can be passed to another library.
// As part of the configuration a hostname, auth token, default set of headers,
// and unix domain socket are resolved from the gh environment configuration.
// These behaviors can be overridden using the opts argument. In this instance
// providing opts.Host will not change the destination of your request as it is
// the responsibility of the consumer to configure this. However, if opts.Host
// does not match the request host, the auth token will not be added to the headers.
// This is to protect against the case where tokens could be sent to an arbitrary
// host.
func HTTPClient(opts *api.ClientOptions) (*http.Client, error) {
	if opts == nil {
		opts = &api.ClientOptions{}
	}
	if optionsNeedResolution(opts) {
		err := resolveOptions(opts)
		if err != nil {
			return nil, err
		}
	}
	client := iapi.NewHTTPClient(opts)
	return &client, nil
}

// CurrentRepository uses git remotes to determine the GitHub repository
// the current directory is tracking.
func CurrentRepository() (repo.Repository, error) {
	override := os.Getenv("GH_REPO")
	if override != "" {
		return repo.Parse(override)
	}

	remotes, err := git.Remotes()
	if err != nil {
		return nil, err
	}
	if len(remotes) == 0 {
		return nil, errors.New("unable to determine current repository, no git remotes configured for this repository")
	}

	translator := ssh.NewTranslator()
	for _, r := range remotes {
		if r.FetchURL != nil {
			r.FetchURL = translator.Translate(r.FetchURL)
		}
		if r.PushURL != nil {
			r.PushURL = translator.Translate(r.PushURL)
		}
	}

	hosts := auth.KnownHosts()

	filteredRemotes := remotes.FilterByHosts(hosts)
	if len(filteredRemotes) == 0 {
		return nil, errors.New("unable to determine current repository, none of the git remotes configured for this repository point to a known GitHub host")
	}

	r := filteredRemotes[0]
	return irepo.New(r.Host, r.Owner, r.Repo), nil
}

func optionsNeedResolution(opts *api.ClientOptions) bool {
	if opts.Host == "" {
		return true
	}
	if opts.AuthToken == "" {
		return true
	}
	if opts.UnixDomainSocket == "" && opts.Transport == nil {
		return true
	}
	return false
}

func resolveOptions(opts *api.ClientOptions) error {
	cfg, _ := config.Read()
	if opts.Host == "" {
		opts.Host, _ = auth.DefaultHost()
	}
	if opts.AuthToken == "" {
		opts.AuthToken, _ = auth.TokenForHost(opts.Host)
		if opts.AuthToken == "" {
			return fmt.Errorf("authentication token not found for host %s", opts.Host)
		}
	}
	if opts.UnixDomainSocket == "" && cfg != nil {
		opts.UnixDomainSocket, _ = cfg.Get([]string{"http_unix_socket"})
	}
	return nil
}
