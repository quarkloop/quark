package commands

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/quarkloop/agent/pkg/agent"
	"github.com/quarkloop/agent/pkg/channel/telegram"
	"github.com/quarkloop/agent/pkg/channel/web"
	"github.com/quarkloop/agent/pkg/runtime"
)

const CmdStartDefaultPort = 8765

// Start creates the "agent start" command.
func Start() *cobra.Command {
	var port int
	var channelsFlag []string

	cmd := &cobra.Command{
		Use:           "start [channels...]",
		Short:         "start the agent runtime",
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
	loadEnvFiles()

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

	log.Println("Starting agent runtime...")
	log.Printf("Enabled channels: %v", validChannels)

	model := os.Getenv("OPENROUTER_MODEL")
	if model == "" {
		model = "openai/gpt-4o-mini"
	}
	log.Printf("Using model: %s", model)

	// Create agent
	a, err := agent.NewAgent(agent.Config{
		ID:            "main",
		Name:          "Main Agent",
		Model:         model,
		ModelListURL:  os.Getenv("MODEL_LIST_URL"),
		OpenRouterKey: os.Getenv("OPENROUTER_API_KEY"),
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
			log.Printf("Registering [web] channel on %s", listenAddr)
			srv.Bus().Register(web.New(listenAddr, a))

		case "telegram":
			token := os.Getenv("TELEGRAM_BOT_TOKEN")
			if token == "" {
				return fmt.Errorf("TELEGRAM_BOT_TOKEN environment variable is required for the telegram channel")
			}

			log.Printf("Registering [telegram] channel")
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

	log.Println("Starting agent background loop...")
	go a.Run(ctx)

	log.Println("Agent server is running. Press Ctrl+C to exit.")
	// Start all channels via ChannelBus and block
	return srv.Run(ctx)
}
