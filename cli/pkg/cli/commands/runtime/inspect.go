package runtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/quarkloop/agent/pkg/infra/term"
	"github.com/quarkloop/api-server/pkg/api"
)

// InspectCLI returns the "inspect" command.
func InspectCLI() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <id>",
		Short: "Show full details of a space",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := api.NewClientApi(apiServerURL()).GetSpace(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			mode := ""
			if s.Port > 0 && s.Status == "running" {
				mode = fetchMode(s.Port)
			}
			term.PrintSpaceDetail(term.SpaceRow{
				ID: s.ID, Name: s.Name, Status: string(s.Status),
				Mode: mode, Port: s.Port, Dir: s.Dir, PID: s.PID,
				CreatedAt: s.CreatedAt,
			})
			return nil
		},
	}
}

// fetchMode queries the runtime's /mode endpoint to get the current agent mode.
func fetchMode(port int) string {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/mode", port))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var result struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}
	return result.Mode
}
