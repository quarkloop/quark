package runtime

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/quarkloop/api-server/pkg/api"
)

func systemStatusCLI() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show api-server status",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := api.NewClientApi(apiServerURL()).SystemInfo(cmd.Context())
			if err != nil {
				return fmt.Errorf("api-server unreachable at %s: %w", apiServerURL(), err)
			}
			data, _ := json.MarshalIndent(info, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}
}
