package httpserver

import (
	"context"
	"fmt"
	"net/http"
)

// Server wraps a stdlib HTTP server.
type Server struct {
	srv *http.Server
}

// New creates an HTTP server bound to host:port using the given handler.
func New(host string, port int, handler http.Handler) *Server {
	return &Server{
		srv: &http.Server{
			Addr:    fmt.Sprintf("%s:%d", host, port),
			Handler: handler,
		},
	}
}

func (s *Server) Start() error {
	return s.srv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
