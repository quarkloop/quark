package repo

// AgentEntry is a simple agent record returned by ListAgents.
type AgentEntry struct {
	Name string
	Ref  string
}

// SkillEntry is a simple skill record returned by ListSkills.
type SkillEntry struct {
	Name string
	Ref  string
}

// KBEntry is a simple KB summary record returned by ListKBEntries.
type KBEntry struct {
	Path string
	Size int
}

// Init scaffolds a new space directory.
func Init(dir string) error { return runInit(dir) }

// Lock resolves all refs and writes the lock file.
func Lock(dir string) error { return runLock(dir) }

// Validate checks the Quarkfile and lock file for errors.
func Validate(dir string) error { return runValidate(dir) }

// AddAgent adds an agent ref to the Quarkfile.
func AddAgent(dir, ref, name string) error { return runAddAgent(dir, ref, name) }

// RemoveAgent removes an agent from the Quarkfile by name.
func RemoveAgent(dir, name string) error { return runRemoveAgent(dir, name) }

// ListAgents returns the agent list from the Quarkfile.
func ListAgents(dir string) ([]AgentEntry, error) { return runListAgents(dir) }

// AddSkill adds a skill ref to the Quarkfile.
func AddSkill(dir, ref, name string) error { return runAddSkill(dir, ref, name) }

// RemoveSkill removes a skill from the Quarkfile by name.
func RemoveSkill(dir, name string) error { return runRemoveSkill(dir, name) }

// ListSkills returns the skill list from the Quarkfile.
func ListSkills(dir string) ([]SkillEntry, error) { return runListSkills(dir) }

// AddKBEntry writes a file to the kb/ directory.
func AddKBEntry(dir, path string, value []byte) error { return runAddKB(dir, path, value) }

// RemoveKBEntry deletes a file from the kb/ directory.
func RemoveKBEntry(dir, path string) error { return runRemoveKB(dir, path) }

// ListKBEntries returns a list of kb/ entries.
func ListKBEntries(dir string) ([]KBEntry, error) { return runListKB(dir) }

// ShowKBEntry reads a KB entry file.
func ShowKBEntry(dir, path string) ([]byte, error) { return runShowKB(dir, path) }
