package api

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type cache struct {
	dir string
	ttl time.Duration
}

type cacheRoundTripper struct {
	fs fileStorage
	rt http.RoundTripper
}

type fileStorage struct {
	dir string
	ttl time.Duration
	mu  *sync.RWMutex
}

type readCloser struct {
	io.Reader
	io.Closer
}

func isCacheableRequest(req *http.Request) bool {
	if strings.EqualFold(req.Method, "GET") || strings.EqualFold(req.Method, "HEAD") {
		return true
	}

	if strings.EqualFold(req.Method, "POST") && (req.URL.Path == "/graphql" || req.URL.Path == "/api/graphql") {
		return true
	}

	return false
}

func isCacheableResponse(res *http.Response) bool {
	return res.StatusCode < 500 && res.StatusCode != 403
}

func cacheKey(req *http.Request) (string, error) {
	h := sha256.New()
	fmt.Fprintf(h, "%s:", req.Method)
	fmt.Fprintf(h, "%s:", req.URL.String())
	fmt.Fprintf(h, "%s:", req.Header.Get("Accept"))
	fmt.Fprintf(h, "%s:", req.Header.Get("Authorization"))

	if req.Body != nil {
		var bodyCopy io.ReadCloser
		req.Body, bodyCopy = copyStream(req.Body)
		defer bodyCopy.Close()
		if _, err := io.Copy(h, bodyCopy); err != nil {
			return "", err
		}
	}

	digest := h.Sum(nil)
	return fmt.Sprintf("%x", digest), nil
}

func (c cache) RoundTripper(rt http.RoundTripper) http.RoundTripper {
	fs := fileStorage{
		dir: c.dir,
		ttl: c.ttl,
		mu:  &sync.RWMutex{},
	}
	return cacheRoundTripper{fs: fs, rt: rt}
}

func (crt cacheRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	reqDir, reqTTL := requestCacheOptions(req)

	if crt.fs.ttl == 0 && reqTTL == 0 {
		return crt.rt.RoundTrip(req)
	}

	if !isCacheableRequest(req) {
		return crt.rt.RoundTrip(req)
	}

	origDir := crt.fs.dir
	if reqDir != "" {
		crt.fs.dir = reqDir
	}
	origTTL := crt.fs.ttl
	if reqTTL != 0 {
		crt.fs.ttl = reqTTL
	}

	key, keyErr := cacheKey(req)
	if keyErr == nil {
		if res, err := crt.fs.read(key); err == nil {
			res.Request = req
			return res, nil
		}
	}

	res, err := crt.rt.RoundTrip(req)
	if err == nil && keyErr == nil && isCacheableResponse(res) {
		_ = crt.fs.store(key, res)
	}

	crt.fs.dir = origDir
	crt.fs.ttl = origTTL

	return res, err
}

// Allow an individual request to override cache options.
func requestCacheOptions(req *http.Request) (string, time.Duration) {
	var dur time.Duration
	// Added alongside the TTL header in https://github.com/cli/go-gh/pull/49.
	// No production consumer of the directory override is known; retain it for
	// compatibility.
	dir := req.Header.Get("X-GH-CACHE-DIR")
	ttl := req.Header.Get("X-GH-CACHE-TTL")
	if ttl != "" {
		dur, _ = time.ParseDuration(ttl)
	}
	return dir, dur
}

func (fs *fileStorage) filePath(key string) string {
	if len(key) >= 6 {
		return filepath.Join(fs.dir, key[0:2], key[2:4], key[4:])
	}
	return filepath.Join(fs.dir, key)
}

func (fs *fileStorage) read(key string) (*http.Response, error) {
	cacheFile := fs.filePath(key)

	fs.mu.RLock()
	defer fs.mu.RUnlock()

	f, err := os.Open(cacheFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	age := time.Since(stat.ModTime())
	if age > fs.ttl {
		return nil, errors.New("cache expired")
	}

	body := &bytes.Buffer{}
	_, err = io.Copy(body, f)
	if err != nil {
		return nil, err
	}

	res, err := http.ReadResponse(bufio.NewReader(body), nil)
	return res, err
}

func (fs *fileStorage) store(key string, res *http.Response) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	cacheFilePath := fs.filePath(key)
	dir := filepath.Dir(cacheFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Finish writing before publishing the entry. Same-directory rename gives
	// atomic replacement on Unix; it is best-effort on other platforms.
	tmpCacheFile, err := os.CreateTemp(dir, ".gh-cache-*")
	if err != nil {
		return err
	}
	tmpCacheFileName := tmpCacheFile.Name()
	// Clean up on errors and panics too. After a successful rename, the
	// temporary path no longer exists, so removing it is harmless.
	defer func() {
		_ = tmpCacheFile.Close()
		_ = os.Remove(tmpCacheFileName)
	}()

	if err := writeCacheResponse(tmpCacheFile, res); err != nil {
		return err
	}
	if err := tmpCacheFile.Close(); err != nil {
		return err
	}

	return renameCacheFile(tmpCacheFileName, cacheFilePath)
}

func writeCacheResponse(w io.Writer, res *http.Response) error {
	if res.Body == nil {
		// Serialize the HTTP response headers only, since there is no body.
		return res.Write(w)
	}

	// Buffer the bytes consumed during serialization so the caller can replay
	// them. Restore the replay reader even if writing fails or panics.
	buffer := &bytes.Buffer{}
	recorder := &errorRecordingReader{Reader: io.TeeReader(res.Body, buffer)}
	source := &readCloser{Reader: recorder, Closer: res.Body}
	res.Body = source
	defer source.Close()
	defer func() {
		res.Body = io.NopCloser(&errorReplayingReader{Reader: buffer, err: recorder.err})
	}()

	return res.Write(w)
}

type errorRecordingReader struct {
	io.Reader
	err error
}

func (r *errorRecordingReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err != nil && err != io.EOF {
		r.err = err
	}
	return n, err
}

type errorReplayingReader struct {
	io.Reader
	err error
}

func (r *errorReplayingReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err == io.EOF && r.err != nil {
		err = r.err
		r.err = nil
	}
	return n, err
}

func copyStream(body io.ReadCloser) (replay, source io.ReadCloser) {
	buffer := &bytes.Buffer{}
	return io.NopCloser(buffer), &readCloser{
		Reader: io.TeeReader(body, buffer),
		Closer: body,
	}
}
