package api

import (
	"fmt"
	"net/url"
	"strings"
)

// HTTPError represents an error response from the GitHub API.
type HTTPError struct {
	StatusCode  int
	RequestURL  *url.URL
	Message     string
	OAuthScopes string
	Errors      []HttpErrorItem
}

// HTTPErrorItem stores additional information about an error response
// returned from the GitHub API.
type HttpErrorItem struct {
	Message  string
	Resource string
	Field    string
	Code     string
}

// Allow HTTPError to satisfy error interface.
func (err HTTPError) Error() string {
	if msgs := strings.SplitN(err.Message, "\n", 2); len(msgs) > 1 {
		return fmt.Sprintf("HTTP %d: %s (%s)\n%s", err.StatusCode, msgs[0], err.RequestURL, msgs[1])
	} else if err.Message != "" {
		return fmt.Sprintf("HTTP %d: %s (%s)", err.StatusCode, err.Message, err.RequestURL)
	}
	return fmt.Sprintf("HTTP %d (%s)", err.StatusCode, err.RequestURL)
}
