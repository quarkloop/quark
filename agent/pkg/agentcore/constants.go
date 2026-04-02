package agentcore

// ─── KB namespace constants ──────────────────────────────────────────────────

const (
	// NSConfig is the KB namespace for configuration values.
	NSConfig = "config"
	// NSPlans is the KB namespace for execution plans.
	NSPlans = "plans"
	// NSEvents is the KB namespace for step-completion events.
	NSEvents = "events"
	// NSArtifacts is the KB namespace for step output artifacts.
	NSArtifacts = "artifacts"
	// NSMemory is the KB namespace for agent memory entries.
	NSMemory = "memory"
	// NSDocuments is the KB namespace for ingested documents.
	NSDocuments = "documents"
	// NSNotes is the KB namespace for freeform notes.
	NSNotes = "notes"
	// NSSnapshots is the KB namespace for context snapshots.
	NSSnapshots = "snapshots"
	// NSSessions is the KB namespace for session records.
	NSSessions = "sessions"
)

// ─── Well-known KB keys ──────────────────────────────────────────────────────

const (
	// KeyGoal is the KB key for the space's goal statement.
	KeyGoal = "goal"
	// KeyMasterPlan is the KB key for the master execution plan.
	KeyMasterPlan = "master"
	// KeySupervisorPrompt is the KB key for the supervisor system prompt.
	KeySupervisorPrompt = "supervisor-prompt"
	// KeyLatestSnapshot is the KB key for the most recent context snapshot.
	KeyLatestSnapshot = "supervisor-latest"
	// KeyMasterPlanDoc is the KB key for the master plan document.
	KeyMasterPlanDoc = "masterplan"
	// KeyMode is the KB key for the persisted agent working mode.
	KeyMode = "mode"
)

// ─── Author identifiers ─────────────────────────────────────────────────────

const (
	// AuthorSupervisor identifies the supervisor agent.
	AuthorSupervisor = "supervisor"
	// AuthorUser identifies user-originated messages.
	AuthorUser = "user"
	// AuthorAgent identifies generic agent-originated messages.
	AuthorAgent = "agent"
	// AuthorSubagent identifies subagent agent system prompts.
	AuthorSubagent = "subagent"
	// AuthorToolExecutor identifies tool execution results.
	AuthorToolExecutor = "tool-executor"
)

// ─── Numeric defaults ────────────────────────────────────────────────────────

const (
	// DefaultContextWindow is the fallback context window size in tokens.
	DefaultContextWindow = 8192
	// DefaultCompactionThreshold is the percentage at which compaction triggers.
	DefaultCompactionThreshold = 80
	// MaxToolIterations is the maximum number of tool-call rounds per subagent step.
	MaxToolIterations = 4
	// MaxAskToolIterations is the maximum number of tool-call rounds in ask mode.
	MaxAskToolIterations = 10
)

const (
	// DefaultSoftThresholdPct is the fraction of the token budget at which
	// compaction is triggered (80%).
	DefaultSoftThresholdPct = 0.80
	// DefaultHardThresholdPct is the fraction of the token budget at which
	// new messages are rejected (95%).
	DefaultHardThresholdPct = 0.95
)
