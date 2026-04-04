package kbcmd

import (
	"github.com/spf13/cobra"
)

func kbGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <namespace/key>",
		Short: "Read a KB entry",
		Args:  cobra.ExactArgs(1),
		RunE:  runKBGet,
	}
}

func runKBGet(cmd *cobra.Command, args []string) error {
	ns, key, err := parseKey(args[0])
	if err != nil {
		return err
	}
	c, err := resolveClient(cmd)
	if err != nil {
		return err
	}
	defer c.Close()
	val, err := c.Get(cmd.Context(), ns, key)
	if err != nil {
		return err
	}
	cmd.Print(string(val))
	return nil
}
