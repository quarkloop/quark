package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	agentapi "github.com/quarkloop/agent-api"
	agentclient "github.com/quarkloop/agent-client"
	"github.com/quarkloop/agent/pkg/infra/term"
)

func streamAgentActivity(ctx context.Context, client *agentclient.Client, fn func(string)) error {
	return client.StreamActivity(ctx, func(record agentapi.ActivityRecord) {
		data, err := json.Marshal(record)
		if err != nil {
			fn(fmt.Sprintf(`{"type":"marshal_error","error":%q}`, err.Error()))
			return
		}
		fn(string(data))
	})
}

func inspectDirectAgent(ctx context.Context, agentURL string) error {
	client := agentclient.New(agentURL)

	health, err := client.Health(ctx)
	if err != nil {
		return err
	}
	mode, err := client.Mode(ctx)
	if err != nil {
		return err
	}

	row := term.SpaceRow{
		ID:        firstNonEmpty(health.AgentID, agentURL),
		Name:      "direct-agent",
		Status:    health.Status,
		Mode:      mode.Mode,
		Port:      parsePort(agentURL),
		Dir:       agentURL,
		CreatedAt: time.Now(),
	}
	term.PrintSpaceDetail(row)
	return nil
}

func parsePort(rawURL string) int {
	u, err := url.Parse(rawURL)
	if err != nil {
		return 0
	}
	port, _ := strconv.Atoi(u.Port())
	return port
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
