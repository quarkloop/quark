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

func (a *ClientApi) ScaffoldRegistry(ctx context.Context) error {
	return a.client.Post(ctx, "/api/v1/registry/scaffold", nil, nil)
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

// --- Skill Management ---

func (a *ClientApi) AddSkill(ctx context.Context, dir, ref, name string) error {
	req := SkillAddRequest{Dir: dir, Ref: ref, Name: name}
	return a.client.Post(ctx, "/api/v1/repo/skills", req, nil)
}

func (a *ClientApi) RemoveSkill(ctx context.Context, dir, name string) error {
	req := SkillRemoveRequest{Dir: dir, Name: name}
	return a.client.Delete(ctx, "/api/v1/repo/skills/"+name, req, nil)
}

func (a *ClientApi) ListSkills(ctx context.Context, dir string) (*SkillListResponse, error) {
	var res SkillListResponse
	if err := a.client.Get(ctx, "/api/v1/repo/skills?dir="+dir, &res); err != nil {
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
