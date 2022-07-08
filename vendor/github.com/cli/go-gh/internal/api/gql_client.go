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

	if isEnterprise(host) {
		host = fmt.Sprintf("https://%s/api/graphql", host)
	} else {
		host = "https://api.github.com/graphql"
	}

	return gqlClient{
		client:     graphql.NewClient(host, &httpClient),
		host:       host,
		httpClient: &httpClient,
	}
}

// Do executes a single GraphQL query request and populates the response into the data argument.
func (c gqlClient) Do(query string, variables map[string]interface{}, response interface{}) error {
	reqBody, err := json.Marshal(map[string]interface{}{"query": query, "variables": variables})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.host, bytes.NewBuffer(reqBody))
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
		return handleHTTPError(resp)
	}

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	gr := &gqlResponse{Data: response}
	err = json.Unmarshal(body, &gr)
	if err != nil {
		return err
	}

	if len(gr.Errors) > 0 {
		return &gqlErrorResponse{Errors: gr.Errors}
	}

	return nil
}

// Mutate executes a single GraphQL mutation request,
// with a mutation derived from m, populating the response into it.
// "m" should be a pointer to struct that corresponds to the GitHub GraphQL schema.
func (c gqlClient) Mutate(name string, m interface{}, variables map[string]interface{}) error {
	return c.client.MutateNamed(context.Background(), name, m, variables)
}

// Query executes a single GraphQL query request,
// with a query derived from q, populating the response into it.
// "q" should be a pointer to struct that corresponds to the GitHub GraphQL schema.
func (c gqlClient) Query(name string, q interface{}, variables map[string]interface{}) error {
	return c.client.QueryNamed(context.Background(), name, q, variables)
}

type gqlResponse struct {
	Data   interface{}
	Errors []gqlError
}

type gqlError struct {
	Type    string
	Message string
}

type gqlErrorResponse struct {
	Errors []gqlError
}

func (gr gqlErrorResponse) Error() string {
	errorMessages := make([]string, 0, len(gr.Errors))
	for _, e := range gr.Errors {
		errorMessages = append(errorMessages, e.Message)
	}
	return fmt.Sprintf("GQL error: %s", strings.Join(errorMessages, "\n"))
}
