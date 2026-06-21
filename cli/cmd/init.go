package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const quarkDtsContent = `/**
 * Quark Platform — TypeScript type definitions for .quark.ts files
 *
 * Place this file alongside your .quark.ts files for IDE autocomplete
 * and type checking. Run: quarkctl init
 */

export interface OnFailureConfig {
    retry: number;
    routeTo: string;
}

export interface NodeDefinition {
    uses: string;
    listens?: string[];
    events?: string[];
    onFailure?: OnFailureConfig;
    timeout?: string;
    [key: string]: unknown;
}

export interface SystemDefinition {
    name: string;
    namespace: string;
    nodes: Record<string, NodeDefinition>;
}
`

const systemQuarkTsTemplate = `/**
 * {{NAME}} — Quark System
 *
 * This file IS the program. Deploy with:
 *   quarkctl system deploy -f {{NAME}}.quark.ts -n alice
 */

export default {
    name: "{{NAME}}",
    namespace: "alice",

    nodes: {
        // Define your nodes here
        // timer: {
        //     uses: "source/timer:v1",
        //     interval: "1s",
        //     events: ["tick"],
        // },
        // cpu: {
        //     uses: "function/cpu-profiler:v1",
        //     listens: ["timer.tick"],
        //     events: ["data"],
        // },
    },
};
`

var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialize a new .quark.ts project with type definitions",
	Long: `Creates a quark.d.ts type definition file and a starter .quark.ts
template in the current directory. The .d.ts file provides IDE autocomplete
and type checking for .quark.ts files.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	name := "system"
	if len(args) > 0 {
		name = args[0]
	}

	// Write quark.d.ts
	dtsPath := "quark.d.ts"
	if _, err := os.Stat(dtsPath); err == nil {
		fmt.Printf("Warning: %s already exists, skipping\n", dtsPath)
	} else {
		if err := os.WriteFile(dtsPath, []byte(quarkDtsContent), 0644); err != nil {
			return fmt.Errorf("write %s: %w", dtsPath, err)
		}
		fmt.Printf("Created %s\n", dtsPath)
	}

	// Write starter .quark.ts
	tsPath := fmt.Sprintf("%s.quark.ts", name)
	if _, err := os.Stat(tsPath); err == nil {
		fmt.Printf("Warning: %s already exists, skipping\n", tsPath)
	} else {
		content := strings.ReplaceAll(systemQuarkTsTemplate, "{{NAME}}", name)
		if err := os.WriteFile(tsPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", tsPath, err)
		}
		fmt.Printf("Created %s\n", tsPath)
	}

	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  1. Edit %s to define your nodes\n", filepath.Base(tsPath))
	fmt.Printf("  2. Deploy: quarkctl system deploy -f %s -n alice\n", filepath.Base(tsPath))
	fmt.Printf("  3. Monitor: quarkctl system list -n alice\n")

	return nil
}
