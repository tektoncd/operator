// Package api is a set of types for interacting with the GitHub API.
package api

import (
	"context"
	"io"
	"net/http"
	"time"
)

// ClientOptions holds available options to configure API clients.
type ClientOptions struct {
	// AuthToken is the authorization token that will be used
	// to authenticate against API endpoints.
	AuthToken string

	// CacheDir is the directory to use for cached API requests.
	// Default is the same directory that gh uses for caching.
	CacheDir string

	// CacheTTL is the time that cached API requests are valid for.
	// Default is 24 hours.
	CacheTTL time.Duration

	// EnableCache specifies if API requests will be cached or not.
	// Default is no caching.
	EnableCache bool

	// Headers are the headers that will be sent with every API request.
	// Default headers set are Accept, Content-Type, Time-Zone, and User-Agent.
	// Default headers will be overridden by keys specified in Headers.
	Headers map[string]string

	// Host is the default host that API requests will be sent to.
	Host string

	// Log specifies a writer to write API request logs to. Default is to respect the GH_DEBUG environment
	// variable, and no logging otherwise.
	Log io.Writer

	// LogIgnoreEnv disables respecting the GH_DEBUG environment variable. This can be useful in test mode
	// or when the extension already offers its own controls for logging to the user.
	LogIgnoreEnv bool

	// LogColorize enables colorized logging to Log for display in a terminal.
	// Default is no coloring.
	LogColorize bool

	// LogVerboseHTTP enables logging HTTP headers and bodies to Log.
	// Default is only logging request URLs and response statuses.
	LogVerboseHTTP bool

	// SkipDefaultHeaders disables setting of the default headers.
	SkipDefaultHeaders bool

	// Timeout specifies a time limit for each API request.
	// Default is no timeout.
	Timeout time.Duration

	// Transport specifies the mechanism by which individual API requests are made.
	// If both Transport and UnixDomainSocket are specified then Transport takes
	// precedence. Due to this behavior any value set for Transport needs to manually
	// handle routing to UnixDomainSocket if necessary. Generally, setting Transport
	// should be reserved for testing purposes.
	// Default is http.DefaultTransport.
	Transport http.RoundTripper

	// UnixDomainSocket specifies the Unix domain socket address by which individual
	// API requests will be routed. If specifed, this will form the base of the API
	// request transport chain.
	// Default is no socket address.
	UnixDomainSocket string
}

// RESTClient is the interface that wraps methods for the different types of
// API requests that are supported by the server.
type RESTClient interface {
	// Do wraps DoWithContext with context.Background.
	Do(method string, path string, body io.Reader, response interface{}) error

	// DoWithContext issues a request with type specified by method to the
	// specified path with the specified body.
	// The response is populated into the response argument.
	DoWithContext(ctx context.Context, method string, path string, body io.Reader, response interface{}) error

	// Delete issues a DELETE request to the specified path.
	// The response is populated into the response argument.
	Delete(path string, response interface{}) error

	// Get issues a GET request to the specified path.
	// The response is populated into the response argument.
	Get(path string, response interface{}) error

	// Patch issues a PATCH request to the specified path with the specified body.
	// The response is populated into the response argument.
	Patch(path string, body io.Reader, response interface{}) error

	// Post issues a POST request to the specified path with the specified body.
	// The response is populated into the response argument.
	Post(path string, body io.Reader, response interface{}) error

	// Put issues a PUT request to the specified path with the specified body.
	// The response is populated into the response argument.
	Put(path string, body io.Reader, response interface{}) error

	// Request wraps RequestWithContext with context.Background.
	Request(method string, path string, body io.Reader) (*http.Response, error)

	// RequestWithContext issues a request with type specified by method to the
	// specified path with the specified body.
	// The response is returned rather than being populated
	// into a response argument.
	RequestWithContext(ctx context.Context, method string, path string, body io.Reader) (*http.Response, error)
}

// GQLClient is the interface that wraps methods for the different types of
// API requests that are supported by the server.
type GQLClient interface {
	// Do wraps DoWithContext using context.Background.
	Do(query string, variables map[string]interface{}, response interface{}) error

	// DoWithContext executes a GraphQL query request.
	// The response is populated into the response argument.
	DoWithContext(ctx context.Context, query string, variables map[string]interface{}, response interface{}) error

	// Mutate wraps MutateWithContext using context.Background.
	Mutate(name string, mutation interface{}, variables map[string]interface{}) error

	// MutateWithContext executes a GraphQL mutation request.
	// The mutation string is derived from the mutation argument, and the
	// response is populated into it.
	// The mutation argument should be a pointer to struct that corresponds
	// to the GitHub GraphQL schema.
	// Provided input will be set as a variable named input.
	MutateWithContext(ctx context.Context, name string, mutation interface{}, variables map[string]interface{}) error

	// Query wraps QueryWithContext using context.Background.
	Query(name string, query interface{}, variables map[string]interface{}) error

	// QueryWithContext executes a GraphQL query request,
	// The query string is derived from the query argument, and the
	// response is populated into it.
	// The query argument should be a pointer to struct that corresponds
	// to the GitHub GraphQL schema.
	QueryWithContext(ctx context.Context, name string, query interface{}, variables map[string]interface{}) error
}
