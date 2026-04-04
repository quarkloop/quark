// Package plancmd provides CLI commands for managing execution plans.
package plancmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/quarkloop/cli/pkg/middleware"
	"github.com/quarkloop/cli/pkg/plan"
	"github.com/quarkloop/cli/pkg/resolve"
)

func NewPlanCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Manage execution plans",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return middleware.RequireSpace()
		},
	}
	cmd.AddCommand(newPlanCreateCmd())
	cmd.AddCommand(newPlanGetCmd())
	cmd.AddCommand(newPlanListCmd())
	cmd.AddCommand(newPlanApproveCmd())
	cmd.AddCommand(newPlanRejectCmd())
	cmd.AddCommand(newPlanUpdateCmd())
	return cmd
}

func resolveClient(cmd *cobra.Command) (*plan.Client, error) {
	if url := resolve.AgentURL(cmd); url != "" {
		return plan.NewHTTP(url), nil
	}
	dir, err := resolve.SpaceDir()
	if err != nil {
		return nil, err
	}
	return plan.NewLocal(dir)
}

func newPlanCreateCmd() *cobra.Command {
	var goal string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new draft plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			defer c.Close()
			p, err := c.Create(cmd.Context(), goal)
			if err != nil {
				return fmt.Errorf("create plan: %w", err)
			}
			fmt.Printf("Plan created: %s\n", p.Goal)
			return nil
		},
	}
	cmd.Flags().StringVar(&goal, "goal", "", "Plan goal description")
	return cmd
}

func newPlanGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <plan-id>",
		Short: "Get a plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			defer c.Close()
			p, err := c.Get(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("get plan: %w", err)
			}
			data, _ := json.MarshalIndent(p, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}
}

func newPlanListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all plans",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			defer c.Close()
			plans, err := c.List(cmd.Context())
			if err != nil {
				return fmt.Errorf("list plans: %w", err)
			}
			if len(plans) == 0 {
				fmt.Println("No plans.")
				return nil
			}
			for _, p := range plans {
				fmt.Printf("%-10s  %s\n", p.Status, p.Goal)
			}
			return nil
		},
	}
}

func newPlanApproveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "approve [plan-id]",
		Short: "Approve the current active plan",
		Long: `Approve the current active plan. In local mode, an optional plan-id
may be provided to approve a specific stored plan. In HTTP mode (connected to a
running agent), only the single active plan is approved and any plan-id argument
is ignored.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			defer c.Close()
			planID := ""
			if len(args) > 0 {
				planID = args[0]
			}
			if _, err := c.Approve(cmd.Context(), planID); err != nil {
				return fmt.Errorf("approve plan: %w", err)
			}
			fmt.Println("Plan approved")
			return nil
		},
	}
}

func newPlanRejectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reject [plan-id]",
		Short: "Reject the current active plan",
		Long: `Reject the current active plan. In local mode, an optional plan-id
may be provided to reject a specific stored plan. In HTTP mode, plan-id is ignored.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			defer c.Close()
			planID := ""
			if len(args) > 0 {
				planID = args[0]
			}
			if err := c.Reject(cmd.Context(), planID); err != nil {
				return fmt.Errorf("reject plan: %w", err)
			}
			fmt.Println("Plan rejected")
			return nil
		},
	}
}

func newPlanUpdateCmd() *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use:   "update <plan-id>",
		Short: "Update a plan's status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if status == "" {
				return fmt.Errorf("--status is required")
			}
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			defer c.Close()
			if err := c.Update(cmd.Context(), args[0], status); err != nil {
				return fmt.Errorf("update plan: %w", err)
			}
			fmt.Printf("Plan %s status updated to %s\n", args[0], status)
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "New status (draft|approved|rejected|executing|completed|failed)")
	return cmd
}
