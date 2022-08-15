// Package repository is a set of types and functions for modeling and
// interacting with GitHub repositories.
package repository

import (
	"fmt"
	"strings"

	"github.com/cli/go-gh/internal/git"
	irepo "github.com/cli/go-gh/internal/repository"
	"github.com/cli/go-gh/pkg/auth"
)

// Repository is the interface that wraps repository information methods.
type Repository interface {
	Host() string
	Name() string
	Owner() string
}

// Parse extracts the repository information from the following
// string formats: "OWNER/REPO", "HOST/OWNER/REPO", and a full URL.
// If the format does not specify a host, use the config to determine a host.
func Parse(s string) (Repository, error) {
	if git.IsURL(s) {
		u, err := git.ParseURL(s)
		if err != nil {
			return nil, err
		}

		host, owner, name, err := git.RepoInfoFromURL(u)
		if err != nil {
			return nil, err
		}

		return irepo.New(host, owner, name), nil
	}

	parts := strings.SplitN(s, "/", 4)
	for _, p := range parts {
		if len(p) == 0 {
			return nil, fmt.Errorf(`expected the "[HOST/]OWNER/REPO" format, got %q`, s)
		}
	}

	switch len(parts) {
	case 3:
		return irepo.New(parts[0], parts[1], parts[2]), nil
	case 2:
		host, _ := auth.DefaultHost()
		return irepo.New(host, parts[0], parts[1]), nil
	default:
		return nil, fmt.Errorf(`expected the "[HOST/]OWNER/REPO" format, got %q`, s)
	}
}

// Parse extracts the repository information from the following
// string formats: "OWNER/REPO", "HOST/OWNER/REPO", and a full URL.
// If the format does not specify a host, use the host provided.
func ParseWithHost(s, host string) (Repository, error) {
	if git.IsURL(s) {
		u, err := git.ParseURL(s)
		if err != nil {
			return nil, err
		}

		host, owner, name, err := git.RepoInfoFromURL(u)
		if err != nil {
			return nil, err
		}

		return irepo.New(host, owner, name), nil
	}

	parts := strings.SplitN(s, "/", 4)
	for _, p := range parts {
		if len(p) == 0 {
			return nil, fmt.Errorf(`expected the "[HOST/]OWNER/REPO" format, got %q`, s)
		}
	}

	switch len(parts) {
	case 3:
		return irepo.New(parts[0], parts[1], parts[2]), nil
	case 2:
		return irepo.New(host, parts[0], parts[1]), nil
	default:
		return nil, fmt.Errorf(`expected the "[HOST/]OWNER/REPO" format, got %q`, s)
	}
}
