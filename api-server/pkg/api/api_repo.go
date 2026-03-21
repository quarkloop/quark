package api

// --- Repo API Payloads ---

type InitRepoRequest struct {
	Dir string `json:"dir"` // Informational or mounting context if needed
}

func (r *InitRepoRequest) GetDir() string { return r.Dir }

type LockRepoRequest struct {
	Dir string `json:"dir"`
}

func (r *LockRepoRequest) GetDir() string { return r.Dir }

type ValidateRepoRequest struct {
	Dir string `json:"dir"`
}

func (r *ValidateRepoRequest) GetDir() string { return r.Dir }

// --- Agent Management Payloads ---

type AgentAddRequest struct {
	Dir  string `json:"dir"`
	Ref  string `json:"ref"`
	Name string `json:"name"`
}

type AgentRemoveRequest struct {
	Dir  string `json:"dir"`
	Name string `json:"name"`
}

type AgentListResponse struct {
	Agents []AgentItem `json:"agents"`
}

type AgentItem struct {
	Name string `json:"name"`
	Ref  string `json:"ref"`
}

// --- Tool Management Payloads ---

type ToolAddRequest struct {
	Dir  string `json:"dir"`
	Ref  string `json:"ref"`
	Name string `json:"name"`
}

type ToolRemoveRequest struct {
	Dir  string `json:"dir"`
	Name string `json:"name"`
}

type ToolListResponse struct {
	Tools []ToolItem `json:"tools"`
}

type ToolItem struct {
	Name string `json:"name"`
	Ref  string `json:"ref"`
}

// --- KB Management Payloads ---

type KBAddRequest struct {
	Dir   string `json:"dir"`
	Path  string `json:"path"`
	Value []byte `json:"value"` // Direct file content transfer
}

type KBRemoveRequest struct {
	Dir  string `json:"dir"`
	Path string `json:"path"`
}

type KBListResponse struct {
	Files []KBFile `json:"files"`
}

type KBFile struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type KBShowResponse struct {
	Content []byte `json:"content"`
}
