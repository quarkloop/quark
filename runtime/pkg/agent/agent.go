// Package agent provides the core agent with typed message loop.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/quarkloop/agent/pkg/channel"
	"github.com/quarkloop/agent/pkg/execution"
	"github.com/quarkloop/agent/pkg/hierarchy"
	"github.com/quarkloop/agent/pkg/llm"
	"github.com/quarkloop/agent/pkg/loop"
	"github.com/quarkloop/agent/pkg/message"
	"github.com/quarkloop/agent/pkg/permissions"
	"github.com/quarkloop/agent/pkg/plan"
	"github.com/quarkloop/agent/pkg/pluginmanager"
	"github.com/quarkloop/agent/pkg/prompt"
	"github.com/quarkloop/agent/pkg/session"
	"github.com/quarkloop/pkg/plugin"
	"github.com/quarkloop/supervisor/pkg/api"
	supclient "github.com/quarkloop/supervisor/pkg/client"
)

// Config holds agent configuration.
type Config struct {
	ID            string
	Name          string
	Description   string
	ModelProvider string
	Model         string
	ModelListURL  string
	PluginsDir    string

	// Execution mode configuration
	ExecutionMode execution.Mode
	ExecutionCfg  execution.Config

	// Permission policy
	PermissionPolicy *permissions.Policy

	// Supervisor configuration (optional - for agents running under supervisor)
	SupervisorURL string
	SpaceID       string
}

// Agent is the main agent instance with a typed message loop.
type Agent struct {
	ID       string
	loop     *loop.Loop
	Sessions *session.Registry
	Plan     *plan.Plan
	Models   *llm.Registry
	Plugins  *pluginmanager.Manager
	Bus      *channel.ChannelBus
	config   Config

	// Hierarchy management
	identity  *hierarchy.Identity
	agents    *hierarchy.Registry
	delegator *hierarchy.Delegator

	// Execution runtime
	execution *execution.Runtime

	// Permission checker
	permissions *permissions.Checker

	// Supervisor client for all space-scoped data operations.
	// Nil only when the agent is running standalone (no supervisor URL).
	supervisorClient *supclient.Client
	// Space is the space name this agent serves; empty when standalone.
	Space string
}

// NewAgent creates a new Agent from config.
func NewAgent(cfg Config) (*Agent, error) {
	// Create the message loop
	l := loop.New(
		loop.WithInboxSize(64),
		loop.WithWorkQueueSize(32),
		loop.WithWorkPriority(true),
		loop.WithUnhandledCallback(func(msg loop.Message) {
			slog.Info("unhandled message", "type", msg.Type())
		}),
	)

	// Create execution runtime
	execCfg := cfg.ExecutionCfg
	if execCfg.Mode == "" {
		execCfg.Mode = execution.ModeAutonomous
	}
	execRuntime, err := execution.NewRuntime(execCfg)
	if err != nil {
		return nil, fmt.Errorf("execution runtime: %w", err)
	}

	// Create hierarchy registry
	agentRegistry := hierarchy.NewRegistry()

	// Determine plugins directory
	pluginsDir := cfg.PluginsDir
	if pluginsDir == "" {
		pluginsDir = "plugins"
	}

	a := &Agent{
		ID:          cfg.ID,
		loop:        l,
		Sessions:    session.NewRegistry(),
		Plan:        plan.New(),
		Models:      llm.NewRegistry(),
		Plugins:     pluginmanager.NewManager(pluginsDir),
		config:      cfg,
		agents:      agentRegistry,
		delegator:   hierarchy.NewDelegator(agentRegistry),
		execution:   execRuntime,
		permissions: permissions.NewChecker(cfg.PermissionPolicy),
	}

	// Create supervisor client if URL is provided
	if cfg.SupervisorURL != "" {
		a.supervisorClient = supclient.New(supclient.WithBaseURL(cfg.SupervisorURL))
		a.Space = cfg.SpaceID
	}

	// Register as main agent
	name := cfg.Name
	if name == "" {
		name = "Main Agent"
	}
	entry, err := agentRegistry.RegisterMain(cfg.ID, name, cfg.Description, hierarchy.DefaultPermissions())
	if err != nil {
		return nil, fmt.Errorf("register main agent: %w", err)
	}
	a.identity = entry.Identity

	// Register this agent's loop
	a.delegator.RegisterLoop(cfg.ID, l)

	// Register handlers
	a.registerHandlers()

	// Configure execution middleware
	execRuntime.ConfigureLoop(l)

	// Add permission middleware if policy is set
	if cfg.PermissionPolicy != nil {
		l.Use(permissions.ToolMiddleware(a.permissions))
	}

	// Add recovery middleware
	l.Use(loop.RecoveryMiddleware)

	// Add observer middleware for logging
	l.Use(loop.ObserverMiddleware(func(msgType string, err error) {
		if err != nil {
			slog.Error("handler error", "type", msgType, "error", err)
		}
	}))

	return a, nil
}

// registerHandlers registers all message handlers.
func (a *Agent) registerHandlers() {
	a.loop.Register(MsgTypeUserMessage, a.handleUserMessage)
	a.loop.Register(MsgTypeInitLLM, a.handleInitLLM)
	a.loop.Register(MsgTypeInitChannel, a.handleInitChannel)
	a.loop.Register(MsgTypeSetModel, a.handleSetModel)
	a.loop.Register(MsgTypeWorkStep, a.handleWorkStep)
	a.loop.Register(MsgTypeToolCall, a.handleToolCall)

	// Register work item handler for delegation
	a.loop.Register("work_item", hierarchy.WorkHandler(a.agents, a.processWork))
}

// Post sends a user message to the agent.
func (a *Agent) Post(sessionID, content string, resp chan message.StreamMessage) {
	a.loop.Send(NewUserMessage(sessionID, content, resp))
}

// Send sends a typed message to the agent loop.
func (a *Agent) Send(msg loop.Message) {
	a.loop.Send(msg)
}

// Run starts the agent's main loop.
func (a *Agent) Run(ctx context.Context) error {
	slog.Info("main loop started", "agent_id", a.ID)

	// Initialize loads both tool and provider plugins.
	if err := a.Plugins.Initialize(ctx); err != nil {
		slog.Error("failed to initialize plugins", "error", err)
	}
	defer a.Plugins.Shutdown()

	// Update agent status
	a.agents.SetStatus(a.ID, hierarchy.StatusRunning)
	defer a.agents.SetStatus(a.ID, hierarchy.StatusComplete)

	// Send initialization messages
	a.sendInitMessages()

	// Start work step ticker for plan execution
	go a.workStepTicker(ctx)

	// Subscribe to supervisor events (session lifecycle, shutdown, etc).
	// This is the agent's only mechanism for learning about sessions — the
	// agent no longer exposes its own session CRUD API.
	go a.subscribeSupervisorEvents(ctx)

	// Run the loop
	return a.loop.Run(ctx)
}

// sendInitMessages queues initialization messages at startup.
func (a *Agent) sendInitMessages() {
	// Get providers loaded from plugins
	providers := a.Plugins.GetProviders()

	// Log loaded providers
	if len(providers) == 0 {
		slog.Warn("no providers loaded from plugins")
	}
	for id := range providers {
		slog.Info("provider available", "id", id)
	}

	fallback := []llm.ModelEntry{}
	if a.config.Model != "" {
		fallback = append(fallback, llm.ModelEntry{
			ID:       a.config.Model,
			Provider: a.config.ModelProvider,
			Name:     a.config.Model,
			Default:  true,
		})
	}

	msg := NewInitLLMMsg()
	msg.ModelListURL = a.config.ModelListURL
	msg.Providers = make(map[string]any)
	for k, v := range providers {
		msg.Providers[k] = v
	}
	msg.Fallback = make([]any, len(fallback))
	for i, e := range fallback {
		msg.Fallback[i] = e
	}

	a.loop.Send(msg)
}

// workStepTicker periodically triggers work step execution.
func (a *Agent) workStepTicker(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check if there's work to do
			select {
			case <-a.Plan.NextStep():
				a.loop.Send(NewWorkStepMsg())
			default:
			}
		}
	}
}

// handleUserMessage processes an incoming user message.
func (a *Agent) handleUserMessage(ctx context.Context, msg loop.Message) error {
	userMsg := msg.(UserMessageMsg)
	defer close(userMsg.Response)

	s := a.Sessions.Get(userMsg.SessionID)
	if s == nil {
		s = a.Sessions.GetOrCreate(userMsg.SessionID, "chat", "")
	}

	s.AddMessage("user", userMsg.Content)

	client := a.Models.GetDefault()
	if client == nil {
		return fmt.Errorf("no LLM client configured")
	}

	history := s.GetMessages()
	fullResponse, err := message.Handle(
		ctx,
		history,
		client,
		prompt.GetSystemPrompt(),
		a.Plan.GetSummary(),
		a.defaultTools(),
		a.executeTool,
		userMsg.Response,
	)
	if err != nil {
		userMsg.Response <- message.StreamMessage{
			Type: "error",
			Data: fmt.Sprintf("Agent Error: %v", err),
		}
		return err
	}

	s.AddMessage("assistant", fullResponse)
	return nil
}

// handleInitLLM initializes or reinitializes LLM models.
func (a *Agent) handleInitLLM(ctx context.Context, msg loop.Message) error {
	payload := msg.(InitLLMMsg)
	slog.Info("initializing LLM models")

	providers := make(map[string]llm.Provider)
	for k, v := range payload.Providers {
		if p, ok := v.(llm.Provider); ok {
			providers[k] = p
		}
	}

	if payload.ModelListURL != "" {
		if err := a.Models.LoadFromURL(payload.ModelListURL, providers); err != nil {
			slog.Warn("remote model list failed, using fallback", "error", err)
		}
	}

	// Fallback: load from config if registry is empty
	if a.Models.GetDefault() == nil && len(payload.Fallback) > 0 {
		entries := make([]llm.ModelEntry, 0, len(payload.Fallback))
		for _, e := range payload.Fallback {
			if entry, ok := e.(llm.ModelEntry); ok {
				entries = append(entries, entry)
			}
		}
		if len(entries) > 0 {
			if err := a.Models.LoadEntries(entries, providers); err != nil {
				slog.Error("fallback model init failed", "error", err)
			}
		}
	}

	if client := a.Models.GetDefault(); client != nil {
		slog.Info("LLM ready", "default_model", a.Models.Default)
	} else {
		slog.Warn("no LLM models available")
	}

	return nil
}

// handleInitChannel processes channel state changes.
func (a *Agent) handleInitChannel(ctx context.Context, msg loop.Message) error {
	payload := msg.(InitChannelMsg)
	if bus, ok := payload.Bus.(*channel.ChannelBus); ok {
		a.Bus = bus
		slog.Info("channel bus registered", "active_channels", len(a.Bus.ActiveChannels()))
	}
	return nil
}

// handleSetModel dynamically changes the active LLM model.
func (a *Agent) handleSetModel(ctx context.Context, msg loop.Message) error {
	payload := msg.(SetModelMsg)
	if a.Models.SetDefault(payload.ModelID) {
		slog.Info("switched default model", "model_id", payload.ModelID)
	} else {
		slog.Warn("model not found in registry", "model_id", payload.ModelID)
	}
	return nil
}

// handleWorkStep executes the next autonomous work step.
func (a *Agent) handleWorkStep(ctx context.Context, msg loop.Message) error {
	client := a.Models.GetDefault()
	if client == nil {
		return nil
	}

	infer := func(ictx context.Context, msgs []plugin.Message, resp chan<- string) (string, error) {
		return client.Infer(ictx, msgs, a.defaultTools(), a.executeTool, func(msgType string, data any) {
			if resp != nil && msgType == "token" {
				if s, ok := data.(string); ok {
					resp <- s
				}
			}
		})
	}

	if err := a.Plan.ExecuteStep(ctx, infer, prompt.GetSystemPrompt()); err != nil {
		slog.Error("work step error", "error", err)
		return err
	}
	return nil
}

// handleToolCall executes a tool call (with permission checking via middleware).
func (a *Agent) handleToolCall(ctx context.Context, msg loop.Message) error {
	toolMsg := msg.(ToolCallMsg)

	// Check permissions (additional check beyond middleware)
	if err := a.permissions.ValidateTool(toolMsg.Tool); err != nil {
		toolMsg.ResultChan <- ToolResult{Error: err}
		return err
	}

	result, err := a.Plugins.ExecuteTool(ctx, toolMsg.Tool, toolMsg.Arguments)
	toolMsg.ResultChan <- ToolResult{Output: result, Error: err}
	return err
}

// processWork processes delegated work from a sub-agent.
func (a *Agent) processWork(ctx context.Context, agentID, task string) (string, error) {
	client := a.Models.GetDefault()
	if client == nil {
		return "", fmt.Errorf("no LLM client configured")
	}

	// Simple inference for sub-agent work
	msgs := []plugin.Message{
		{Role: "system", Content: prompt.GetSystemPrompt()},
		{Role: "user", Content: task},
	}

	return client.Infer(ctx, msgs, a.defaultTools(), a.executeTool, nil)
}

// defaultTools returns the available tools.
func (a *Agent) defaultTools() []plugin.ToolSchema {
	return a.Plugins.GetTools()
}

// executeTool executes a requested tool via pluginmanager.
func (a *Agent) executeTool(ctx context.Context, name, arguments string) (string, error) {
	// Check permissions
	if err := a.permissions.ValidateTool(name); err != nil {
		return "", err
	}
	return a.Plugins.ExecuteTool(ctx, name, arguments)
}

// Identity returns the agent's hierarchy identity.
func (a *Agent) Identity() *hierarchy.Identity {
	return a.identity
}

// Agents returns the hierarchy registry.
func (a *Agent) Agents() *hierarchy.Registry {
	return a.agents
}

// Delegator returns the work delegator.
func (a *Agent) Delegator() *hierarchy.Delegator {
	return a.delegator
}

// Execution returns the execution runtime.
func (a *Agent) Execution() *execution.Runtime {
	return a.execution
}

// Permissions returns the permission checker.
func (a *Agent) Permissions() *permissions.Checker {
	return a.permissions
}

// SpawnSubAgent spawns a new sub-agent with the given configuration.
func (a *Agent) SpawnSubAgent(config *hierarchy.SpawnConfig) (*hierarchy.AgentEntry, error) {
	return a.agents.Spawn(a.ID, config)
}

// DelegateWork delegates work to a sub-agent.
func (a *Agent) DelegateWork(ctx context.Context, agentID, task string, timeout time.Duration) (hierarchy.WorkResult, error) {
	work := hierarchy.WorkItem{
		BaseMessage: loop.NewPriorityMessage("work_item", 5),
		AgentID:     agentID,
		Task:        task,
		Timeout:     timeout,
	}
	return a.delegator.DelegateAndWait(ctx, a.ID, work)
}

// Supervisor returns the supervisor client, or nil if the agent is running
// standalone.
func (a *Agent) Supervisor() *supclient.Client {
	return a.supervisorClient
}

// HasSupervisor returns true if the agent is running under a supervisor.
func (a *Agent) HasSupervisor() bool {
	return a.supervisorClient != nil
}

// subscribeSupervisorEvents consumes the supervisor's space event stream and
// mirrors session lifecycle events into the local in-memory registry so the
// agent can serve messages for sessions the supervisor has created. The call
// returns when ctx is cancelled or the stream terminates; callers should
// reconnect with backoff.
func (a *Agent) subscribeSupervisorEvents(ctx context.Context) {
	if a.supervisorClient == nil || a.Space == "" {
		slog.Info("supervisor event stream disabled", "client", a.supervisorClient != nil, "space", a.Space)
		return
	}
	slog.Info("subscribing to supervisor events", "space", a.Space)
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		err := a.supervisorClient.StreamEventsWithReady(ctx, a.Space,
			func() { slog.Info("supervisor event stream ready", "space", a.Space) },
			func(ev api.Event) { a.applyEvent(ev) },
		)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			slog.Error("supervisor event stream error, retrying", "error", err, "retry_in", backoff)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

// applyEvent updates agent runtime state in response to a supervisor event.
func (a *Agent) applyEvent(ev api.Event) {
	switch ev.Kind {
	case api.EventSessionCreated:
		var p struct {
			ID    string `json:"id"`
			Type  string `json:"type"`
			Title string `json:"title"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil || p.ID == "" {
			return
		}
		a.Sessions.GetOrCreate(p.ID, p.Type, p.Title)
		slog.Info("session created", "id", p.ID, "type", p.Type)
	case api.EventSessionDeleted:
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil || p.ID == "" {
			return
		}
		a.Sessions.Delete(p.ID)
		slog.Info("session deleted", "id", p.ID)
	}
}
