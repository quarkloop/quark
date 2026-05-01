package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/quarkloop/runtime/pkg/agent"
	"github.com/quarkloop/runtime/pkg/channel/telegram"
	"github.com/quarkloop/runtime/pkg/channel/web"
	"github.com/quarkloop/runtime/pkg/runtime"
)

const CmdStartDefaultPort = 8765

// Start creates the "runtime start" command.
func Start() *cobra.Command {
	var port int
	var channelsFlag []string

	cmd := &cobra.Command{
		Use:           "start [channels...]",
		Short:         "start the runtime",
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			var channels []string

			// If flag was explicitly changed from default, or args are empty, use the flag
			if cmd.Flags().Changed("channel") || len(args) == 0 {
				channels = append(channels, channelsFlag...)
			}

			// Support positional arguments (e.g. `binary start channel web telegram`)
			for _, arg := range args {
				if arg != "channel" && arg != "channels" {
					channels = append(channels, arg)
				}
			}

			if len(channels) == 0 {
				channels = []string{"web"} // Fallback
			}

			return runStart(port, channels)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", CmdStartDefaultPort, "HTTP listen port")
	cmd.Flags().StringSliceVarP(&channelsFlag, "channel", "c", []string{"web"}, "Channels to use (e.g., 'web', 'telegram', 'web,telegram', or 'all')")

	return cmd
}

func runStart(port int, channels []string) error {
	if os.Getenv("QUARK_SUPERVISOR_URL") == "" {
		loadEnvFiles()
	}

	// 1. Deduplicate channels and handle "all"
	activeChannels := make(map[string]bool)
	for _, ch := range channels {
		if ch == "all" {
			activeChannels["web"] = true
			activeChannels["telegram"] = true
		} else {
			activeChannels[ch] = true
		}
	}

	if len(activeChannels) == 0 {
		return fmt.Errorf("no channels specified to start")
	}

	// 2. Early validation: fail fast if any channel is invalid
	var validChannels []string
	for ch := range activeChannels {
		switch ch {
		case "web", "telegram":
			validChannels = append(validChannels, ch)
		default:
			return fmt.Errorf("unknown channel requested: %q", ch)
		}
	}

	slog.Info("starting runtime")
	slog.Info("enabled channels", "channels", fmt.Sprintf("%v", validChannels))

	modelProvider := os.Getenv("QUARK_MODEL_PROVIDER")
	modelName := os.Getenv("QUARK_MODEL_NAME")
	if modelProvider == "" || modelName == "" {
		return fmt.Errorf("model provider and name are required")
	}
	slog.Info("using model", "provider", modelProvider, "model", modelName)

	// Create agent
	a, err := agent.NewAgent(agent.Config{
		ID:            "main",
		Name:          "Main Agent",
		ModelProvider: modelProvider,
		Model:         modelName,
		ModelListURL:  os.Getenv("MODEL_LIST_URL"),
		PluginsDir:    os.Getenv("QUARK_PLUGINS_DIR"),
		SupervisorURL: os.Getenv("QUARK_SUPERVISOR_URL"),
		SpaceID:       os.Getenv("QUARK_SPACE"),
	})
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Create server with ChannelBus
	srv := runtime.NewServer()

	// Wire ChannelBus to agent via typed message
	a.Send(agent.NewInitChannelMsg(srv.Bus()))

	// 3. Instantiate and register the requested channels
	for _, ch := range validChannels {
		switch ch {
		case "web":
			listenAddr := fmt.Sprintf(":%d", port)
			slog.Info("registering web channel", "listen_addr", listenAddr)
			srv.Bus().Register(web.New(listenAddr, a))

		case "telegram":
			token := os.Getenv("TELEGRAM_BOT_TOKEN")
			if token == "" {
				return fmt.Errorf("TELEGRAM_BOT_TOKEN environment variable is required for the telegram channel")
			}

			slog.Info("registering telegram channel")
			srv.Bus().Register(telegram.New(
				telegram.Config{Token: token},
				a,
				func(id, chType, title string) { a.Sessions.GetOrCreate(id, chType, title) },
			))
		}
	}

	// Start agent loop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	slog.Info("starting agent background loop")
	go a.Run(ctx)

	slog.Info("runtime server is running, press Ctrl+C to exit")
	// Start all channels via ChannelBus and block
	return srv.Run(ctx)
}
