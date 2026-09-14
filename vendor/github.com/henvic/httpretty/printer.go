package httpretty

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"maps"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/henvic/httpretty/internal/color"
	"github.com/henvic/httpretty/internal/header"
)

func newPrinter(l *Logger) printer {
	l.mu.Lock()
	defer l.mu.Unlock()
	return printer{
		logger:  l,
		flusher: l.flusher,
	}
}

type printer struct {
	flusher Flusher
	logger  *Logger
	buf     bytes.Buffer
}

func (p *printer) maybeOnReady() {
	if p.flusher == OnReady {
		p.flush()
	}
}

func (p *printer) flush() {
	if p.flusher == NoBuffer {
		return
	}
	p.logger.mu.Lock()
	defer p.logger.mu.Unlock()
	defer p.buf.Reset()
	w := p.logger.getWriter()
	fmt.Fprint(w, p.buf.String())
}

func (p *printer) print(a ...any) {
	p.logger.mu.Lock()
	defer p.logger.mu.Unlock()
	w := p.logger.getWriter()
	if p.flusher == NoBuffer {
		fmt.Fprint(w, a...)
		return
	}
	fmt.Fprint(&p.buf, a...)
}

func (p *printer) println(a ...any) {
	p.logger.mu.Lock()
	defer p.logger.mu.Unlock()
	w := p.logger.getWriter()
	if p.flusher == NoBuffer {
		fmt.Fprintln(w, a...)
		return
	}
	fmt.Fprintln(&p.buf, a...)
}

func (p *printer) printf(format string, a ...any) {
	p.logger.mu.Lock()
	defer p.logger.mu.Unlock()
	w := p.logger.getWriter()
	if p.flusher == NoBuffer {
		fmt.Fprintf(w, format, a...)
		return
	}
	fmt.Fprintf(&p.buf, format, a...)
}

func (p *printer) printRequest(req *http.Request) {
	if p.logger.RequestHeader {
		p.printRequestHeader(req)
		p.maybeOnReady()
	}
	if p.logger.RequestBody && req.Body != nil {
		p.printRequestBody(req)
		p.maybeOnReady()
	}
}

func (p *printer) printRequestInfo(req *http.Request) {
	to := req.URL.String()
	// req.URL.Host is empty on the request received by a server
	if req.URL.Host == "" {
		to = req.Host + to
		schema := "http://"
		if req.TLS != nil {
			schema = "https://"
		}
		to = schema + to
	}
	p.printf("* Request to %s\n", p.format(color.FgBlue, to))
	if req.RemoteAddr != "" {
		p.printf("* Request from %s\n", p.format(color.FgBlue, req.RemoteAddr))
	}
}

// checkFilter checks if the request is filtered and if the Request value is nil.
func (p *printer) checkFilter(req *http.Request) (skip bool) {
	filter := p.logger.getFilter()
	if req == nil {
		p.printf("> %s\n", p.format(color.FgRed, "error: null request"))
		return true
	}
	if filter == nil {
		return false
	}
	ok, err := safeFilter(filter, req)
	if err != nil {
		p.printf("* cannot filter request: %s: %s\n", p.format(color.FgBlue, fmt.Sprintf("%s %s", req.Method, req.URL)), p.format(color.FgRed, err.Error()))
		return false // never filter out the request if the filter errored
	}
	return ok
}

func safeFilter(filter Filter, req *http.Request) (skip bool, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("panic: %v", e)
		}
	}()
	return filter(req)
}

func (p *printer) printResponse(resp *http.Response) {
	if resp == nil {
		p.printf("< %s\n", p.format(color.FgRed, "error: null response"))
		p.maybeOnReady()
		return
	}
	if p.logger.ResponseHeader {
		p.printResponseHeader(resp.Proto, resp.Status, resp.Header)
		p.maybeOnReady()
	}

	// The client only fills resp.Trailer once the body is read to EOF by httpretty.
	// When the body is left unread, too large, binary, or filtered we don't capture trailers.
	var readToEnd bool
	if p.logger.ResponseBody && resp.Body != nil && (resp.Request == nil || resp.Request.Method != http.MethodHead) {
		readToEnd = p.printResponseBodyOut(resp)
		p.maybeOnReady()
	}
	if p.logger.ResponseHeader && len(resp.Trailer) > 0 {
		switch {
		case hasTrailerValues(resp.Trailer):
			p.printTrailers('<', resp.Trailer)
			p.maybeOnReady()
		case !readToEnd:
			p.printf("* %s\n", p.format(color.FgBlue, "trailers announced but not captured"))
			p.maybeOnReady()
		}
	}
}

func (p *printer) checkBodyFiltered(h http.Header) (skip bool, err error) {
	if f := p.logger.getBodyFilter(); f != nil {
		defer func() {
			if e := recover(); e != nil {
				p.printf("* panic while filtering body: %v\n", e)
			}
		}()
		return f(h)
	}
	return false, nil
}

// printResponseBodyOut prints the client response body and reports whether the
// body was read to EOF.
func (p *printer) printResponseBodyOut(resp *http.Response) (readToEnd bool) {
	if resp.ContentLength == 0 {
		return true
	}
	skip, err := p.checkBodyFiltered(resp.Header)
	if err != nil {
		p.printf("* %s\n", p.format(color.FgRed, "error on response body filter: ", err.Error()))
	}
	if skip {
		return false
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != "" && isBinaryMediatype(contentType) {
		p.println("* body contains binary data")
		return false
	}
	if p.logger.MaxResponseBody > 0 && resp.ContentLength > p.logger.MaxResponseBody {
		p.printf("* body is too long (%d bytes) to print, skipping (longer than %d bytes)\n", resp.ContentLength, p.logger.MaxResponseBody)
		return false
	}
	contentType := resp.Header.Get("Content-Type")
	if resp.ContentLength == -1 {
		newBody, readToEnd := p.printBodyUnknownLength(contentType, p.logger.MaxResponseBody, resp.Body)
		if newBody != nil {
			resp.Body = newBody
		}
		return readToEnd
	}
	var buf bytes.Buffer
	tee := io.TeeReader(resp.Body, &buf)
	defer resp.Body.Close()
	defer func() {
		resp.Body = io.NopCloser(&buf)
	}()
	p.printBodyReader(contentType, tee)
	return true
}

// isBinary uses heuristics to guess if file is binary (actually, "printable" in the terminal).
// See discussion at https://groups.google.com/forum/#!topic/golang-nuts/YeLL7L7SwWs
func isBinary(body []byte) bool {
	if len(body) > 512 {
		body = body[:512]
	}
	// If file contains UTF-8 OR UTF-16 BOM, consider it non-binary.
	// Reference: https://tools.ietf.org/html/draft-ietf-websec-mime-sniff-03#section-5
	if len(body) >= 3 && (bytes.Equal(body[:2], []byte{0xFE, 0xFF}) || // UTF-16BE BOM
		bytes.Equal(body[:2], []byte{0xFF, 0xFE}) || // UTF-16LE BOM
		bytes.Equal(body[:3], []byte{0xEF, 0xBB, 0xBF})) { // UTF-8 BOM
		return false
	}
	// If all of the first n octets are binary data octets, consider it binary.
	// Reference: https://github.com/golang/go/blob/349e7df2c3d0f9b5429e7c86121499c137faac7e/src/net/http/sniff.go#L297-L309
	// c.f. section 5, step 4.
	for _, b := range body {
		switch {
		case b <= 0x08,
			b == 0x0B,
			0x0E <= b && b <= 0x1A,
			0x1C <= b && b <= 0x1F:
			return true
		}
	}
	// Otherwise, check against a white list of binary mimetypes.
	mediatype, _, err := mime.ParseMediaType(http.DetectContentType(body))
	if err != nil {
		return false
	}
	return isBinaryMediatype(mediatype)
}

var binaryMediatypes = map[string]struct{}{
	"application/pdf":               {},
	"application/postscript":        {},
	"image":                         {}, // for practical reasons, any image (including SVG) is considered binary data
	"audio":                         {},
	"application/ogg":               {},
	"video":                         {},
	"application/vnd.ms-fontobject": {},
	"font":                          {},
	"application/gzip":              {},
	"application/x-gzip":            {},
	"application/zip":               {},
	"application/x-rar-compressed":  {},
	"application/wasm":              {},
}

func isBinaryMediatype(mediatype string) bool {
	if _, ok := binaryMediatypes[mediatype]; ok {
		return true
	}
	if parts := strings.SplitN(mediatype, "/", 2); len(parts) == 2 {
		if _, ok := binaryMediatypes[parts[0]]; ok {
			return true
		}
	}
	return false
}

const maxDefaultUnknownReadable = 4096 // bytes

// printBodyUnknownLength is used for (tentatively) printing a body of unknown length.
func (p *printer) printBodyUnknownLength(contentType string, maxLength int64, r io.ReadCloser) (newBody io.ReadCloser, readToEnd bool) {
	if maxLength == 0 {
		maxLength = maxDefaultUnknownReadable
	}
	pb := make([]byte, maxLength+1) // read one extra bit to assure the length is longer than acceptable
	n, err := io.ReadFull(r, pb)
	pb = pb[0:n] // trim any nil symbols left after writing in the byte slice.
	buf := bytes.NewReader(pb)
	newBody = newBodyReaderBuf(buf, r)
	switch {
	// Server requests always return req.Body != nil, but the Reader returns io.EOF immediately.
	// Avoiding returning early to mitigate any risk of bad reader implementations that might
	// send something even after returning io.EOF if read again.
	case err == io.EOF && n == 0:
		readToEnd = true
	case err == nil && int64(n) > maxLength:
		p.printf("* body is too long, skipping (contains more than %d bytes)\n", n-1)
	case err == io.ErrUnexpectedEOF || err == nil:
		// cannot pass same bytes reader below because we only read it once.
		readToEnd = true
		p.printBodyReader(contentType, bytes.NewReader(pb))
	default:
		p.printf("* cannot read body: %v (%d bytes read)\n", err, n)
	}
	return
}

func findPeerCertificate(hostname string, state *tls.ConnectionState) (cert *x509.Certificate) {
	if chains := state.VerifiedChains; chains != nil && chains[0] != nil && chains[0][0] != nil {
		return chains[0][0]
	}
	if hostname == "" && len(state.PeerCertificates) > 0 {
		// skip finding a match for a given hostname if hostname is not available (e.g., a client certificate)
		return state.PeerCertificates[0]
	}
	// the chain is not created when tls.Config.InsecureSkipVerify is set, then let's try to find a match to display
	for _, cert := range state.PeerCertificates {
		if err := cert.VerifyHostname(hostname); err == nil {
			return cert
		}
	}
	return nil
}

func (p *printer) printTLSInfo(state *tls.ConnectionState, skipVerifyChains bool) {
	if state == nil {
		return
	}
	protocol := tls.VersionName(state.Version)
	cipher := tls.CipherSuiteName(state.CipherSuite)
	p.printf("* TLS connection using %s / %s", p.format(color.FgBlue, protocol), p.format(color.FgBlue, cipher))
	if !skipVerifyChains && state.VerifiedChains == nil {
		p.print(" (insecure=true)")
	}
	p.println()
	if state.NegotiatedProtocol != "" {
		p.printf("* ALPN: %v accepted\n", p.format(color.FgBlue, state.NegotiatedProtocol))
	}
}

func (p *printer) printOutgoingClientTLS(config *tls.Config) {
	if config == nil || len(config.Certificates) == 0 {
		return
	}
	p.println("* Client certificate:")
	// Please notice tls.Config.BuildNameToCertificate() doesn't store the certificate Leaf field.
	// You need to explicitly parse and store it with something such as:
	// cert.Leaf, err = x509.ParseCertificate(cert.Certificate)
	if cert := config.Certificates[0].Leaf; cert != nil {
		p.printCertificate("", cert)
	} else {
		p.println(`** unparsed certificate found, skipping`)
	}
}

func (p *printer) printIncomingClientTLS(state *tls.ConnectionState) {
	// if no TLS state is null or no client TLS certificate is found, return early.
	if state == nil || len(state.PeerCertificates) == 0 {
		return
	}
	p.println("* Client certificate:")
	if cert := findPeerCertificate("", state); cert != nil {
		p.printCertificate("", cert)
	} else {
		p.println(p.format(color.FgRed, "** No valid certificate was found"))
	}
}

func (p *printer) printTLSServer(host string, state *tls.ConnectionState) {
	if state == nil {
		return
	}
	hostname, _, err := net.SplitHostPort(host)
	if err != nil {
		// assume the error is due to "missing port in address"
		hostname = host
	}
	p.println("* Server certificate:")
	if cert := findPeerCertificate(hostname, state); cert != nil {
		// server certificate messages are slightly similar to how "curl -v" shows
		p.printCertificate(hostname, cert)
	} else {
		p.println(p.format(color.FgRed, "** No valid certificate was found"))
	}
}

func (p *printer) printCertificate(hostname string, cert *x509.Certificate) {
	p.printf(`*  subject: %v
*  start date: %v
*  expire date: %v
`,
		p.format(color.FgBlue, cert.Subject),
		p.format(color.FgBlue, cert.NotBefore.Format(time.UnixDate)),
		p.format(color.FgBlue, cert.NotAfter.Format(time.UnixDate)),
	)
	if hostname != "" {
		if san, ok := matchedSAN(hostname, cert); ok {
			if san == "" {
				p.printf("*  subjectAltName: \"%s\" matches cert's IP address!\n",
					p.format(color.FgBlue, hostname))
			} else {
				p.printf("*  subjectAltName: \"%s\" matches cert's \"%s\"\n",
					p.format(color.FgBlue, hostname),
					p.format(color.FgBlue, san))
			}
		}
	}
	p.printf("*  issuer: %v\n", p.format(color.FgBlue, cert.Issuer))
	if hostname == "" {
		return
	}
	if err := cert.VerifyHostname(hostname); err != nil {
		p.printf("*  %s\n", p.format(color.FgRed, err.Error()))
		return
	}
	p.println("*  TLS certificate verify ok.")
}

// matchedSAN finds the cert SAN entry that matches hostname, following the
// RFC 6125 wildcard rule (leftmost label only). For IP-literal hostnames it
// scans IPAddresses and returns "" with ok=true to signal an IP match.
func matchedSAN(hostname string, cert *x509.Certificate) (string, bool) {
	if ip := net.ParseIP(hostname); ip != nil {
		for _, certIP := range cert.IPAddresses {
			if certIP.Equal(ip) {
				return "", true
			}
		}
		return "", false
	}
	host := strings.TrimSuffix(strings.ToLower(hostname), ".")
	for _, name := range cert.DNSNames {
		if matchHostname(strings.ToLower(name), host) {
			return name, true
		}
	}
	return "", false
}

func matchHostname(pattern, host string) bool {
	pattern = strings.TrimSuffix(pattern, ".")
	if pattern == "" || host == "" {
		return false
	}
	patternParts := strings.Split(pattern, ".")
	hostParts := strings.Split(host, ".")
	if len(patternParts) != len(hostParts) {
		return false
	}
	for i, part := range patternParts {
		if i == 0 && part == "*" {
			continue
		}
		if part != hostParts[i] {
			return false
		}
	}
	return true
}

// printServerResponse prints the headers the handler set.
// Naturally, we do not capture anything added later, such as Date.
func (p *printer) printServerResponse(req *http.Request, rec *responseRecorder) {
	var trailers http.Header
	if p.logger.ResponseHeader {
		var headers http.Header
		headers, trailers = splitTrailers(rec.Header())
		p.printResponseHeader(req.Proto, fmt.Sprintf("%d %s", rec.statusCode, http.StatusText(rec.statusCode)), headers)
	}
	p.printServerResponseBody(rec)
	if p.logger.ResponseHeader && hasTrailerValues(trailers) {
		p.printTrailers('<', trailers)
	}
}

func (p *printer) printServerResponseBody(rec *responseRecorder) {
	if !p.logger.ResponseBody || rec.size == 0 {
		return
	}
	skip, err := p.checkBodyFiltered(rec.Header())
	if err != nil {
		p.printf("* %s\n", p.format(color.FgRed, "error on response body filter: ", err.Error()))
	}
	if skip {
		return
	}
	if mediatype := rec.Header().Get("Content-Type"); mediatype != "" && isBinaryMediatype(mediatype) {
		p.println("* body contains binary data")
		return
	}
	if p.logger.MaxResponseBody > 0 && rec.size > p.logger.MaxResponseBody {
		p.printf("* body is too long (%d bytes) to print, skipping (longer than %d bytes)\n", rec.size, p.logger.MaxResponseBody)
		return
	}
	p.printBodyReader(rec.Header().Get("Content-Type"), rec.buf)
}

// statusColor returns color attributes for an HTTP status line
// based on the status class:
// 1xx (informational), 2xx (success) is green, 3xx (redirection) is yellow,
// 4xx (client error) is red, and 5xx (server error) is bold red.
// Any non-standard classes (0xx, 6xx-9xx) are blue,
// and an empty status or one that doesn't start with a digit, is shown red.
func statusColor(status string) []color.Attribute {
	if len(status) == 0 {
		return []color.Attribute{color.FgRed}
	}
	switch status[0] {
	case '2':
		return []color.Attribute{color.FgGreen}
	case '3':
		return []color.Attribute{color.FgYellow}
	case '4':
		return []color.Attribute{color.FgRed}
	case '5':
		return []color.Attribute{color.Bold, color.FgRed}
	case '0', '1', '6', '7', '8', '9':
		return []color.Attribute{color.FgBlue}
	default:
		return []color.Attribute{color.FgRed}
	}
}

func (p *printer) printResponseHeader(proto, status string, h http.Header) {
	p.printf("< %s %s\n",
		p.format(color.FgBlue, color.Bold, proto),
		p.format(statusColor(status), status))
	p.printHeaders('<', h)
	p.println()
}

// printTrailers that are sent after the body.
func (p *printer) printTrailers(prefix rune, h http.Header) {
	p.printf("%c Trailers:\n", prefix)
	p.printHeaders(prefix, h)
	p.println()
}

// hasTrailerValues reports whether h carries at least one non-empty value.
// The HTTP client pre-populates resp.Trailer with nil values for each key
// declared in the response's Trailer header before the body is read to EOF,
// so a non-zero len(resp.Trailer) alone does not mean any trailer value is
// actually available to print.
func hasTrailerValues(h http.Header) bool {
	for _, v := range h {
		if len(v) > 0 {
			return true
		}
	}
	return false
}

// splitTrailers separates a handled server response's recorded header map into
// the headers sent in the header block and the trailers sent after the body.
//
// An http.Server emits trailers two ways, both reconstructed here the same way
// httptest.ResponseRecorder.Result builds its Trailer: keys announced ahead of
// time in the "Trailer" header, and keys written with the http.TrailerPrefix
// magic prefix. Both otherwise linger in the ResponseWriter header map, so they
// are kept out of headers to avoid printing them as if they were sent with the
// header block. The "Trailer" announcement header itself is left in headers.
//
// In the common case where h carries no trailers, h itself is returned along
// with nil trailers, so callers must treat both maps as read-only.
func splitTrailers(h http.Header) (headers, trailers http.Header) {
	var announced map[string]struct{}
	for _, list := range h["Trailer"] {
		for key := range strings.SplitSeq(list, ",") {
			if key = http.CanonicalHeaderKey(strings.TrimSpace(key)); key != "" {
				if announced == nil {
					announced = map[string]struct{}{}
				}
				announced[key] = struct{}{}
			}
		}
	}
	split := false
	for key := range h {
		if _, ok := announced[key]; ok {
			split = true
			break
		}
		if strings.HasPrefix(key, http.TrailerPrefix) {
			split = true
			break
		}
	}
	if !split {
		return h, nil
	}
	headers = http.Header{}
	trailers = http.Header{}
	for key, vv := range h {
		if _, ok := announced[key]; ok {
			trailers[key] = vv
			continue
		}
		if name, ok := strings.CutPrefix(key, http.TrailerPrefix); ok {
			for _, v := range vv {
				trailers.Add(name, v)
			}
			continue
		}
		headers[key] = vv
	}
	return headers, trailers
}

func (p *printer) printBodyReader(contentType string, r io.Reader) {
	body, err := io.ReadAll(r)
	if err != nil {
		p.printf("* cannot read body: %v\n", p.format(color.FgRed, err.Error()))
		return
	}
	p.printBody(contentType, body, 0)
}

// maxMultipartDepth bounds how deep nested multipart bodies are split into parts;
// deeper parts are printed as a regular body. Without a bound, a maliciously nested
// multipart body of n bytes would amplify to O(n²) memory, as splitting each level
// copies all the levels nested under it.
const maxMultipartDepth = 2

// printBody of a message or of a part of a multipart message, nested depth levels deep.
func (p *printer) printBody(contentType string, body []byte, depth int) {
	mediatype, params, _ := mime.ParseMediaType(contentType)
	f := p.formatter(mediatype)
	// A multipart body is printed part by part, unless a formatter handles it.
	if f == nil && depth < maxMultipartDepth && strings.HasPrefix(mediatype, "multipart/") && params["boundary"] != "" &&
		p.printMultipart(mediatype, params["boundary"], body, depth) {
		return
	}
	if isBinary(body) {
		p.println("* body contains binary data")
		return
	}
	if f == nil {
		p.println(string(body))
		return
	}
	var formatted bytes.Buffer
	if err := p.safeBodyFormat(f, &formatted, body); err != nil {
		p.printf("* body cannot be formatted: %v\n%s\n", p.format(color.FgRed, err.Error()), string(body))
		return
	}
	p.println(formatted.String())
}

// formatter returns the first formatter matching the media type, if any.
func (p *printer) formatter(mediatype string) Formatter {
	for _, f := range p.logger.Formatters {
		if p.safeBodyMatch(f, mediatype) {
			return f
		}
	}
	return nil
}

// printMultipart prints each part of a multipart body on its own, so parts are
// formatted, and checked for binary content, individually.
//
// It reports whether the body was printed. A body that cannot be parsed (say, a
// truncated one) is left for the caller to print as-is, and nothing is printed here.
func (p *printer) printMultipart(mediatype, boundary string, body []byte, depth int) bool {
	type bodyPart struct {
		header http.Header
		body   []byte
	}
	var parts []bodyPart
	mr := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		next, err := mr.NextRawPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false
		}
		b, err := io.ReadAll(next)
		if err != nil {
			return false
		}
		parts = append(parts, bodyPart{
			header: http.Header(next.Header),
			body:   b,
		})
	}
	if len(parts) == 0 {
		return false
	}
	noun := "parts"
	if len(parts) == 1 {
		noun = "part"
	}
	p.printf("* %s body with %d %s\n", mediatype, len(parts), noun)
	for i, part := range parts {
		p.printf("* part %d\n", i+1)
		p.printHeaders('|', part.header)
		if len(part.body) == 0 {
			continue
		}
		p.println()
		p.printBody(part.header.Get("Content-Type"), part.body, depth+1)
	}
	return true
}

func (p *printer) safeBodyMatch(f Formatter, mediatype string) bool {
	defer func() {
		if e := recover(); e != nil {
			p.printf("* panic while testing body format: %v\n", e)
		}
	}()
	return f.Match(mediatype)
}

func (p *printer) safeBodyFormat(f Formatter, w io.Writer, src []byte) (err error) {
	defer func() {
		// should not return panic as error because we want to try the next formatter
		if e := recover(); e != nil {
			err = fmt.Errorf("panic: %v", e)
		}
	}()
	return f.Format(w, src)
}

func (p *printer) format(s ...any) string {
	if p.logger.Colors {
		return color.Format(s...)
	}
	return color.StripAttributes(s...)
}

func (p *printer) printHeaders(prefix rune, h http.Header) {
	if !p.logger.SkipSanitize {
		h = header.Sanitize(header.DefaultSanitizers, h)
	}

	longest, sorted := sortHeaderKeys(h, p.logger.cloneSkipHeader())
	for _, key := range sorted {
		for _, v := range h[key] {
			var pad string
			if p.logger.Align {
				pad = strings.Repeat(" ", longest-len(key))
			}
			p.printf("%c %s%s %s%s\n", prefix,
				p.format(color.FgBlue, color.Bold, key),
				p.format(color.FgRed, ":"),
				pad,
				p.format(color.FgYellow, v))
		}
	}
}

func sortHeaderKeys(h http.Header, skipped map[string]struct{}) (int, []string) {
	var (
		keys    = make([]string, 0, len(h))
		longest int
	)
	for key := range h {
		if _, skip := skipped[key]; skip {
			continue
		}
		keys = append(keys, key)
		if l := len(key); l > longest {
			longest = l
		}
	}
	sort.Strings(keys)
	if i := slices.Index(keys, "Host"); i > -1 {
		keys = append([]string{"Host"}, slices.Delete(keys, i, i+1)...)
	}
	return longest, keys
}

func (p *printer) printRequestHeader(req *http.Request) {
	p.printf("> %s %s %s\n",
		p.format(color.FgBlue, color.Bold, req.Method),
		p.format(color.FgYellow, req.URL.RequestURI()),
		p.format(color.FgBlue, req.Proto))
	p.printHeaders('>', addRequestHeaders(req))
	p.println()
}

// addRequestHeaders returns a copy of the given header with an additional headers set, if known.
func addRequestHeaders(req *http.Request) http.Header {
	cp := http.Header{}
	maps.Copy(cp, req.Header)

	if len(req.Header.Values("Content-Length")) == 0 && req.ContentLength > 0 {
		cp.Set("Content-Length", fmt.Sprintf("%d", req.ContentLength))
	}

	if len(req.Header.Values("Transfer-Encoding")) == 0 && len(req.TransferEncoding) > 0 {
		cp.Set("Transfer-Encoding", strings.Join(req.TransferEncoding, ", "))
	}

	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	if host != "" {
		cp.Set("Host", host)
	}
	return cp
}

func (p *printer) printRequestBody(req *http.Request) {
	// For client requests, a request with zero content-length and no body is also treated as unknown.
	if req.Body == nil {
		return
	}
	skip, err := p.checkBodyFiltered(req.Header)
	if err != nil {
		p.printf("* %s\n", p.format(color.FgRed, "error on request body filter: ", err.Error()))
	}
	if skip {
		return
	}
	if mediatype := req.Header.Get("Content-Type"); mediatype != "" && isBinaryMediatype(mediatype) {
		p.println("* body contains binary data")
		return
	}
	if p.logger.MaxRequestBody > 0 && req.ContentLength > p.logger.MaxRequestBody {
		p.printf("* body is too long (%d bytes) to print, skipping (longer than %d bytes)\n",
			req.ContentLength, p.logger.MaxRequestBody)
		return
	}
	contentType := req.Header.Get("Content-Type")
	if req.ContentLength > 0 {
		var buf bytes.Buffer
		tee := io.TeeReader(req.Body, &buf)
		defer req.Body.Close()
		defer func() {
			req.Body = io.NopCloser(&buf)
		}()
		p.printBodyReader(contentType, tee)
		return
	}
	if newBody, _ := p.printBodyUnknownLength(contentType, p.logger.MaxRequestBody, req.Body); newBody != nil {
		req.Body = newBody
	}
}

func (p *printer) printTimeRequest() (end func()) {
	startRequest := time.Now()
	p.printf("* Request at %v\n", startRequest)
	return func() {
		p.printf("* Request took %v\n", time.Since(startRequest))
	}
}
