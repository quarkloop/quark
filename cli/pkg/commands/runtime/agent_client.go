package runtimecmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	agentapi "github.com/quarkloop/agent-api"
	agentclient "github.com/quarkloop/agent-client"
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

func parsePort(rawURL string) int {
	u, err := url.Parse(rawURL)
	if err != nil {
		return 0
	}
	port, _ := strconv.Atoi(u.Port())
	return port
}
