package kbcmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func kbSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <namespace/key> <value|@file>",
		Short: "Write a KB entry (use @<path> to read from file)",
		Args:  cobra.ExactArgs(2),
		RunE:  runKBSet,
	}
}

func runKBSet(cmd *cobra.Command, args []string) error {
	ns, key, err := parseKey(args[0])
	if err != nil {
		return err
	}
	var value []byte
	if strings.HasPrefix(args[1], "@") {
		value, err = os.ReadFile(args[1][1:])
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}
	} else {
		value = []byte(args[1])
	}
	c, err := resolveClient(cmd)
	if err != nil {
		return err
	}
	defer c.Close()
	return c.Set(cmd.Context(), ns, key, value)
}
