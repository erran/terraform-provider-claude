// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

// Package client is a thin HTTP client for the Claude Admin API.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// DefaultBaseURL is the public Claude Admin API endpoint.
	DefaultBaseURL = "https://api.anthropic.com"

	// APIVersion is sent in the anthropic-version header on every request.
	APIVersion = "2023-06-01"
)

// Client talks to the Claude Admin API using an OAuth bearer token carrying
// the org:admin scope.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	userAgent  string
}

// New returns a Client configured with the given OAuth bearer token. baseURL
// may be empty, in which case DefaultBaseURL is used.
func New(token, baseURL, userAgent string, httpClient *http.Client) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		userAgent:  userAgent,
	}
}

// APIError is returned for non-2xx responses from the Admin API.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("claude admin api returned %d: %s", e.StatusCode, e.Body)
}

// NotFound reports whether the error is an APIError with a 404 status.
func NotFound(err error) bool {
	var apiErr *APIError
	if e, ok := err.(*APIError); ok {
		apiErr = e
	}
	return apiErr != nil && apiErr.StatusCode == http.StatusNotFound
}

// Do issues an authenticated request against path (e.g.
// "/v1/organizations/service_accounts"). If body is non-nil it is JSON
// encoded. If out is non-nil a successful response body is decoded into it.
func (c *Client) Do(ctx context.Context, method, path string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		reqBody = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}

	req.Header.Set("anthropic-version", APIVersion)
	req.Header.Set("authorization", "Bearer "+c.token)
	if c.userAgent != "" {
		req.Header.Set("user-agent", c.userAgent)
	}
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("performing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decoding response body: %w", err)
		}
	}

	return nil
}
