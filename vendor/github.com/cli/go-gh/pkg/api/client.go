// Package api is a set of types for GitHub API.
package api

import (
	"io"
	"net/http"
	"time"
)

// Available options to configure API clients.
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
	// Default headers set are Accept, Authorization, Content-Type, Time-Zone, and User-Agent.
	// Default headers will be overridden by keys specified in Headers.
	Headers map[string]string

	// Host is the host that every API request will be sent to.
	Host string

	// Log specifies a writer to write API request logs to.
	// Default is no logging.
	Log io.Writer

	// Timeout specifies a time limit for each API request.
	// Default is no timeout.
	Timeout time.Duration

	// Transport specifies the mechanism by which individual API requests are made.
	// Default is http.DefaultTransport.
	Transport http.RoundTripper
}

// RESTClient is the interface that wraps methods for the different types of
// API requests that are supported by the server.
type RESTClient interface {
	// Do issues a request with type specified by method to the
	// specified path with the specified body.
	// The response is populated into the response argument.
	Do(method string, path string, body io.Reader, response interface{}) error

	// Delete issues a DELETE request to the specified path.
	// The response is populated into the response argument.
	Delete(path string, response interface{}) error

	// GET issues a GET request to the specified path.
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
}

// GQLClient is the interface that wraps methods for the different types of
// API requests that are supported by the server.
type GQLClient interface {
	// Do executes a GraphQL query request.
	// The response is populated into the response argument.
	Do(query string, variables map[string]interface{}, response interface{}) error

	// Mutate executes a GraphQL mutation request.
	// The mutation string is derived from the mutation argument, and the
	// response is populated into it.
	// The mutation argument should be a pointer to struct that corresponds
	// to the GitHub GraphQL schema.
	// Provided input will be set as a variable named input.
	Mutate(name string, mutation interface{}, variables map[string]interface{}) error

	// Query executes a GraphQL query request,
	// The query string is derived from the query argument, and the
	// response is populated into it.
	// The query argument should be a pointer to struct that corresponds
	// to the GitHub GraphQL schema.
	Query(name string, query interface{}, variables map[string]interface{}) error
}
