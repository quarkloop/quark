package agentapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

const (
	defaultActivityLimit = 128
	maxActivityLimit     = 1024
)

type Service interface {
	Health(ctx context.Context, r *http.Request) (*HealthResponse, error)
	Mode(ctx context.Context, r *http.Request) (*ModeResponse, error)
	Stats(ctx context.Context, r *http.Request) (StatsResponse, error)
	Chat(ctx context.Context, r *http.Request, req ChatRequest) (*ChatResponse, error)
	Stop(ctx context.Context, r *http.Request) error
	Plan(ctx context.Context, r *http.Request) (*Plan, error)
	Activity(ctx context.Context, r *http.Request, limit int) ([]ActivityRecord, error)
	StreamActivity(ctx context.Context, r *http.Request, emit func(ActivityRecord) error) error
}

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

func Error(statusCode int, message string, err error) *HTTPError {
	return &HTTPError{StatusCode: statusCode, Message: message, Err: err}
}

type HandlerOption func(*handlerConfig)

type handlerConfig struct {
	basePaths    []string
	defaultLimit int
	maxLimit     int
}

func WithBasePath(path string) HandlerOption {
	return func(cfg *handlerConfig) {
		cfg.basePaths = []string{path}
	}
}

func WithAliasBasePath(path string) HandlerOption {
	return func(cfg *handlerConfig) {
		cfg.basePaths = append(cfg.basePaths, path)
	}
}

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

type Handler struct {
	service Service
	cfg     handlerConfig
}

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

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	for _, basePath := range h.cfg.basePaths {
		mux.HandleFunc("GET "+JoinPath(basePath, PathHealth), h.handleHealth)
		mux.HandleFunc("GET "+JoinPath(basePath, PathMode), h.handleMode)
		mux.HandleFunc("GET "+JoinPath(basePath, PathStats), h.handleStats)
		mux.HandleFunc("POST "+JoinPath(basePath, PathChat), h.handleChat)
		mux.HandleFunc("POST "+JoinPath(basePath, PathStop), h.handleStop)
		mux.HandleFunc("GET "+JoinPath(basePath, PathPlan), h.handlePlan)
		mux.HandleFunc("GET "+JoinPath(basePath, PathActivity), h.handleActivity)
		mux.HandleFunc("GET "+JoinPath(basePath, PathActivityStream), h.handleActivityStream)
	}
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp, err := h.service.Health(r.Context(), r)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleMode(w http.ResponseWriter, r *http.Request) {
	resp, err := h.service.Mode(r.Context(), r)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	resp, err := h.service.Stats(r.Context(), r)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleChat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, Error(http.StatusBadRequest, "invalid request body", err))
		return
	}
	resp, err := h.service.Chat(r.Context(), r, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleStop(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Stop(r.Context(), r); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) handlePlan(w http.ResponseWriter, r *http.Request) {
	resp, err := h.service.Plan(r.Context(), r)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

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

func writeJSON(w http.ResponseWriter, statusCode int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(body)
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
