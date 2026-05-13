package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/quarkloop/pkg/plugin"
	servicev1 "github.com/quarkloop/pkg/serviceapi/gen/quark/service/v1"
	"github.com/quarkloop/pkg/serviceapi/servicekit"
)

// Catalog is the runtime-owned view of discovered gRPC services. It is the
// single service capability surface exposed to the agent loop.
type Catalog struct {
	descriptors []*servicev1.ServiceDescriptor
	executor    *Executor
	prompt      string
}

func NewCatalog(descriptors []*servicev1.ServiceDescriptor) *Catalog {
	copied := make([]*servicev1.ServiceDescriptor, 0, len(descriptors))
	for _, desc := range descriptors {
		if desc == nil {
			continue
		}
		copied = append(copied, servicekit.CloneDescriptor(desc))
	}
	return &Catalog{
		descriptors: copied,
		executor:    NewExecutor(copied),
		prompt:      PromptBlock(copied),
	}
}

func DiscoverCatalog(ctx context.Context, endpoints []Endpoint) (*Catalog, []error) {
	descriptors, errs := Discover(ctx, endpoints)
	return NewCatalog(descriptors), errs
}

func (c *Catalog) Empty() bool {
	return c == nil || len(c.descriptors) == 0
}

func (c *Catalog) Descriptors() []*servicev1.ServiceDescriptor {
	if c == nil {
		return nil
	}
	out := make([]*servicev1.ServiceDescriptor, 0, len(c.descriptors))
	for _, desc := range c.descriptors {
		out = append(out, servicekit.CloneDescriptor(desc))
	}
	return out
}

func (c *Catalog) Prompt() string {
	if c == nil {
		return ""
	}
	return c.prompt
}

func (c *Catalog) ToolSchemas() []plugin.ToolSchema {
	if c == nil || c.executor == nil {
		return nil
	}
	return c.executor.ToolSchemas()
}

func (c *Catalog) ExecuteTool(ctx context.Context, name, arguments string) (string, bool, error) {
	if name != ToolName {
		return "", false, nil
	}
	if c == nil || c.executor == nil || len(c.descriptors) == 0 {
		return "", true, fmt.Errorf("no gRPC services are available")
	}
	out, err := c.executor.Execute(ctx, arguments)
	return strings.TrimSpace(out), true, err
}
