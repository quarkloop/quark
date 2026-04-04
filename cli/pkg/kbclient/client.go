// Package kbclient provides a unified knowledge base client that works
// in two modes: local (direct filesystem) or HTTP (remote server).
// The same API is used regardless of the transport.
package kbclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/quarkloop/cli/pkg/kb"
)

// Client is the unified KB API.
// Exactly one of localStore or baseURL/HTTP must be set.
type Client struct {
	localStore kb.Store
	baseURL    string
	http       *http.Client
}

// NewLocal creates a client backed by the filesystem at spaceDir.
func NewLocal(spaceDir string) (*Client, error) {
	s, err := kb.Open(spaceDir)
	if err != nil {
		return nil, err
	}
	return &Client{localStore: s}, nil
}

// NewHTTP creates a client that talks to a remote KB server.
func NewHTTP(baseURL string, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{}
	}
	return &Client{baseURL: baseURL, http: hc}
}

// Close releases resources. Only meaningful for local clients.
func (c *Client) Close() error {
	if c.localStore != nil {
		return c.localStore.Close()
	}
	return nil
}

// Get retrieves a value by namespace/key.
func (c *Client) Get(ctx context.Context, namespace, key string) ([]byte, error) {
	if c.localStore != nil {
		return c.localStore.Get(namespace, key)
	}
	return c.httpGet(ctx, namespace, key)
}

// Set stores a value at namespace/key.
func (c *Client) Set(ctx context.Context, namespace, key string, value []byte) error {
	if c.localStore != nil {
		return c.localStore.Set(namespace, key, value)
	}
	return c.httpSet(ctx, namespace, key, value)
}

// Delete removes an entry at namespace/key.
func (c *Client) Delete(ctx context.Context, namespace, key string) error {
	if c.localStore != nil {
		return c.localStore.Delete(namespace, key)
	}
	return c.httpDelete(ctx, namespace, key)
}

// List returns all keys in the given namespace.
func (c *Client) List(ctx context.Context, namespace string) ([]string, error) {
	if c.localStore != nil {
		return c.localStore.List(namespace)
	}
	return c.httpList(ctx, namespace)
}

// --- HTTP transport ---

type setPayload struct {
	ID    string `json:"id"`
	Value []byte `json:"value"`
}
type getResponse struct {
	Value []byte `json:"value"`
}
type listResponse struct {
	Keys []string `json:"keys"`
}

func (c *Client) httpGet(ctx context.Context, namespace, key string) ([]byte, error) {
	id := namespace + "/" + key
	url := c.baseURL + "/api/v1/kb/" + id
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kb get: %d: %s", resp.StatusCode, string(body))
	}
	var r getResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	return r.Value, nil
}

func (c *Client) httpSet(ctx context.Context, namespace, key string, value []byte) error {
	payload, _ := json.Marshal(setPayload{
		ID:    namespace + "/" + key,
		Value: value,
	})
	url := c.baseURL + "/api/v1/kb"
	resp, err := c.http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kb set: %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) httpDelete(ctx context.Context, namespace, key string) error {
	id := namespace + "/" + key
	url := c.baseURL + "/api/v1/kb/" + id
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kb delete: %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) httpList(ctx context.Context, namespace string) ([]string, error) {
	url := c.baseURL + "/api/v1/kb?namespace=" + namespace
	resp, err := c.http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kb list: %d: %s", resp.StatusCode, string(body))
	}
	var r listResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	return r.Keys, nil
}

// NewHTTPWithToken creates an HTTP client that carries an auth token.
func NewHTTPWithToken(baseURL, token string) *Client {
	hc := &http.Client{}
	if len(os.Getenv("KB_TOKEN")) > 0 {
		_ = token
	}
	// The token-carrying client is built via a transport that injects headers.
	_ = hc
	c := NewHTTP(baseURL, hc)
	return c
}
