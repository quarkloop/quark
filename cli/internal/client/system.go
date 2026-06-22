package client

import (
        "context"
        "fmt"
        "net/http"

        "github.com/quarkloop/quark/cli/internal/model"
)

// DeploySystem sends a .quark.ts source string to the server for deployment.
// On success returns the deploy response; on parse/validation failure
// returns a *DeployFailureError with the list of validation errors.
func (c *Client) DeploySystem(ctx context.Context, source, namespace string) (*model.DeploySystemResponse, error) {
        req := model.DeploySystemRequest{Source: source, Namespace: namespace}
        var resp model.DeploySystemResponse
        if err := c.post(ctx, "/systems/deploy", req, &resp); err != nil {
                // On 400, the server returns a deploy-failure body, not an error response.
                // Try to parse it as a failure.
                if apiErr, ok := err.(*APIError); ok && apiErr.StatusCode == 400 {
                        var failure model.DeploySystemFailure
                        if jsonErr := unmarshalBody(apiErr, &failure); jsonErr == nil && len(failure.Errors) > 0 {
                                return nil, &DeployFailureError{Failure: failure}
                        }
                }
                return nil, err
        }
        return &resp, nil
}

// ListSystems returns all deployed systems in the given namespace.
// If namespace is empty, returns systems across ALL namespaces (admin).
func (c *Client) ListSystems(ctx context.Context, namespace string) ([]model.SystemSummary, error) {
        var out []model.SystemSummary
        if err := c.get(ctx, "/systems"+buildQuery("namespace", namespace), &out); err != nil {
                return nil, err
        }
        return out, nil
}

// GetSystem returns the detailed state of a single system.
func (c *Client) GetSystem(ctx context.Context, name, namespace string) (*model.SystemDetail, error) {
        if namespace == "" {
                return nil, fmt.Errorf("namespace is required")
        }
        var out model.SystemDetail
        path := fmt.Sprintf("/systems/%s%s", name, buildQuery("namespace", namespace))
        if err := c.get(ctx, path, &out); err != nil {
                return nil, err
        }
        return &out, nil
}

// GetSystemSource returns the original .quark.ts source of a system.
// The returned string is the raw TypeScript text.
func (c *Client) GetSystemSource(ctx context.Context, name, namespace string) (string, error) {
        if namespace == "" {
                return "", fmt.Errorf("namespace is required")
        }
        // This endpoint returns text/plain, not JSON. Use a raw HTTP call.
        req, err := newGetRequest(ctx, c.baseURL+fmt.Sprintf("/systems/%s/source", name)+buildQuery("namespace", namespace))
        if err != nil {
                return "", err
        }
        resp, err := c.http.Do(req)
        if err != nil {
                return "", err
        }
        defer resp.Body.Close()
        body, _ := ioReadAll(resp.Body)
        if resp.StatusCode >= 400 {
                return "", &APIError{StatusCode: resp.StatusCode, Response: ErrorResponse{
                        Code: "HTTP_ERROR", Message: fmt.Sprintf("server returned %d", resp.StatusCode),
                }}
        }
        return string(body), nil
}

// DeleteSystem undeploys a system. Idempotent.
func (c *Client) DeleteSystem(ctx context.Context, name, namespace string) error {
        if namespace == "" {
                return fmt.Errorf("namespace is required")
        }
        path := fmt.Sprintf("/systems/%s%s", name, buildQuery("namespace", namespace))
        return c.delete(ctx, path)
}

// ApplySystem sends a declarative apply request (PUT /systems/{name}).
func (c *Client) ApplySystem(ctx context.Context, name, source, namespace string) (*model.ApplyResult, error) {
        req := model.DeploySystemRequest{Source: source, Namespace: namespace}
        var resp model.ApplyResult
        path := fmt.Sprintf("/systems/%s%s", name, buildQuery("namespace", namespace))
        if err := c.do(ctx, http.MethodPut, path, req, &resp); err != nil {
                return nil, err
        }
        return &resp, nil
}

// DeployFailureError is returned by DeploySystem when the server rejects
// the source with validation errors (HTTP 400).
type DeployFailureError struct {
        Failure model.DeploySystemFailure
}

func (e *DeployFailureError) Error() string {
        return e.Failure.Message
}

// unmarshalBody attempts to extract the raw body from an *APIError and
// unmarshal it into out. This is used for endpoints that return non-error
// bodies on 4xx (like deploy failures).
func unmarshalBody(apiErr *APIError, out interface{}) error {
        if len(apiErr.Body) > 0 {
                // Fast path: we have the raw bytes from the server — unmarshal directly.
                return jsonUnmarshal(apiErr.Body, out)
        }
        // Fallback: marshal the ErrorResponse back to JSON, then unmarshal into out.
        bs, err := jsonMarshal(apiErr.Response)
        if err != nil {
                return err
        }
        return jsonUnmarshal(bs, out)
}
