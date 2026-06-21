// Package client is a typed HTTP client for the Quark server REST API.
//
// One method per REST endpoint. Returns Go structs from internal/model.
// No I/O outside HTTP — no file reads, no terminal output. The cmd/ layer
// is responsible for calling these methods and formatting the results.
package client

import (
        "bytes"
        "context"
        "encoding/json"
        "fmt"
        "io"
        "net/http"
        "net/url"
        "strings"
        "time"
)

// Client is the typed HTTP client for the Quark server.
type Client struct {
        baseURL string
        http    *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option {
        return func(c *Client) { c.http.Timeout = d }
}

// New constructs a Client pointing at the given server base URL (e.g. "http://localhost:8080").
func New(baseURL string, opts ...Option) *Client {
        c := &Client{
                baseURL: strings.TrimRight(baseURL, "/"),
                http:    &http.Client{Timeout: 30 * time.Second},
        }
        for _, opt := range opts {
                opt(c)
        }
        return c
}

// do performs an HTTP request and unmarshals the JSON response into out.
// On non-2xx responses it returns an *APIError with the server's error body.
func (c *Client) do(ctx context.Context, method, path string, body interface{}, out interface{}) error {
        var bodyReader io.Reader
        if body != nil {
                bs, err := json.Marshal(body)
                if err != nil {
                        return fmt.Errorf("marshal request body: %w", err)
                }
                bodyReader = bytes.NewReader(bs)
        }
        req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
        if err != nil {
                return fmt.Errorf("build request: %w", err)
        }
        if body != nil {
                req.Header.Set("Content-Type", "application/json")
        }
        resp, err := c.http.Do(req)
        if err != nil {
                return fmt.Errorf("http request: %w", err)
        }
        defer resp.Body.Close()
        respBody, _ := io.ReadAll(resp.Body)
        if resp.StatusCode >= 400 {
                var errResp ErrorResponse
                if json.Unmarshal(respBody, &errResp) == nil && errResp.Message != "" {
                        return &APIError{StatusCode: resp.StatusCode, Response: errResp, Body: respBody}
                }
                return &APIError{StatusCode: resp.StatusCode, Response: ErrorResponse{
                        Code:    "HTTP_ERROR",
                        Message: fmt.Sprintf("server returned %d: %s", resp.StatusCode, string(respBody)),
                }, Body: respBody}
        }
        if out == nil || len(respBody) == 0 {
                return nil
        }
        if err := json.Unmarshal(respBody, out); err != nil {
                return fmt.Errorf("unmarshal response: %w (body: %s)", err, string(respBody))
        }
        return nil
}

// get is a convenience wrapper for GET requests with no body.
func (c *Client) get(ctx context.Context, path string, out interface{}) error {
        return c.do(ctx, http.MethodGet, path, nil, out)
}

// post is a convenience wrapper for POST requests.
func (c *Client) post(ctx context.Context, path string, body, out interface{}) error {
        return c.do(ctx, http.MethodPost, path, body, out)
}

// delete is a convenience wrapper for DELETE requests.
func (c *Client) delete(ctx context.Context, path string) error {
        return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// buildQuery encodes non-empty key-value pairs into a URL query string starting with "?".
// Returns empty string if no pairs are non-empty.
func buildQuery(pairs ...string) string {
        if len(pairs)%2 != 0 {
                panic("buildQuery requires even number of args (key, value, ...)")
        }
        v := url.Values{}
        for i := 0; i < len(pairs); i += 2 {
                key, val := pairs[i], pairs[i+1]
                if val != "" {
                        v.Set(key, val)
                }
        }
        if len(v) == 0 {
                return ""
        }
        return "?" + v.Encode()
}

// newGetRequest builds a GET request with the given URL.
func newGetRequest(ctx context.Context, url string) (*http.Request, error) {
        return http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
}

// ioReadAll is io.ReadAll (aliased so system.go doesn't need a separate import).
func ioReadAll(r io.Reader) ([]byte, error) {
        return io.ReadAll(r)
}

// jsonMarshal is json.Marshal (aliased to avoid import in system.go).
func jsonMarshal(v interface{}) ([]byte, error) {
        return json.Marshal(v)
}

// jsonUnmarshal is json.Unmarshal (aliased to avoid import in system.go).
func jsonUnmarshal(data []byte, v interface{}) error {
        return json.Unmarshal(data, v)
}

// ErrorResponse is the server's error response shape.
type ErrorResponse struct {
        Code    string                 `json:"code"`
        Message string                 `json:"message"`
        Details map[string]interface{} `json:"details,omitempty"`
}

// APIError is returned when the server responds with a non-2xx status.
type APIError struct {
        StatusCode int
        Response   ErrorResponse
        // Body is the raw HTTP response body. Used by unmarshalBody to recover
        // structured error shapes that don't fit ErrorResponse (e.g. deploy
        // failures carry {message, errors:[...]}).
        Body []byte
}

func (e *APIError) Error() string {
        return fmt.Sprintf("HTTP %d: %s — %s", e.StatusCode, e.Response.Code, e.Response.Message)
}
