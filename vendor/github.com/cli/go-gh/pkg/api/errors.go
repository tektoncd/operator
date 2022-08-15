package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	contentType = "Content-Type"
)

var jsonTypeRE = regexp.MustCompile(`[/+]json($|;)`)

// HTTPError represents an error response from the GitHub API.
type HTTPError struct {
	Errors     []HTTPErrorItem
	Headers    http.Header
	Message    string
	RequestURL *url.URL
	StatusCode int
}

// HTTPErrorItem stores additional information about an error response
// returned from the GitHub API.
type HTTPErrorItem struct {
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

// GQLError represents an error response from GitHub GraphQL API.
type GQLError struct {
	Errors []GQLErrorItem
}

// GQLErrorItem stores additional information about an error response
// returned from the GitHub GraphQL API.
type GQLErrorItem struct {
	Message string
	Path    []interface{}
	Type    string
}

// Allow GQLError to satisfy error interface.
func (gr GQLError) Error() string {
	errorMessages := make([]string, 0, len(gr.Errors))
	for _, e := range gr.Errors {
		msg := e.Message
		if p := e.pathString(); p != "" {
			msg = fmt.Sprintf("%s (%s)", msg, p)
		}
		errorMessages = append(errorMessages, msg)
	}
	return fmt.Sprintf("GraphQL: %s", strings.Join(errorMessages, ", "))
}

// Match determines if the GQLError is about a specific type on a specific path.
// If the path argument ends with a ".", it will match all its subpaths.
func (gr GQLError) Match(expectType, expectPath string) bool {
	for _, e := range gr.Errors {
		if e.Type != expectType || !matchPath(e.pathString(), expectPath) {
			return false
		}
	}
	return true
}

func (ge GQLErrorItem) pathString() string {
	var res strings.Builder
	for i, v := range ge.Path {
		if i > 0 {
			res.WriteRune('.')
		}
		fmt.Fprintf(&res, "%v", v)
	}
	return res.String()
}

func matchPath(p, expect string) bool {
	if strings.HasSuffix(expect, ".") {
		return strings.HasPrefix(p, expect) || p == strings.TrimSuffix(expect, ".")
	}
	return p == expect
}

// HandleHTTPError parses a http.Response into a HTTPError.
func HandleHTTPError(resp *http.Response) error {
	httpError := HTTPError{
		Headers:    resp.Header,
		RequestURL: resp.Request.URL,
		StatusCode: resp.StatusCode,
	}

	if !jsonTypeRE.MatchString(resp.Header.Get(contentType)) {
		httpError.Message = resp.Status
		return httpError
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		httpError.Message = err.Error()
		return httpError
	}

	var parsedBody struct {
		Message string `json:"message"`
		Errors  []json.RawMessage
	}
	if err := json.Unmarshal(body, &parsedBody); err != nil {
		return httpError
	}

	var messages []string
	if parsedBody.Message != "" {
		messages = append(messages, parsedBody.Message)
	}
	for _, raw := range parsedBody.Errors {
		switch raw[0] {
		case '"':
			var errString string
			_ = json.Unmarshal(raw, &errString)
			messages = append(messages, errString)
			httpError.Errors = append(httpError.Errors, HTTPErrorItem{Message: errString})
		case '{':
			var errInfo HTTPErrorItem
			_ = json.Unmarshal(raw, &errInfo)
			msg := errInfo.Message
			if errInfo.Code != "" && errInfo.Code != "custom" {
				msg = fmt.Sprintf("%s.%s %s", errInfo.Resource, errInfo.Field, errorCodeToMessage(errInfo.Code))
			}
			if msg != "" {
				messages = append(messages, msg)
			}
			httpError.Errors = append(httpError.Errors, errInfo)
		}
	}
	httpError.Message = strings.Join(messages, "\n")

	return httpError
}

// Convert common error codes to human readable messages
// See https://docs.github.com/en/rest/overview/resources-in-the-rest-api#client-errors for more details.
func errorCodeToMessage(code string) string {
	switch code {
	case "missing", "missing_field":
		return "is missing"
	case "invalid", "unprocessable":
		return "is invalid"
	case "already_exists":
		return "already exists"
	default:
		return code
	}
}
