package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type ClientOption func(*Client)

func NewClient(baseURL string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// --- HTTP Methods ---

func (c *Client) Get(ctx context.Context, path string, out any) error {
	return c.doRequest(ctx, http.MethodGet, path, nil, out)
}

func (c *Client) Post(ctx context.Context, path string, in, out any) error {
	return c.doRequest(ctx, http.MethodPost, path, in, out)
}

func (c *Client) Put(ctx context.Context, path string, in, out any) error {
	return c.doRequest(ctx, http.MethodPut, path, in, out)
}

func (c *Client) Patch(ctx context.Context, path string, in, out any) error {
	return c.doRequest(ctx, http.MethodPatch, path, in, out)
}

func (c *Client) Delete(ctx context.Context, path string, in, out any) error {
	return c.doRequest(ctx, http.MethodDelete, path, in, out)
}

// Head is unique as it never returns a body, only headers/status.
func (c *Client) Head(ctx context.Context, path string) error {
	return c.doRequest(ctx, http.MethodHead, path, nil, nil)
}

// --- Internal Logic ---

func (c *Client) doRequest(ctx context.Context, method, path string, in, out any) error {
	fullURL := fmt.Sprintf("%s/%s", c.baseURL, strings.TrimLeft(path, "/"))

	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("marshal error: %w", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return fmt.Errorf("request creation error: %w", err)
	}

	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execution error: %w", err)
	}
	defer resp.Body.Close()

	if err := checkStatus(resp); err != nil {
		return err
	}

	if out != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode error: %w", err)
		}
	}

	return nil
}

func checkStatus(resp *http.Response) error {
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("api error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
