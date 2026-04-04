package kbcmd

import (
	"github.com/spf13/cobra"
)

func kbDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <namespace/key>",
		Short: "Delete a KB entry",
		Args:  cobra.ExactArgs(1),
		RunE:  runKBDelete,
	}
}

func runKBDelete(cmd *cobra.Command, args []string) error {
	ns, key, err := parseKey(args[0])
	if err != nil {
		return err
	}
	c, err := resolveClient(cmd)
	if err != nil {
		return err
	}
	defer c.Close()
	return c.Delete(cmd.Context(), ns, key)
}
