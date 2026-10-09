// Package vcshost parses and normalises hosted VCS hostnames and matches them
// against an administrator supplied allowlist.
//
// It deliberately has no Pipelines-as-Code dependency, which is why it sits
// outside pkg/provider: the settings package, the secrets package and the
// provider packages all share the exact same parsing rules without any of them
// creating an import cycle.
//
// Normalisation maps a hostname exactly the way a resolver would, by running the
// IDNA lookup profile over the raw input before anything else. A hostname that
// looks like github.com to a human but resolves elsewhere therefore normalises
// to the name that would really be dialled, and is refused. Callers must still
// build every URL from the value Parse returns rather than from the raw input,
// so that policy and DNS can never disagree. See Canonical.
package vcshost

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"golang.org/x/net/idna"
)

const (
	// PublicGitHub is the canonical hostname of the public github.com instance.
	PublicGitHub = "github.com"
	// PublicGitLab is the canonical hostname of the public gitlab.com instance.
	PublicGitLab = "gitlab.com"
	// PublicBitbucket is the canonical hostname of the public bitbucket.org instance.
	PublicBitbucket = "bitbucket.org"
	// PublicGitea is the canonical hostname of the public gitea.com instance.
	PublicGitea = "gitea.com"
	// PublicCodeberg is the canonical hostname of the public codeberg.org instance.
	PublicCodeberg = "codeberg.org"
)

// Public SaaS instances have well known hostnames that whoever sends a webhook
// payload cannot point somewhere else. The map also folds the API hostname of an
// instance onto its canonical hostname, so that api.github.com and github.com
// are treated as the same instance.
var publicHosts = map[string]string{
	PublicGitHub:        PublicGitHub,
	"api.github.com":    PublicGitHub,
	PublicGitLab:        PublicGitLab,
	PublicBitbucket:     PublicBitbucket,
	"api.bitbucket.org": PublicBitbucket,
	PublicGitea:         PublicGitea,
	PublicCodeberg:      PublicCodeberg,
}

// idnaProfile maps unicode hostnames to their punycode form the same way a
// resolver would, so that an allowlist entry written in unicode matches the
// punycode hostname a provider puts in its payloads.
var idnaProfile = idna.New(idna.MapForLookup(), idna.StrictDomainName(false), idna.BidiRule())

// IsPublic reports whether host is a well known public SaaS instance. The host
// is expected to have been normalised by Parse first.
func IsPublic(host string) bool {
	_, ok := publicHosts[strings.ToLower(host)]
	return ok
}

// Canonical returns the canonical hostname for a public SaaS instance, so that
// api.github.com and github.com are treated as the same instance.
//
// Callers must build every URL from this value and never from the hostname they
// were given: this normalisation is what makes a lookalike hostname collapse
// onto the host it imitates instead of being dialled as written.
func Canonical(host string) string {
	if canonical, ok := publicHosts[strings.ToLower(host)]; ok {
		return canonical
	}
	return strings.ToLower(host)
}

// privateSuffixes are the DNS suffixes reserved for names that only resolve
// inside a cluster, a private network or a single machine.
var privateSuffixes = []string{
	".localhost",
	".local",
	".internal",
	".intranet",
	".home.arpa",
	".svc",
	".cluster.local",
}

// IsPrivate reports whether host designates something that is not reachable on
// the public internet: loopback, link local (which includes the cloud metadata
// endpoint), a private range, an unqualified name or an in cluster Kubernetes
// service name. Such a host may well be legitimate, but it must only ever be
// trusted because an administrator listed it explicitly, never because a webhook
// payload mentioned it.
func IsPrivate(host string) bool {
	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	}
	hostname = strings.ToLower(strings.TrimSuffix(strings.Trim(hostname, "[]"), "."))

	if ip := net.ParseIP(hostname); ip != nil {
		return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
			ip.IsPrivate() || ip.IsUnspecified()
	}
	// net.ParseIP only accepts the canonical dotted quad, but the C resolver
	// behind cgo also accepts the inet_aton spellings, where 127.1, 0177.0.0.1
	// and 0x7f.1 all reach the loopback address. Those never designate a real
	// hostname, so refuse to learn any of them automatically.
	if isNumericAddress(hostname) {
		return true
	}
	if !strings.Contains(hostname, ".") {
		return true
	}
	if hostname == "localhost" {
		return true
	}
	for _, suffix := range privateSuffixes {
		if strings.HasSuffix(hostname, suffix) {
			return true
		}
	}
	// <service>.<namespace>.svc.<cluster domain>, whatever the cluster domain is.
	return strings.Contains(hostname, ".svc.")
}

// isNumericAddress reports whether every label of hostname is a decimal, octal
// or hexadecimal number, which is what makes inet_aton read it as an IP address
// rather than as a name.
func isNumericAddress(hostname string) bool {
	labels := strings.Split(hostname, ".")
	for _, label := range labels {
		if label == "" {
			return false
		}
		digits := label
		if after, found := strings.CutPrefix(label, "0x"); found {
			digits = after
			if digits == "" {
				return false
			}
			for _, r := range digits {
				if !strings.ContainsRune("0123456789abcdef", r) {
					return false
				}
			}
			continue
		}
		for _, r := range digits {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

// ErrURLUnsafeComponents is returned by SplitURL for a URL carrying
// credentials, a query or a fragment. Callers that speak of a hostname rather
// than of a URL match on it to word the refusal in their own terms.
var ErrURLUnsafeComponents = errors.New("provider URL must not contain credentials, a query or a fragment")

// SplitURL parses a provider URL the way an administrator is allowed to write
// it: bare (ghe.example.com), or with a scheme (https://ghe.example.com/gitlab).
// A value without a scheme is read as https, which is what every caller wants
// for a hostname typed by hand.
//
// It only parses and rejects a URL that names no host or carries credentials, a
// query or a fragment, all of which are signs the value was not meant to be a
// provider endpoint. Deciding which schemes are acceptable is left to the
// caller: a self hosted instance reachable over plain http is legitimate on the
// paths that build a client, and refused on the paths that only take a hostname.
func SplitURL(rawURL string) (*url.URL, error) {
	raw := strings.TrimSpace(rawURL)
	if raw == "" {
		return nil, fmt.Errorf("provider URL is empty")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("invalid provider URL %q", rawURL)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, ErrURLUnsafeComponents
	}
	return parsed, nil
}

// Parse validates a hostname and returns it normalised. The value may be given
// bare (ghe.example.com) or as an https URL (https://ghe.example.com). Anything
// carrying a path, a query, a fragment, credentials or a non https scheme is
// rejected: those are signs the value did not come from a trusted source.
func Parse(rawHost string) (string, error) {
	if strings.TrimSpace(rawHost) == "" {
		return "", fmt.Errorf("hostname is empty")
	}
	parsed, err := SplitURL(rawHost)
	if err != nil {
		if errors.Is(err, ErrURLUnsafeComponents) {
			return "", fmt.Errorf("hostname must not contain credentials, query or fragment")
		}
		return "", fmt.Errorf("invalid hostname")
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return "", fmt.Errorf("hostname scheme must be https, got %q", parsed.Scheme)
	}
	if path := strings.TrimSuffix(parsed.EscapedPath(), "/"); path != "" {
		return "", fmt.Errorf("hostname must not contain a path")
	}
	return normalise(parsed.Hostname(), parsed.Port())
}

// normalise drops the root label trailing dot, maps the hostname to the punycode
// form a resolver would look up and lowercases the result, so that two spellings
// of the same hostname always produce the same string.
//
// The IDNA mapping runs on the raw hostname, before any case folding of our own.
// strings.ToLower applies simple case mapping, which for a handful of runes
// (U+0130 LATIN CAPITAL LETTER I WITH DOT ABOVE, for one) differs from the full
// mapping a resolver applies: lowercasing first would classify GITHUB.com spelt
// with that rune as github.com while net/http dialled xn--github-qyd.com.
func normalise(hostname, port string) (string, error) {
	hostname = strings.TrimSuffix(hostname, ".")
	if hostname == "" {
		return "", fmt.Errorf("invalid hostname")
	}
	isIP := net.ParseIP(hostname) != nil
	if !isIP {
		ascii, err := idnaProfile.ToASCII(hostname)
		if err != nil {
			return "", fmt.Errorf("invalid hostname %q: %w", hostname, err)
		}
		hostname = strings.ToLower(ascii)
		// The mapping must be a fixed point, otherwise the value we hand back
		// would still normalise to something else further down the stack.
		if again, err := idnaProfile.ToASCII(hostname); err != nil || !strings.EqualFold(again, hostname) {
			return "", fmt.Errorf("hostname %q does not normalise to a stable form", hostname)
		}
		if !validDNSHostname(hostname) {
			return "", fmt.Errorf("hostname %q normalises to an invalid DNS hostname", hostname)
		}
	}
	if port != "" {
		return net.JoinHostPort(hostname, port), nil
	}
	if isIP && strings.Contains(hostname, ":") {
		return "[" + hostname + "]", nil
	}
	return hostname, nil
}

func validDNSHostname(hostname string) bool {
	if len(hostname) > 253 {
		return false
	}
	for label := range strings.SplitSeq(hostname, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
				return false
			}
		}
	}
	return true
}

// ParseAllowlist splits a comma separated list of hostnames into normalised
// canonical hostnames. An empty value yields an empty list, which means the
// allowlist has not been configured.
func ParseAllowlist(raw string) ([]string, error) {
	var hosts []string
	seen := map[string]bool{}
	for entry := range strings.SplitSeq(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		host, err := Parse(entry)
		if err != nil {
			return nil, fmt.Errorf("invalid hostname %q: %w", entry, err)
		}
		host = Canonical(host)
		if seen[host] {
			continue
		}
		seen[host] = true
		hosts = append(hosts, host)
	}
	return hosts, nil
}

// Allowed reports whether host appears in the allowlist. Both sides are
// canonicalised so that github.com and api.github.com match each other.
func Allowed(allowlist []string, host string) bool {
	canonical := Canonical(host)
	for _, allowed := range allowlist {
		if Canonical(allowed) == canonical {
			return true
		}
	}
	return false
}

// Join renders an allowlist back into the comma separated ConfigMap value.
func Join(allowlist []string) string {
	return strings.Join(allowlist, ",")
}
