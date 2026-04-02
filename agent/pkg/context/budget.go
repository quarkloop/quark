package llmctx

// defaultSoftThresholdPct is the fraction of the budget at which compaction triggers.
const defaultSoftThresholdPct = 0.80

// defaultHardThresholdPct is the fraction of the budget at which new messages are rejected.
const defaultHardThresholdPct = 0.95

// BudgetConfig specifies token budget thresholds for a session.
// If TotalTokens is 0, the context window size is used.
type BudgetConfig struct {
	TotalTokens      int     // 0 = use context window size
	SoftThresholdPct float64 // triggers compaction (default 0.80)
	HardThresholdPct float64 // rejects new messages (default 0.95)
}

// DefaultBudget returns a BudgetConfig with default thresholds derived from
// the given window size. TotalTokens is set to windowSize; if windowSize is 0
// the budget is effectively unbounded.
func DefaultBudget(windowSize int) *BudgetConfig {
	return &BudgetConfig{
		TotalTokens:      windowSize,
		SoftThresholdPct: defaultSoftThresholdPct,
		HardThresholdPct: defaultHardThresholdPct,
	}
}

// BudgetStatus is a snapshot of the current budget utilization.
type BudgetStatus struct {
	TotalBudget      int     `json:"total_budget"`
	UsedTokens       int     `json:"used_tokens"`
	AvailableTokens  int     `json:"available_tokens"`
	UsagePct         float64 `json:"usage_pct"`
	SoftThreshold    int     `json:"soft_threshold"`
	HardThreshold    int     `json:"hard_threshold"`
	CompactionNeeded bool    `json:"compaction_needed"`
	AtHardLimit      bool    `json:"at_hard_limit"`
}
