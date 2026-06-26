package client

import (
        "context"
        "fmt"
        "net/http"

        "github.com/quarkloop/quark/cli/internal/model"
)

// ListNamespaces returns all active namespaces with metrics.
func (c *Client) ListNamespaces(ctx context.Context) ([]model.NamespaceSummary, error) {
        var out []model.NamespaceSummary
        if err := c.get(ctx, "/api/v1/namespaces", &out); err != nil {
                return nil, err
        }
        return out, nil
}

// GetNamespace returns details and metrics for a single namespace.
func (c *Client) GetNamespace(ctx context.Context, namespace string) (*model.NamespaceDetail, error) {
        var out model.NamespaceDetail
        path := fmt.Sprintf("/api/v1/namespaces/%s", namespace)
        if err := c.get(ctx, path, &out); err != nil {
                return nil, err
        }
        return &out, nil
}

// DeploySystem deploys a system via POST /api/v1/namespaces/{ns}/systems.
func (c *Client) DeploySystem(ctx context.Context, source, namespace string) (*model.DeploySystemResponse, error) {
        req := model.DeploySystemRequest{Source: source, Namespace: namespace}
        var resp model.DeploySystemResponse
        path := fmt.Sprintf("/api/v1/namespaces/%s/systems", namespace)
        if err := c.post(ctx, path, req, &resp); err != nil {
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

// ListSystems returns all systems in a namespace.
func (c *Client) ListSystems(ctx context.Context, namespace string) ([]model.SystemSummary, error) {
        var out []model.SystemSummary
        path := fmt.Sprintf("/api/v1/namespaces/%s/systems", namespace)
        if err := c.get(ctx, path, &out); err != nil {
                return nil, err
        }
        return out, nil
}

// GetSystem returns details of a single system.
func (c *Client) GetSystem(ctx context.Context, name, namespace string) (*model.SystemDetail, error) {
        var out model.SystemDetail
        path := fmt.Sprintf("/api/v1/namespaces/%s/systems/%s", namespace, name)
        if err := c.get(ctx, path, &out); err != nil {
                return nil, err
        }
        return &out, nil
}

// GetSystemSource returns the raw .quark.ts source.
func (c *Client) GetSystemSource(ctx context.Context, name, namespace string) (string, error) {
        req, err := newGetRequest(ctx, c.baseURL+fmt.Sprintf("/api/v1/namespaces/%s/systems/%s/source", namespace, name))
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

// DeleteSystem undeploys a system.
func (c *Client) DeleteSystem(ctx context.Context, name, namespace string) error {
        path := fmt.Sprintf("/api/v1/namespaces/%s/systems/%s", namespace, name)
        return c.delete(ctx, path)
}

// ApplySystem sends a declarative apply request.
func (c *Client) ApplySystem(ctx context.Context, name, source, namespace string) (*model.ApplyResult, error) {
        req := model.DeploySystemRequest{Source: source, Namespace: namespace}
        var resp model.ApplyResult
        path := fmt.Sprintf("/api/v1/namespaces/%s/systems/%s", namespace, name)
        if err := c.do(ctx, http.MethodPut, path, req, &resp); err != nil {
                return nil, err
        }
        return &resp, nil
}

// ListNodes returns all nodes in a system (or all nodes in a namespace if system is empty).
func (c *Client) ListNodes(ctx context.Context, namespace, system string) ([]model.NodeSummary, error) {
        var out []model.NodeSummary
        path := fmt.Sprintf("/api/v1/namespaces/%s/systems/%s/nodes", namespace, system)
        if err := c.get(ctx, path, &out); err != nil {
                return nil, err
        }
        return out, nil
}

// GetNode returns details of a single node.
func (c *Client) GetNode(ctx context.Context, name, namespace, system string) (*model.NodeDetail, error) {
        var out model.NodeDetail
        path := fmt.Sprintf("/api/v1/namespaces/%s/systems/%s/nodes/%s", namespace, system, name)
        if err := c.get(ctx, path, &out); err != nil {
                return nil, err
        }
        return &out, nil
}

// NodeLifecycle performs a lifecycle operation on a node.
func (c *Client) NodeLifecycle(ctx context.Context, action, name, namespace, system string) error {
        path := fmt.Sprintf("/api/v1/namespaces/%s/systems/%s/nodes/%s/%s", namespace, system, name, action)
        return c.post(ctx, path, nil, nil)
}

// ListEvents queries events in a namespace.
func (c *Client) ListEvents(ctx context.Context, namespace, system, node, kinds, since, until string, limit int, all bool) ([]model.Event, error) {
        limitStr := ""
        if limit > 0 {
                limitStr = fmt.Sprintf("%d", limit)
        }
        allStr := ""
        if all {
                allStr = "true"
        }
        path := fmt.Sprintf("/api/v1/namespaces/%s/events", namespace) + buildQuery(
                "system", system, "node", node, "kinds", kinds,
                "since", since, "until", until, "limit", limitStr, "all", allStr)
        var out []model.Event
        if err := c.get(ctx, path, &out); err != nil {
                return nil, err
        }
        return out, nil
}

// ListRegistry returns all registered node implementations.
func (c *Client) ListRegistry(ctx context.Context, query string) ([]model.RegistryEntry, error) {
        var out []model.RegistryEntry
        path := "/api/v1/registry" + buildQuery("q", query)
        if err := c.get(ctx, path, &out); err != nil {
                return nil, err
        }
        return out, nil
}

// GetRegistryEntry looks up a specific implementation by URI.
func (c *Client) GetRegistryEntry(ctx context.Context, uri string) (*model.RegistryEntry, error) {
        var out model.RegistryEntry
        path := fmt.Sprintf("/api/v1/registry/%s", uri)
        if err := c.get(ctx, path, &out); err != nil {
                return nil, err
        }
        return &out, nil
}

// DeployFailureError is returned by DeploySystem when the server rejects the source.
type DeployFailureError struct {
        Failure model.DeploySystemFailure
}

func (e *DeployFailureError) Error() string {
        return e.Failure.Message
}

func unmarshalBody(apiErr *APIError, out interface{}) error {
        if len(apiErr.Body) > 0 {
                return jsonUnmarshal(apiErr.Body, out)
        }
        bs, err := jsonMarshal(apiErr.Response)
        if err != nil {
                return err
        }
        return jsonUnmarshal(bs, out)
}
