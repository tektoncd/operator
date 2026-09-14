package api

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/asciisanitizer"
	"github.com/cli/go-gh/v2/pkg/config"
	"github.com/cli/go-gh/v2/pkg/term"
	"github.com/henvic/httpretty"
	"github.com/thlib/go-timezone-local/tzlocal"
	"golang.org/x/text/transform"
)

const (
	accept          = "Accept"
	authorization   = "Authorization"
	apiVersion      = "X-GitHub-Api-Version"
	apiVersionValue = "2022-11-28"
	contentType     = "Content-Type"
	github          = "github.com"
	jsonContentType = "application/json; charset=utf-8"
	localhost       = "github.localhost"
	modulePath      = "github.com/cli/go-gh"
	timeZone        = "Time-Zone"
	userAgent       = "User-Agent"
)

var jsonTypeRE = regexp.MustCompile(`[/+]json($|;)`)

func DefaultHTTPClient() (*http.Client, error) {
	return NewHTTPClient(ClientOptions{})
}

// NewHTTPClient builds a client that can be passed to another library.
//
// As part of the configuration a hostname, auth token, default set of headers,
// and unix domain socket are resolved from the gh environment configuration.
// These behaviors can be overridden using the opts argument. In this instance
// providing opts.Host or opts.APIHost will not change the destination of your
// request, as it is the responsibility of the consumer to configure this. The auth
// token is only added to requests targeting opts.Host or one of its subdomains, or
// the exact opts.APIHost when configured. This prevents tokens from being sent to
// arbitrary hosts.
func NewHTTPClient(opts ClientOptions) (*http.Client, error) {
	var err error
	if optionsNeedResolution(opts) {
		opts, err = resolveOptions(opts)
		if err != nil {
			return nil, err
		}
	}

	opts, err = resolveAPIHost(opts)
	if err != nil {
		return nil, err
	}

	transport := http.DefaultTransport

	if opts.UnixDomainSocket != "" {
		transport = newUnixDomainSocketRoundTripper(opts.UnixDomainSocket)
	}

	if opts.Transport != nil {
		transport = opts.Transport
	}

	transport = newSanitizerRoundTripper(transport)

	if opts.CacheDir == "" {
		opts.CacheDir = config.CacheDir()
	}
	if opts.EnableCache && opts.CacheTTL == 0 {
		opts.CacheTTL = time.Hour * 24
	}
	c := cache{dir: opts.CacheDir, ttl: opts.CacheTTL}
	transport = c.RoundTripper(transport)

	if opts.Log == nil && !opts.LogIgnoreEnv {
		ghDebug := os.Getenv("GH_DEBUG")
		switch ghDebug {
		case "", "0", "false", "no":
			// no logging
		default:
			opts.Log = os.Stderr
			opts.LogColorize = !term.IsColorDisabled() && term.IsTerminal(os.Stderr)
			opts.LogVerboseHTTP = strings.Contains(ghDebug, "api")
		}
	}

	if opts.Log != nil {
		logger := &httpretty.Logger{
			Time:            true,
			TLS:             false,
			Colors:          opts.LogColorize,
			RequestHeader:   opts.LogVerboseHTTP,
			RequestBody:     opts.LogVerboseHTTP,
			ResponseHeader:  opts.LogVerboseHTTP,
			ResponseBody:    opts.LogVerboseHTTP,
			Formatters:      []httpretty.Formatter{&jsonFormatter{colorize: opts.LogColorize}},
			MaxResponseBody: 100000,
		}
		logger.SetOutput(opts.Log)
		logger.SetBodyFilter(func(h http.Header) (skip bool, err error) {
			return !inspectableMIMEType(h.Get(contentType)), nil
		})
		transport = logger.RoundTripper(transport)
	}

	if opts.Headers == nil {
		opts.Headers = map[string]string{}
	}
	if !opts.SkipDefaultHeaders {
		setDefaultHeaders(opts.Headers)
	}
	transport = newHeaderRoundTripper(opts.Host, opts.APIHost, opts.AuthToken, opts.Headers, transport)

	return &http.Client{Transport: transport, Timeout: opts.Timeout, CheckRedirect: opts.CheckRedirect}, nil
}

func inspectableMIMEType(t string) bool {
	return strings.HasPrefix(t, "text/") ||
		strings.HasPrefix(t, "application/x-www-form-urlencoded") ||
		jsonTypeRE.MatchString(t)
}

func isSameDomain(requestHost, domain string) bool {
	requestHost = strings.ToLower(requestHost)
	domain = strings.ToLower(domain)
	return (requestHost == domain) || strings.HasSuffix(requestHost, "."+domain)
}

// swapHost returns rawURL with its host replaced by apiHost.
func swapHost(rawURL, apiHost string) string {
	if apiHost == "" {
		return rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.Host = apiHost
	return u.String()
}

func isGarage(host string) bool {
	return strings.EqualFold(host, "garage.github.com")
}

type headerRoundTripper struct {
	headers map[string]string
	host    string
	apiHost string
	rt      http.RoundTripper
}

func setDefaultHeaders(headers map[string]string) {
	if _, ok := headers[contentType]; !ok {
		headers[contentType] = jsonContentType
	}
	if _, ok := headers[userAgent]; !ok {
		headers[userAgent] = "go-gh"
		info, ok := debug.ReadBuildInfo()
		if ok {
			for _, dep := range info.Deps {
				if dep.Path == modulePath {
					headers[userAgent] += fmt.Sprintf(" %s", dep.Version)
					break
				}
			}
		}
	}
	if _, ok := headers[timeZone]; !ok {
		tz := currentTimeZone()
		if tz != "" {
			headers[timeZone] = tz
		}
	}
	if _, ok := headers[apiVersion]; !ok {
		headers[apiVersion] = apiVersionValue
	}
	if _, ok := headers[accept]; !ok {
		// Preview for PullRequest.mergeStateStatus.
		a := "application/vnd.github.merge-info-preview+json"
		// Preview for visibility when RESTing repos into an org.
		a += ", application/vnd.github.nebula-preview"
		headers[accept] = a
	}
}

func newHeaderRoundTripper(host string, apiHost string, authToken string, headers map[string]string, rt http.RoundTripper) http.RoundTripper {
	if _, ok := headers[authorization]; !ok && authToken != "" {
		headers[authorization] = fmt.Sprintf("token %s", authToken)
	}
	if len(headers) == 0 {
		return rt
	}
	return headerRoundTripper{
		host:    host,
		apiHost: apiHost,
		headers: headers,
		rt:      rt,
	}
}

func (hrt headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range hrt.headers {
		// If the default headers include an authorization header, only add it when
		// the request targets the canonical host or one of its subdomains, or the
		// exact configured API host.
		requestHost := req.URL.Hostname()
		if k == authorization {
			isAPIHost := hrt.apiHost != "" && strings.EqualFold(requestHost, hrt.apiHost)
			if !isSameDomain(requestHost, hrt.host) && !isAPIHost {
				continue
			}
		}

		// If the header is already set in the request, don't overwrite it.
		if req.Header.Get(k) == "" {
			req.Header.Set(k, v)
		}
	}

	return hrt.rt.RoundTrip(req)
}

func newUnixDomainSocketRoundTripper(socketPath string) http.RoundTripper {
	dial := func(network, addr string) (net.Conn, error) {
		return net.Dial("unix", socketPath)
	}

	return &http.Transport{
		Dial:              dial,
		DialTLS:           dial,
		DisableKeepAlives: true,
	}
}

type sanitizerRoundTripper struct {
	rt http.RoundTripper
}

func newSanitizerRoundTripper(rt http.RoundTripper) http.RoundTripper {
	return sanitizerRoundTripper{rt: rt}
}

func (srt sanitizerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := srt.rt.RoundTrip(req)
	if err != nil || !jsonTypeRE.MatchString(resp.Header.Get(contentType)) {
		return resp, err
	}
	sanitizedReadCloser := struct {
		io.Reader
		io.Closer
	}{
		Reader: transform.NewReader(resp.Body, &asciisanitizer.Sanitizer{JSON: true}),
		Closer: resp.Body,
	}
	resp.Body = sanitizedReadCloser
	return resp, err
}

func currentTimeZone() string {
	tz, err := tzlocal.RuntimeTZ()
	if err != nil {
		return ""
	}
	return tz
}
