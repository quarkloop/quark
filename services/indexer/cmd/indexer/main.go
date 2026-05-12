package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/quarkloop/services/indexer/internal/app"
	"github.com/quarkloop/services/indexer/internal/dgraph"
)

func main() {
	var addr string
	var dgraphAddr string
	var skillDir string
	flag.StringVar(&addr, "addr", "127.0.0.1:7301", "gRPC listen address")
	flag.StringVar(&dgraphAddr, "dgraph", "127.0.0.1:9080", "Dgraph Alpha gRPC address")
	flag.StringVar(&skillDir, "skill-dir", "", "directory containing the service SKILL.md")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	driver, err := dgraph.New(context.Background(), dgraph.Config{
		Address: dgraphAddr,
		Logger:  logger,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := app.Run(context.Background(), app.Config{
		Address:  addr,
		Driver:   driver,
		SkillDir: skillDir,
		Logger:   logger,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
