// kb is the quark knowledge base CLI tool.
//
// It provides read/write/list access to a space's JSONL knowledge base.
// Each space directory has its own KB under kb/kb.jsonl.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/quarkloop/kb/pkg/kb"
)

func main() {
	var dir string

	root := &cobra.Command{
		Use:          "kb",
		Short:        "quark kb — read and write the space knowledge base",
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVar(&dir, "dir", ".", "Space directory containing the KB")

	root.AddCommand(getCmd(&dir))
	root.AddCommand(setCmd(&dir))
	root.AddCommand(deleteCmd(&dir))
	root.AddCommand(listCmd(&dir))

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}


func getCmd(dir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "get <namespace/key>",
		Short: "Read a KB entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ns, key, err := splitNsKey(args[0])
			if err != nil {
				return err
			}
			store, err := kb.Open(*dir)
			if err != nil {
				return err
			}
			defer store.Close()
			val, err := store.Get(ns, key)
			if err != nil {
				return err
			}
			fmt.Print(string(val))
			return nil
		},
	}
}

func setCmd(dir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "set <namespace/key> <value>",
		Short: "Write a KB entry",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ns, key, err := splitNsKey(args[0])
			if err != nil {
				return err
			}
			store, err := kb.Open(*dir)
			if err != nil {
				return err
			}
			defer store.Close()
			return store.Set(ns, key, []byte(args[1]))
		},
	}
}

func deleteCmd(dir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <namespace/key>",
		Short: "Delete a KB entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ns, key, err := splitNsKey(args[0])
			if err != nil {
				return err
			}
			store, err := kb.Open(*dir)
			if err != nil {
				return err
			}
			defer store.Close()
			return store.Delete(ns, key)
		},
	}
}

func listCmd(dir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list <namespace>",
		Short: "List all keys in a KB namespace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := kb.Open(*dir)
			if err != nil {
				return err
			}
			defer store.Close()
			keys, err := store.List(args[0])
			if err != nil {
				return err
			}
			for _, k := range keys {
				fmt.Println(k)
			}
			return nil
		},
	}
}

func splitNsKey(s string) (string, string, error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid namespace/key %q — must be <namespace>/<key>", s)
	}
	return parts[0], parts[1], nil
}
