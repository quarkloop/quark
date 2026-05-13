package services

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	servicev1 "github.com/quarkloop/pkg/serviceapi/gen/quark/service/v1"
	"github.com/quarkloop/pkg/serviceapi/servicekit"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	EnvServiceAddrs     = "QUARK_SERVICE_ADDRS"
	EnvIndexerAddr      = "QUARK_INDEXER_ADDR"
	EnvBuildReleaseAddr = "QUARK_BUILD_RELEASE_ADDR"
	EnvSpaceServiceAddr = "QUARK_SPACE_SERVICE_ADDR"
	EnvDisableDiscovery = "QUARK_DISABLE_SERVICE_DISCOVERY"
)

type Endpoint struct {
	Name    string
	Address string
}

type DiscoveryError struct {
	Endpoint Endpoint
	Err      error
}

func (e DiscoveryError) Error() string {
	return fmt.Sprintf("discover service %s at %s: %v", e.Endpoint.Name, e.Endpoint.Address, e.Err)
}

func EndpointsFromEnv() []Endpoint {
	if DiscoveryDisabledFromEnv() {
		return nil
	}
	endpoints := ParseEndpoints(os.Getenv(EnvServiceAddrs))
	endpoints = appendIfSet(endpoints, "indexer", os.Getenv(EnvIndexerAddr))
	endpoints = appendIfSet(endpoints, "build-release", os.Getenv(EnvBuildReleaseAddr))
	endpoints = appendIfSet(endpoints, "space", os.Getenv(EnvSpaceServiceAddr))
	return dedupe(endpoints)
}

func DiscoveryDisabledFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(EnvDisableDiscovery))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func ParseEndpoints(raw string) []Endpoint {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
	out := make([]Endpoint, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name := ""
		addr := part
		if before, after, ok := strings.Cut(part, "="); ok {
			name = strings.TrimSpace(before)
			addr = strings.TrimSpace(after)
		}
		if addr == "" {
			continue
		}
		out = append(out, Endpoint{Name: name, Address: addr})
	}
	return out
}

func Discover(ctx context.Context, endpoints []Endpoint) ([]*servicev1.ServiceDescriptor, []error) {
	descriptors := make([]*servicev1.ServiceDescriptor, 0, len(endpoints))
	var errs []error
	for _, endpoint := range endpoints {
		if endpoint.Address == "" {
			continue
		}
		callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		conn, err := servicekit.Dial(callCtx, endpoint.Address)
		if err != nil {
			cancel()
			errs = append(errs, DiscoveryError{Endpoint: endpoint, Err: err})
			continue
		}
		client := servicev1.NewServiceRegistryClient(conn)
		resp, err := client.ListServices(callCtx, &emptypb.Empty{})
		cancel()
		_ = conn.Close()
		if err != nil {
			errs = append(errs, DiscoveryError{Endpoint: endpoint, Err: err})
			continue
		}
		for _, desc := range resp.GetServices() {
			cp := servicekit.CloneDescriptor(desc)
			if cp.Address == "" {
				cp.Address = endpoint.Address
			}
			if cp.Name == "" {
				cp.Name = endpoint.Name
			}
			descriptors = append(descriptors, cp)
		}
	}
	sort.Slice(descriptors, func(i, j int) bool { return descriptors[i].GetName() < descriptors[j].GetName() })
	return descriptors, errs
}

func PromptBlock(descriptors []*servicev1.ServiceDescriptor) string {
	if len(descriptors) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n## Available gRPC Services\n\n")
	b.WriteString("The runtime has discovered the following gRPC services. Call them with the `grpc-service` tool when a task needs service-backed capabilities.\n")
	b.WriteString("\n`grpc-service` arguments must be JSON with `service`, `method`, and `request` fields. The `request` field must match the protobuf JSON shape for the RPC request message.\n")
	for _, desc := range descriptors {
		fmt.Fprintf(&b, "\n### %s\n\n", desc.GetName())
		fmt.Fprintf(&b, "- Type: `%s`\n", desc.GetType())
		fmt.Fprintf(&b, "- Version: `%s`\n", desc.GetVersion())
		fmt.Fprintf(&b, "- Address: `%s`\n", desc.GetAddress())
		if len(desc.GetRpcs()) > 0 {
			b.WriteString("- RPCs:\n")
			for _, rpc := range desc.GetRpcs() {
				fmt.Fprintf(&b, "  - `%s/%s`: `%s` -> `%s`", rpc.GetService(), rpc.GetMethod(), rpc.GetRequest(), rpc.GetResponse())
				if rpc.GetDescription() != "" {
					fmt.Fprintf(&b, " - %s", rpc.GetDescription())
				}
				b.WriteByte('\n')
			}
		}
		for _, skill := range desc.GetSkills() {
			if strings.TrimSpace(skill.GetMarkdown()) == "" {
				continue
			}
			fmt.Fprintf(&b, "\nService skill `%s`:\n\n%s\n", skill.GetName(), strings.TrimSpace(skill.GetMarkdown()))
		}
	}
	b.WriteString("\nUse service skills together with tool results. Do not invent service responses; call `grpc-service` and use its returned JSON.\n")
	return b.String()
}

func appendIfSet(endpoints []Endpoint, name, addr string) []Endpoint {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return endpoints
	}
	return append(endpoints, Endpoint{Name: name, Address: addr})
}

func dedupe(in []Endpoint) []Endpoint {
	seen := make(map[string]bool, len(in))
	out := make([]Endpoint, 0, len(in))
	for _, endpoint := range in {
		key := endpoint.Name + "\x00" + endpoint.Address
		if endpoint.Address == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, endpoint)
	}
	return out
}
