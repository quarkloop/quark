package agentapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
)

const (
	defaultActivityLimit = 128
	maxActivityLimit     = 1024
)

// Service defines the contract that both the direct agent runtime and the
// api-server proxy must implement. Each method corresponds to one HTTP
// endpoint in the agent API.
type Service interface {
	// Health performs a lightweight liveness check. Returns the agent ID
	// and status. Should be fast enough for port-scanning discovery.
	Health(ctx context.Context, r *http.Request) (*HealthResponse, error)

	// Info returns agent metadata: configured LLM provider and model,
	// current working mode, and list of registered tools.
	Info(ctx context.Context, r *http.Request) (*InfoResponse, error)

	// Mode returns the agent's current working mode (ask/plan/masterplan/auto).
	Mode(ctx context.Context, r *http.Request) (*ModeResponse, error)

	// Stats returns detailed agent statistics including context window
	// metrics, token consumption, and compaction history.
	Stats(ctx context.Context, r *http.Request) (StatsResponse, error)

	// Chat processes a user message and returns the agent's reply.
	// The mode, streaming flag, and file attachments are taken from the request.
	Chat(ctx context.Context, r *http.Request, req ChatRequest) (*ChatResponse, error)

	// Stop requests a graceful agent shutdown. Returns nil on acceptance.
	Stop(ctx context.Context, r *http.Request) error

	// Plan returns the agent's current execution plan, or an error if
	// no plan exists.
	Plan(ctx context.Context, r *http.Request) (*Plan, error)

	// Activity returns the most recent activity records up to limit.
	Activity(ctx context.Context, r *http.Request, limit int) ([]ActivityRecord, error)

	// StreamActivity opens a long-lived connection and calls emit for
	// each new activity record. Blocks until the context is cancelled.
	StreamActivity(ctx context.Context, r *http.Request, emit func(ActivityRecord) error) error

	// Sessions returns all sessions.
	Sessions(ctx context.Context, r *http.Request) ([]SessionRecord, error)

	// Session returns a specific session by key.
	Session(ctx context.Context, r *http.Request, sessionKey string) (*SessionRecord, error)

	// SessionActivity returns activity records for a specific session.
	SessionActivity(ctx context.Context, r *http.Request, sessionKey string, limit int) ([]ActivityRecord, error)

	// CreateSession creates a new session.
	CreateSession(ctx context.Context, r *http.Request, req CreateSessionRequest) (*CreateSessionResponse, error)

	// DeleteSession deletes a session by key.
	DeleteSession(ctx context.Context, r *http.Request, sessionKey string) error

	// ApprovePlan sets the current plan's status to "approved".
	ApprovePlan(ctx context.Context, r *http.Request) (*Plan, error)

	// RejectPlan removes the current draft plan.
	RejectPlan(ctx context.Context, r *http.Request) error
}

// HTTPError is a structured error that carries an HTTP status code.
// The handler layer uses errors.As to extract it and write the appropriate
// HTTP response.
type HTTPError struct {
	StatusCode int
	Message    string
	Err        error
}

func (e *HTTPError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return http.StatusText(e.StatusCode)
}

func (e *HTTPError) Unwrap() error { return e.Err }

// Error constructs an HTTPError with the given status code and message.
func Error(statusCode int, message string, err error) *HTTPError {
	return &HTTPError{StatusCode: statusCode, Message: message, Err: err}
}

// HandlerOption configures the Handler via the functional options pattern.
type HandlerOption func(*handlerConfig)

type handlerConfig struct {
	basePaths    []string
	defaultLimit int
	maxLimit     int
}

// WithBasePath overrides the default base path for route registration.
func WithBasePath(path string) HandlerOption {
	return func(cfg *handlerConfig) {
		cfg.basePaths = []string{path}
	}
}

// WithAliasBasePath adds an additional base path. Routes are registered
// under both the primary and alias paths (used for root "/" alias).
func WithAliasBasePath(path string) HandlerOption {
	return func(cfg *handlerConfig) {
		cfg.basePaths = append(cfg.basePaths, path)
	}
}

// WithActivityLimits overrides the default and maximum activity record
// limits for GET /activity.
func WithActivityLimits(defaultLimit, maxLimit int) HandlerOption {
	return func(cfg *handlerConfig) {
		if defaultLimit > 0 {
			cfg.defaultLimit = defaultLimit
		}
		if maxLimit > 0 {
			cfg.maxLimit = maxLimit
		}
	}
}

// Handler adapts a Service implementation to net/http routes.
type Handler struct {
	service Service
	cfg     handlerConfig
}

// NewHandler creates a Handler that delegates to the given Service.
func NewHandler(service Service, opts ...HandlerOption) *Handler {
	cfg := handlerConfig{
		basePaths:    []string{DefaultBasePath},
		defaultLimit: defaultActivityLimit,
		maxLimit:     maxActivityLimit,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &Handler{service: service, cfg: cfg}
}

// RegisterRoutes registers all agent API routes on the given ServeMux.
// Routes are registered under each configured base path.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	for _, basePath := range h.cfg.basePaths {
		mux.HandleFunc("GET "+JoinPath(basePath, PathHealth), h.handleHealth)
		mux.HandleFunc("GET "+JoinPath(basePath, PathInfo), h.handleInfo)
		mux.HandleFunc("GET "+JoinPath(basePath, PathMode), h.handleMode)
		mux.HandleFunc("GET "+JoinPath(basePath, PathStats), h.handleStats)
		mux.HandleFunc("POST "+JoinPath(basePath, PathChat), h.handleChat)
		mux.HandleFunc("POST "+JoinPath(basePath, PathStop), h.handleStop)
		mux.HandleFunc("GET "+JoinPath(basePath, PathPlan), h.handlePlan)
		mux.HandleFunc("POST "+JoinPath(basePath, PathPlanApprove), h.handlePlanApprove)
		mux.HandleFunc("POST "+JoinPath(basePath, PathPlanReject), h.handlePlanReject)
		mux.HandleFunc("GET "+JoinPath(basePath, PathActivity), h.handleActivity)
		mux.HandleFunc("GET "+JoinPath(basePath, PathActivityStream), h.handleActivityStream)
		mux.HandleFunc("GET "+JoinPath(basePath, PathSessions), h.handleSessions)
		mux.HandleFunc("POST "+JoinPath(basePath, PathSessions), h.handleCreateSession)
		mux.HandleFunc("GET "+JoinPath(basePath, PathSession), h.handleSession)
		mux.HandleFunc("DELETE "+JoinPath(basePath, PathSession), h.handleDeleteSession)
		mux.HandleFunc("GET "+JoinPath(basePath, PathSessionActivity), h.handleSessionActivity)
	}
}

// handleHealth serves GET /health — lightweight liveness probe returning
// the agent ID and status string. Used by discovery port scanning.
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp, err := h.service.Health(r.Context(), r)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleInfo serves GET /info — returns agent metadata including the
// LLM provider, model name, current working mode, and registered tools.
func (h *Handler) handleInfo(w http.ResponseWriter, r *http.Request) {
	resp, err := h.service.Info(r.Context(), r)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleMode serves GET /mode — returns the agent's current working mode
// (ask, plan, masterplan, or auto).
func (h *Handler) handleMode(w http.ResponseWriter, r *http.Request) {
	resp, err := h.service.Mode(r.Context(), r)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleStats serves GET /stats — returns detailed agent statistics
// including context window metrics and token consumption.
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	resp, err := h.service.Stats(r.Context(), r)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleChat serves POST /chat — accepts a user message (JSON or
// multipart/form-data with file attachments) and returns the agent's reply.
// The request may specify a working mode and streaming preference.
func (h *Handler) handleChat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	ct := r.Header.Get("Content-Type")
	mediaType, _, _ := mime.ParseMediaType(ct)

	switch mediaType {
	case "multipart/form-data":
		if err := h.parseChatMultipart(r, &req); err != nil {
			writeError(w, Error(http.StatusBadRequest, err.Error(), err))
			return
		}
	default:
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, Error(http.StatusBadRequest, "invalid request body", err))
			return
		}
	}

	resp, err := h.service.Chat(r.Context(), r, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// parseChatMultipart extracts a ChatRequest from a multipart/form-data body.
// The "message" field provides the text; optional "mode" and "stream" fields
// set those options. All other parts are treated as file attachments.
const maxUploadSize = 32 << 20 // 32 MB

func (h *Handler) parseChatMultipart(r *http.Request, req *ChatRequest) error {
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		return fmt.Errorf("parse multipart: %w", err)
	}
	req.Message = r.FormValue("message")
	req.Mode = r.FormValue("mode")
	if r.FormValue("stream") == "true" {
		req.Stream = true
	}

	for _, headers := range r.MultipartForm.File {
		for _, fh := range headers {
			f, err := fh.Open()
			if err != nil {
				return fmt.Errorf("open file %s: %w", fh.Filename, err)
			}
			data, err := io.ReadAll(f)
			f.Close()
			if err != nil {
				return fmt.Errorf("read file %s: %w", fh.Filename, err)
			}
			ct := fh.Header.Get("Content-Type")
			if ct == "" {
				ct = "application/octet-stream"
			}
			req.Files = append(req.Files, FileAttachment{
				Name:     fh.Filename,
				MimeType: ct,
				Size:     fh.Size,
				Content:  data,
			})
		}
	}
	return nil
}

// handleStop serves POST /stop — requests graceful agent shutdown.
// Returns 202 Accepted on success.
func (h *Handler) handleStop(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Stop(r.Context(), r); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// handlePlan serves GET /plan — returns the agent's current execution
// plan including goal, steps, approval status, and completion state.
func (h *Handler) handlePlan(w http.ResponseWriter, r *http.Request) {
	resp, err := h.service.Plan(r.Context(), r)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleActivity serves GET /activity — returns historical activity
// records. Accepts an optional ?limit= query parameter (default 128,
// max 1024).
func (h *Handler) handleActivity(w http.ResponseWriter, r *http.Request) {
	limit := h.cfg.defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, Error(http.StatusBadRequest, "invalid activity limit", err))
			return
		}
		limit = parsed
	}
	if limit <= 0 {
		limit = h.cfg.defaultLimit
	}
	if limit > h.cfg.maxLimit {
		limit = h.cfg.maxLimit
	}
	resp, err := h.service.Activity(r.Context(), r, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleActivityStream serves GET /activity/stream — opens a Server-Sent
// Events connection that pushes real-time activity records. Each SSE frame
// contains a JSON-encoded ActivityRecord. The connection stays open until
// the client disconnects.
func (h *Handler) handleActivityStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, Error(http.StatusInternalServerError, "streaming unsupported", nil))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	if err := h.service.StreamActivity(r.Context(), r, func(record ActivityRecord) error {
		payload, err := json.Marshal(record)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}); err != nil && !errors.Is(err, context.Canceled) {
		writeError(w, err)
	}
}

func writeJSON(w http.ResponseWriter, statusCode int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(body)
}

// handleSessions serves GET /sessions — returns all sessions.
func (h *Handler) handleSessions(w http.ResponseWriter, r *http.Request) {
	resp, err := h.service.Sessions(r.Context(), r)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleCreateSession serves POST /sessions — creates a new session.
func (h *Handler) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, Error(http.StatusBadRequest, "invalid request body", err))
		return
	}
	resp, err := h.service.CreateSession(r.Context(), r, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

// handleSession serves GET /sessions/{sessionKey} — returns a single session.
func (h *Handler) handleSession(w http.ResponseWriter, r *http.Request) {
	sessionKey := r.PathValue("sessionKey")
	resp, err := h.service.Session(r.Context(), r, sessionKey)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleDeleteSession serves DELETE /sessions/{sessionKey}.
func (h *Handler) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	sessionKey := r.PathValue("sessionKey")
	if err := h.service.DeleteSession(r.Context(), r, sessionKey); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSessionActivity serves GET /sessions/{sessionKey}/activity — returns
// activity records scoped to a specific session.
func (h *Handler) handleSessionActivity(w http.ResponseWriter, r *http.Request) {
	sessionKey := r.PathValue("sessionKey")
	limit := h.cfg.defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, Error(http.StatusBadRequest, "invalid limit", err))
			return
		}
		limit = parsed
	}
	if limit <= 0 {
		limit = h.cfg.defaultLimit
	}
	if limit > h.cfg.maxLimit {
		limit = h.cfg.maxLimit
	}
	resp, err := h.service.SessionActivity(r.Context(), r, sessionKey, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handlePlanApprove serves POST /plan/approve — sets the plan status to approved.
func (h *Handler) handlePlanApprove(w http.ResponseWriter, r *http.Request) {
	resp, err := h.service.ApprovePlan(r.Context(), r)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handlePlanReject serves POST /plan/reject — removes the current draft plan.
func (h *Handler) handlePlanReject(w http.ResponseWriter, r *http.Request) {
	if err := h.service.RejectPlan(r.Context(), r); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeError(w http.ResponseWriter, err error) {
	statusCode := http.StatusInternalServerError
	message := err.Error()
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		if httpErr.StatusCode > 0 {
			statusCode = httpErr.StatusCode
		}
		if httpErr.Message != "" {
			message = httpErr.Message
		}
	}
	writeJSON(w, statusCode, ErrorResponse{Error: message})
}
