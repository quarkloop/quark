// Package runtime manages the application lifecycle with graceful shutdown.
package runtime

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/quarkloop/runtime/pkg/channel"
)

// Server manages the ChannelBus lifecycle.
type Server struct {
	bus *channel.ChannelBus
}

// NewServer creates a new Server with a ChannelBus.
func NewServer() *Server {
	return &Server{bus: channel.NewChannelBus()}
}

// Bus returns the ChannelBus for channel registration.
func (s *Server) Bus() *channel.ChannelBus {
	return s.bus
}

// Run starts all channels via the ChannelBus and blocks until SIGINT/SIGTERM.
func (s *Server) Run(ctx context.Context) error {
	if err := s.bus.Start(ctx); err != nil {
		return fmt.Errorf("start channels: %w", err)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	fmt.Println("\nshutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return s.bus.Stop(shutdownCtx)
}
