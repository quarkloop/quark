package api

import (
	"context"
)

// --- Initialization & Validation ---

func (a *ClientApi) InitRepo(ctx context.Context, dir string) error {
	req := InitRepoRequest{Dir: dir}
	return a.client.Post(ctx, "/api/v1/repo/init", req, nil)
}

func (a *ClientApi) LockRepo(ctx context.Context, dir string) error {
	req := LockRepoRequest{Dir: dir}
	return a.client.Post(ctx, "/api/v1/repo/lock", req, nil)
}

func (a *ClientApi) ValidateRepo(ctx context.Context, dir string) error {
	req := ValidateRepoRequest{Dir: dir}
	return a.client.Post(ctx, "/api/v1/repo/validate", req, nil)
}

// --- Agent Management ---

func (a *ClientApi) AddAgent(ctx context.Context, dir, ref, name string) error {
	req := AgentAddRequest{Dir: dir, Ref: ref, Name: name}
	return a.client.Post(ctx, "/api/v1/repo/agents", req, nil)
}

func (a *ClientApi) RemoveAgent(ctx context.Context, dir, name string) error {
	req := AgentRemoveRequest{Dir: dir, Name: name}
	// Note: client.Delete uses `in` as body payload
	return a.client.Delete(ctx, "/api/v1/repo/agents/"+name, req, nil)
}

func (a *ClientApi) ListAgents(ctx context.Context, dir string) (*AgentListResponse, error) {
	var res AgentListResponse
	if err := a.client.Get(ctx, "/api/v1/repo/agents?dir="+dir, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// --- Tool Management ---

func (a *ClientApi) AddTool(ctx context.Context, dir, ref, name string) error {
	req := ToolAddRequest{Dir: dir, Ref: ref, Name: name}
	return a.client.Post(ctx, "/api/v1/repo/tools", req, nil)
}

func (a *ClientApi) RemoveTool(ctx context.Context, dir, name string) error {
	req := ToolRemoveRequest{Dir: dir, Name: name}
	return a.client.Delete(ctx, "/api/v1/repo/tools/"+name, req, nil)
}

func (a *ClientApi) ListTools(ctx context.Context, dir string) (*ToolListResponse, error) {
	var res ToolListResponse
	if err := a.client.Get(ctx, "/api/v1/repo/tools?dir="+dir, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// --- KB Disk Management ---

func (a *ClientApi) AddKB(ctx context.Context, dir, path string, content []byte) error {
	req := KBAddRequest{Dir: dir, Path: path, Value: content}
	return a.client.Post(ctx, "/api/v1/repo/kb", req, nil)
}

func (a *ClientApi) RemoveKB(ctx context.Context, dir, path string) error {
	req := KBRemoveRequest{Dir: dir, Path: path}
	// Path encoded in body due to slashes
	return a.client.Delete(ctx, "/api/v1/repo/kb", req, nil)
}

func (a *ClientApi) ListKB(ctx context.Context, dir string) (*KBListResponse, error) {
	var res KBListResponse
	if err := a.client.Get(ctx, "/api/v1/repo/kb?dir="+dir, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (a *ClientApi) ShowKB(ctx context.Context, dir, path string) (*KBShowResponse, error) {
	var res KBShowResponse
	if err := a.client.Get(ctx, "/api/v1/repo/kb/show?dir="+dir+"&path="+path, &res); err != nil {
		return nil, err
	}
	return &res, nil
}
