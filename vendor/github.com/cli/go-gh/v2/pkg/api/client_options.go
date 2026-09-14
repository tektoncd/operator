// Package api is a set of types for interacting with the GitHub API.
package api

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/auth"
	"github.com/cli/go-gh/v2/pkg/config"
)

// ClientOptions holds available options to configure API clients.
type ClientOptions struct {
	// APIHost overrides the hostname that REST and GraphQL API requests are
	// sent to instead of the default Host. It must be a bare
	// hostname, without a scheme or port, for example "api.example.com".
	//
	// When empty, the api_host value configured for Host in gh config is used,
	// if there is one. Client construction fails when the resulting value,
	// whether set here or read from gh config, is not a bare hostname.
	//
	// If AuthToken is not provided, the Host will be used for token lookup,
	// and requests to APIHost will be allowed to include that token. APIHost
	// must therefore be a trusted endpoint. Configuring APIHost does not prevent
	// the token from being sent to Host or its subdomains.
	//
	// Absolute URLs passed to RESTClient methods are requested as given
	// and are never rewritten to APIHost. They are authenticated when they
	// target Host or one of its subdomains, or the exact APIHost.
	APIHost string

	// AuthToken is the authorization token that will be used
	// to authenticate against API endpoints.
	AuthToken string

	// CacheDir is the directory to use for cached API requests.
	// Default is the same directory that gh uses for caching.
	CacheDir string

	// CacheTTL is the time that cached API requests are valid for.
	// Default is 24 hours.
	CacheTTL time.Duration

	// CheckRedirect specifies the policy for handling redirects.
	// If nil, the default http.Client CheckRedirect policy is used.
	//
	// This matters for requests where following a redirect silently changes
	// the meaning of the request. For example, Go's default policy converts a
	// DELETE into a GET when it follows a 301, so a caller deleting a renamed
	// resource can receive a success response having deleted nothing.
	CheckRedirect func(*http.Request, []*http.Request) error

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

func optionsNeedResolution(opts ClientOptions) bool {
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

func resolveOptions(opts ClientOptions) (ClientOptions, error) {
	cfg, _ := config.Read(nil)
	if opts.Host == "" {
		opts.Host, _ = auth.DefaultHost()
	}
	if opts.AuthToken == "" {
		opts.AuthToken, _ = auth.TokenForHost(opts.Host)
		if opts.AuthToken == "" {
			return ClientOptions{}, fmt.Errorf("authentication token not found for host %s", opts.Host)
		}
	}
	if opts.UnixDomainSocket == "" && cfg != nil {
		opts.UnixDomainSocket, _ = cfg.Get([]string{"http_unix_socket"})
	}
	return opts, nil
}

func resolveAPIHost(opts ClientOptions) (ClientOptions, error) {
	if opts.APIHost == "" {
		configuredAPIHost, ok := apiHost(opts.Host)
		if !ok {
			return opts, nil
		}

		opts.APIHost = configuredAPIHost
	}

	if err := validAPIHost(opts.APIHost); err != nil {
		return ClientOptions{}, fmt.Errorf(
			`invalid api_host for %s: %v`,
			opts.Host,
			err,
		)
	}

	return opts, nil
}

func validAPIHost(apiHost string) error {
	invalidHostnameError := fmt.Errorf(`%q must be a hostname without a scheme or port, for example "api.example.com"`, apiHost)

	// A bare hostname has no surrounding whitespace, and no port or IPv6 literal.
	if apiHost == "" || strings.TrimSpace(apiHost) != apiHost || strings.Contains(apiHost, ":") {
		return invalidHostnameError
	}

	// Parsing as a scheme relative URL rejects userinfo, paths, queries and fragments,
	// since any of those make the parsed host differ from the input.
	u, err := url.Parse("//" + apiHost)
	if err != nil {
		return invalidHostnameError
	}
	if u.Host != apiHost || u.Hostname() == "" {
		return invalidHostnameError
	}
	return nil
}
