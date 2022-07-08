package git

import (
	"fmt"
	"net/url"
	"strings"
)

func IsURL(u string) bool {
	return strings.HasPrefix(u, "git@") || isSupportedProtocol(u)
}

func isSupportedProtocol(u string) bool {
	return strings.HasPrefix(u, "ssh:") ||
		strings.HasPrefix(u, "git+ssh:") ||
		strings.HasPrefix(u, "git:") ||
		strings.HasPrefix(u, "http:") ||
		strings.HasPrefix(u, "git+https:") ||
		strings.HasPrefix(u, "https:")
}

func isPossibleProtocol(u string) bool {
	return isSupportedProtocol(u) ||
		strings.HasPrefix(u, "ftp:") ||
		strings.HasPrefix(u, "ftps:") ||
		strings.HasPrefix(u, "file:")
}

// ParseURL normalizes git remote urls.
func ParseURL(rawURL string) (u *url.URL, err error) {
	if !isPossibleProtocol(rawURL) &&
		strings.ContainsRune(rawURL, ':') &&
		// Not a Windows path.
		!strings.ContainsRune(rawURL, '\\') {
		// Support scp-like syntax for ssh protocol.
		rawURL = "ssh://" + strings.Replace(rawURL, ":", "/", 1)
	}

	u, err = url.Parse(rawURL)
	if err != nil {
		return
	}

	if u.Scheme == "git+ssh" {
		u.Scheme = "ssh"
	}

	if u.Scheme == "git+https" {
		u.Scheme = "https"
	}

	if u.Scheme != "ssh" {
		return
	}

	if strings.HasPrefix(u.Path, "//") {
		u.Path = strings.TrimPrefix(u.Path, "/")
	}

	if idx := strings.Index(u.Host, ":"); idx >= 0 {
		u.Host = u.Host[0:idx]
	}

	return
}

// Extract GitHub repository information from a git remote URL.
func RepoInfoFromURL(u *url.URL) (host string, owner string, name string, err error) {
	if u.Hostname() == "" {
		return "", "", "", fmt.Errorf("no hostname detected")
	}

	parts := strings.SplitN(strings.Trim(u.Path, "/"), "/", 3)
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("invalid path: %s", u.Path)
	}

	return normalizeHostname(u.Hostname()), parts[0], strings.TrimSuffix(parts[1], ".git"), nil
}

func normalizeHostname(h string) string {
	return strings.ToLower(strings.TrimPrefix(h, "www."))
}
