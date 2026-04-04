package kbcmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func kbListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all keys in a KB namespace",
		Args:  cobra.ExactArgs(1),
		RunE:  runKBList,
	}
}

func runKBList(cmd *cobra.Command, args []string) error {
	c, err := resolveClient(cmd)
	if err != nil {
		return err
	}
	defer c.Close()
	keys, err := c.List(cmd.Context(), args[0])
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		fmt.Printf("No keys in namespace %s\n", args[0])
		return nil
	}
	for _, k := range keys {
		fmt.Println(k)
	}
	return nil
}
