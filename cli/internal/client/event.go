package client

import (
	"context"
	"fmt"

	"github.com/quarkloop/quark/cli/internal/model"
)

// EventListOptions configures an event query.
type EventListOptions struct {
	Namespace     string
	System      string
	Node      string
	Kinds         string // comma-separated
	Since         string // ISO-8601
	Until         string // ISO-8601
	Limit         int
	AllNamespaces bool // admin mode
}

// ListEvents queries events matching the given filters.
func (c *Client) ListEvents(ctx context.Context, opts EventListOptions) ([]model.Event, error) {
	if !opts.AllNamespaces && opts.Namespace == "" {
		return nil, fmt.Errorf("namespace is required (or set AllNamespaces=true for admin mode)")
	}
	var out []model.Event
	limitStr := ""
	if opts.Limit > 0 {
		limitStr = fmt.Sprintf("%d", opts.Limit)
	}
	allStr := ""
	if opts.AllNamespaces {
		allStr = "true"
	}
	path := "/events" + buildQuery(
		"namespace", opts.Namespace,
		"system", opts.System,
		"node", opts.Node,
		"kinds", opts.Kinds,
		"since", opts.Since,
		"until", opts.Until,
		"limit", limitStr,
		"all", allStr,
	)
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CountEvents returns the count of events matching the given filters.
func (c *Client) CountEvents(ctx context.Context, opts EventListOptions) (int64, error) {
	if !opts.AllNamespaces && opts.Namespace == "" {
		return 0, fmt.Errorf("namespace is required (or set AllNamespaces=true for admin mode)")
	}
	var out int64
	allStr := ""
	if opts.AllNamespaces {
		allStr = "true"
	}
	path := "/events/count" + buildQuery(
		"namespace", opts.Namespace,
		"system", opts.System,
		"node", opts.Node,
		"kinds", opts.Kinds,
		"all", allStr,
	)
	if err := c.get(ctx, path, &out); err != nil {
		return 0, err
	}
	return out, nil
}
