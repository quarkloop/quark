package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/quarkloop/agent/pkg/infra/signals"
	"github.com/quarkloop/api-server/pkg/server"
)

func main() {
	cfg := server.DefaultConfig()
	flag.StringVar(&cfg.Host, "host", cfg.Host, "Host to listen on")
	flag.IntVar(&cfg.Port, "port", cfg.Port, "Port to listen on")
	flag.StringVar(&cfg.DataDir, "data-dir", cfg.DataDir, "Data directory")
	flag.Parse()

	ctx, cancel := signals.NotifyContext(context.Background())
	defer cancel()

	srv, err := server.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "api-server init: %v\n", err)
		os.Exit(1)
	}
	defer srv.Close()

	log.Printf("quark api-server v0.1.0")
	if err := srv.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "api-server: %v\n", err)
		os.Exit(1)
	}
}
