package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cli/go-gh/pkg/api"
)

// Implements api.RESTClient interface.
type restClient struct {
	client http.Client
	host   string
}

func NewRESTClient(host string, opts *api.ClientOptions) api.RESTClient {
	return restClient{
		client: NewHTTPClient(opts),
		host:   host,
	}
}

func (c restClient) RequestWithContext(ctx context.Context, method string, path string, body io.Reader) (*http.Response, error) {
	url := restURL(c.host, path)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	if !success {
		defer resp.Body.Close()
		return nil, api.HandleHTTPError(resp)
	}

	return resp, err
}

func (c restClient) Request(method string, path string, body io.Reader) (*http.Response, error) {
	return c.RequestWithContext(context.Background(), method, path, body)
}

func (c restClient) DoWithContext(ctx context.Context, method string, path string, body io.Reader, response interface{}) error {
	url := restURL(c.host, path)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	if !success {
		defer resp.Body.Close()
		return api.HandleHTTPError(resp)
	}

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(b, &response)
	if err != nil {
		return err
	}

	return nil
}

func (c restClient) Do(method string, path string, body io.Reader, response interface{}) error {
	return c.DoWithContext(context.Background(), method, path, body, response)
}

func (c restClient) Delete(path string, resp interface{}) error {
	return c.Do(http.MethodDelete, path, nil, resp)
}

func (c restClient) Get(path string, resp interface{}) error {
	return c.Do(http.MethodGet, path, nil, resp)
}

func (c restClient) Patch(path string, body io.Reader, resp interface{}) error {
	return c.Do(http.MethodPatch, path, body, resp)
}

func (c restClient) Post(path string, body io.Reader, resp interface{}) error {
	return c.Do(http.MethodPost, path, body, resp)
}

func (c restClient) Put(path string, body io.Reader, resp interface{}) error {
	return c.Do(http.MethodPut, path, body, resp)
}

func restURL(hostname string, pathOrURL string) string {
	if strings.HasPrefix(pathOrURL, "https://") || strings.HasPrefix(pathOrURL, "http://") {
		return pathOrURL
	}
	return restPrefix(hostname) + pathOrURL
}

func restPrefix(hostname string) string {
	if isGarage(hostname) {
		return fmt.Sprintf("https://%s/api/v3/", hostname)
	}
	hostname = normalizeHostname(hostname)
	if isEnterprise(hostname) {
		return fmt.Sprintf("https://%s/api/v3/", hostname)
	}
	if strings.EqualFold(hostname, localhost) {
		return fmt.Sprintf("http://api.%s/", hostname)
	}
	return fmt.Sprintf("https://api.%s/", hostname)
}
