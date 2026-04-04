// Package plan provides CLI commands for managing execution plans.
// Plans are stored in .quark/plans/ as JSONL files.
package plan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/quarkloop/cli/pkg/middleware"
	"github.com/quarkloop/cli/pkg/quarkfile"
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

func spaceDir(cmd *cobra.Command) string {
	dir, _ := cmd.Flags().GetString("dir")
	if dir == "" {
		return "."
	}
	return dir
}

func plansDir(spaceDir string) (string, error) {
	abs, err := filepath.Abs(spaceDir)
	if err != nil {
		return "", err
	}
	if !quarkfile.Exists(abs) {
		return "", fmt.Errorf("no Quarkfile found in %s", abs)
	}
	return filepath.Join(abs, ".quark", "plans"), nil
}

type planRecord struct {
	ID        string    `json:"id"`
	Goal      string    `json:"goal"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func newPlanCreateCmd() *cobra.Command {
	var goal string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new draft plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			pDir, err := plansDir(spaceDir(cmd))
			if err != nil {
				return err
			}
			if err := os.MkdirAll(pDir, 0755); err != nil {
				return fmt.Errorf("create plans dir: %w", err)
			}
			id := fmt.Sprintf("plan-%d", time.Now().UnixNano())
			record := planRecord{
				ID:        id,
				Goal:      goal,
				Status:    "draft",
				CreatedAt: time.Now(),
			}
			data, err := json.Marshal(record)
			if err != nil {
				return err
			}
			path := filepath.Join(pDir, id+".jsonl")
			if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
				return fmt.Errorf("write plan: %w", err)
			}
			fmt.Printf("Plan created: %s\n", id)
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
			pDir, err := plansDir(spaceDir(cmd))
			if err != nil {
				return err
			}
			path := filepath.Join(pDir, args[0]+".jsonl")
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("plan not found: %s", args[0])
			}
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
			pDir, err := plansDir(spaceDir(cmd))
			if err != nil {
				return err
			}
			entries, err := os.ReadDir(pDir)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Println("No plans.")
					return nil
				}
				return fmt.Errorf("list plans: %w", err)
			}
			if len(entries) == 0 {
				fmt.Println("No plans.")
				return nil
			}
			for _, e := range entries {
				data, err := os.ReadFile(filepath.Join(pDir, e.Name()))
				if err != nil {
					continue
				}
				var r planRecord
				if err := json.Unmarshal(data, &r); err != nil {
					continue
				}
				fmt.Printf("%s  %-10s  %s\n", r.ID, r.Status, r.Goal)
			}
			return nil
		},
	}
}

func newPlanApproveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "approve <plan-id>",
		Short: "Approve a draft plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return updatePlanStatus(spaceDir(cmd), args[0], "approved")
		},
	}
}

func newPlanRejectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reject <plan-id>",
		Short: "Reject a draft plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return updatePlanStatus(spaceDir(cmd), args[0], "rejected")
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
			return updatePlanStatus(spaceDir(cmd), args[0], status)
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "New status (draft|approved|rejected|executing|completed|failed)")
	return cmd
}

func updatePlanStatus(spaceDir, planID, status string) error {
	pDir, err := plansDir(spaceDir)
	if err != nil {
		return err
	}
	path := filepath.Join(pDir, planID+".jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("plan not found: %s", planID)
	}

	var record planRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return fmt.Errorf("parse plan: %w", err)
	}
	record.Status = status

	data, err = json.Marshal(record)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("update plan: %w", err)
	}
	fmt.Printf("Plan %s status updated to %s\n", planID, status)
	return nil
}
