package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cli/go-gh/pkg/api"

	graphql "github.com/cli/shurcooL-graphql"
)

// Implements api.GQLClient interface.
type gqlClient struct {
	client     *graphql.Client
	host       string
	httpClient *http.Client
}

func NewGQLClient(host string, opts *api.ClientOptions) api.GQLClient {
	httpClient := NewHTTPClient(opts)
	endpoint := gqlEndpoint(host)
	return gqlClient{
		client:     graphql.NewClient(endpoint, &httpClient),
		host:       endpoint,
		httpClient: &httpClient,
	}
}

// DoWithContext executes a single GraphQL query request and populates the response into the data argument.
func (c gqlClient) DoWithContext(ctx context.Context, query string, variables map[string]interface{}, response interface{}) error {
	reqBody, err := json.Marshal(map[string]interface{}{"query": query, "variables": variables})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.host, bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	if !success {
		return api.HandleHTTPError(resp)
	}

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	gr := gqlResponse{Data: response}
	err = json.Unmarshal(body, &gr)
	if err != nil {
		return err
	}

	if len(gr.Errors) > 0 {
		return api.GQLError{Errors: gr.Errors}
	}

	return nil
}

// Do wraps DoWithContext using context.Background.
func (c gqlClient) Do(query string, variables map[string]interface{}, response interface{}) error {
	return c.DoWithContext(context.Background(), query, variables, response)
}

// MutateWithContext executes a single GraphQL mutation request,
// with a mutation derived from m, populating the response into it.
// "m" should be a pointer to struct that corresponds to the GitHub GraphQL schema.
func (c gqlClient) MutateWithContext(ctx context.Context, name string, m interface{}, variables map[string]interface{}) error {
	return c.client.MutateNamed(ctx, name, m, variables)
}

// Mutate wraps MutateWithContext using context.Background.
func (c gqlClient) Mutate(name string, m interface{}, variables map[string]interface{}) error {
	return c.MutateWithContext(context.Background(), name, m, variables)
}

// QueryWithContext executes a single GraphQL query request,
// with a query derived from q, populating the response into it.
// "q" should be a pointer to struct that corresponds to the GitHub GraphQL schema.
func (c gqlClient) QueryWithContext(ctx context.Context, name string, q interface{}, variables map[string]interface{}) error {
	return c.client.QueryNamed(ctx, name, q, variables)
}

// Query wraps QueryWithContext using context.Background.
func (c gqlClient) Query(name string, q interface{}, variables map[string]interface{}) error {
	return c.QueryWithContext(context.Background(), name, q, variables)
}

type gqlResponse struct {
	Data   interface{}
	Errors []api.GQLErrorItem
}

func gqlEndpoint(host string) string {
	if isGarage(host) {
		return fmt.Sprintf("https://%s/api/graphql", host)
	}
	host = normalizeHostname(host)
	if isEnterprise(host) {
		return fmt.Sprintf("https://%s/api/graphql", host)
	}
	if strings.EqualFold(host, localhost) {
		return fmt.Sprintf("http://api.%s/graphql", host)
	}
	return fmt.Sprintf("https://api.%s/graphql", host)
}
