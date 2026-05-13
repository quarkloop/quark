package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/quarkloop/pkg/plugin"
	buildreleasev1 "github.com/quarkloop/pkg/serviceapi/gen/quark/buildrelease/v1"
	indexerv1 "github.com/quarkloop/pkg/serviceapi/gen/quark/indexer/v1"
	servicev1 "github.com/quarkloop/pkg/serviceapi/gen/quark/service/v1"
	spacev1 "github.com/quarkloop/pkg/serviceapi/gen/quark/space/v1"
	"github.com/quarkloop/pkg/serviceapi/servicekit"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/emptypb"
)

const ToolName = "grpc-service"

var _ = []any{
	indexerv1.File_quark_indexer_v1_indexer_proto,
	buildreleasev1.File_quark_buildrelease_v1_build_release_proto,
	spacev1.File_quark_space_v1_space_proto,
	emptypb.File_google_protobuf_empty_proto,
}

type Executor struct {
	descriptors []*servicev1.ServiceDescriptor
}

type ToolRequest struct {
	Service string          `json:"service"`
	Method  string          `json:"method"`
	Request json.RawMessage `json:"request"`
}

func NewExecutor(descriptors []*servicev1.ServiceDescriptor) *Executor {
	out := make([]*servicev1.ServiceDescriptor, 0, len(descriptors))
	for _, desc := range descriptors {
		out = append(out, servicekit.CloneDescriptor(desc))
	}
	return &Executor{descriptors: out}
}

func (e *Executor) ToolSchemas() []plugin.ToolSchema {
	if e == nil || len(e.descriptors) == 0 {
		return nil
	}
	return []plugin.ToolSchema{{
		Name:        ToolName,
		Description: "Call a discovered Quark gRPC service by service name, method, and JSON protobuf request.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "string",
					"description": "Service descriptor name such as indexer, space, or build-release, or the full protobuf service name.",
				},
				"method": map[string]any{
					"type":        "string",
					"description": "RPC method name such as IndexDocument or GetContext.",
				},
				"request": map[string]any{
					"type":        "object",
					"description": "JSON representation of the protobuf request message.",
				},
			},
			"required": []string{"service", "method", "request"},
		},
	}}
}

func (e *Executor) Execute(ctx context.Context, arguments string) (string, error) {
	if e == nil {
		return "", fmt.Errorf("service executor is not configured")
	}
	var req ToolRequest
	if err := json.Unmarshal([]byte(arguments), &req); err != nil {
		return "", fmt.Errorf("parse grpc-service arguments: %w", err)
	}
	rpc, address, err := e.resolve(req.Service, req.Method)
	if err != nil {
		return "", err
	}

	msgType, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(rpc.GetRequest()))
	if err != nil {
		return "", fmt.Errorf("request type %s not registered: %w", rpc.GetRequest(), err)
	}
	in := dynamicpb.NewMessage(msgType.Descriptor())
	if len(req.Request) > 0 {
		if err := protojson.Unmarshal(req.Request, in); err != nil {
			return "", fmt.Errorf("decode %s: %w", rpc.GetRequest(), err)
		}
	}

	respType, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(rpc.GetResponse()))
	if err != nil {
		return "", fmt.Errorf("response type %s not registered: %w", rpc.GetResponse(), err)
	}
	out := dynamicpb.NewMessage(respType.Descriptor())

	conn, err := servicekit.Dial(ctx, address)
	if err != nil {
		return "", fmt.Errorf("dial %s: %w", address, err)
	}
	defer conn.Close()

	fullMethod := "/" + rpc.GetService() + "/" + rpc.GetMethod()
	if err := conn.Invoke(ctx, fullMethod, in, out); err != nil {
		return "", fmt.Errorf("call %s: %w", fullMethod, err)
	}
	data, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("encode response: %w", err)
	}
	return string(data), nil
}

func (e *Executor) resolve(serviceName, method string) (*servicev1.RpcDescriptor, string, error) {
	for _, desc := range e.descriptors {
		if desc.GetAddress() == "" {
			continue
		}
		for _, rpc := range desc.GetRpcs() {
			if rpc.GetMethod() != method {
				continue
			}
			if serviceName == desc.GetName() || serviceName == desc.GetType() || serviceName == rpc.GetService() {
				return rpc, desc.GetAddress(), nil
			}
		}
	}
	return nil, "", fmt.Errorf("service rpc not found: service=%q method=%q", serviceName, method)
}
