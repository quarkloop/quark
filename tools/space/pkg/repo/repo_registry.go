package repo

import (
	"fmt"
	"os"
	"path/filepath"

	registryfeature "github.com/quarkloop/tools/space/pkg/registry"
)

func ScaffoldRegistry() error {
	root := registryfeature.LocalRegistryDir()

	files := map[string]string{
		"agents/quark/supervisor/latest.yaml": supervisorAgentYAML,
		"agents/quark/researcher/latest.yaml": researcherAgentYAML,
		"agents/quark/writer/latest.yaml":     writerAgentYAML,
		"skills/quark/bash/latest.yaml":       bashSkillYAML,
		"skills/quark/read/latest.yaml":       readSkillYAML,
		"skills/quark/write/latest.yaml":      writeSkillYAML,
		"skills/quark/web-search/latest.yaml": webSearchSkillYAML,
	}

	for relPath, content := range files {
		full := filepath.Join(root, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			return fmt.Errorf("creating dir for %s: %w", relPath, err)
		}
		if _, err := os.Stat(full); err == nil {
			continue // skip existing
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", relPath, err)
		}
	}
	return nil
}

// --- scaffold templates ---

const supervisorAgentYAML = `# Supervisor agent — orchestrates worker agents to achieve a goal.
ref: quark/supervisor
name: supervisor
version: "1.0.0"

config:
  context_window: 200000
  compaction: sliding
  memory_policy: summarize

capabilities:
  spawn_agents: true
  max_workers: 10
  create_plans: true

required_skills: []

system_prompt: |
  You are the supervisor agent for this Quark space. Your role is to:
  1. Understand the goal provided in the knowledge base under config/goal
  2. Break it into concrete steps assignable to worker agents
  3. Monitor progress and adjust the plan as needed
  4. Declare completion when the goal is fully achieved

  Always respond with valid JSON when asked to produce or update a plan.
  Be specific and actionable in step descriptions.
`

const researcherAgentYAML = `# Researcher agent — gathers information and synthesizes findings.
ref: quark/researcher
name: researcher
version: "1.0.0"

config:
  context_window: 100000
  compaction: sliding
  memory_policy: accumulate

capabilities:
  spawn_agents: false
  max_workers: 0
  create_plans: false

required_skills:
  - web-search

system_prompt: |
  You are a research agent. When given a task:
  1. Identify the key questions to answer
  2. Use the web-search tool to gather current information
  3. Synthesize findings into a clear, structured report
  4. Cite your sources

  Return a comprehensive yet concise research report.
`

const writerAgentYAML = `# Writer agent — produces well-structured written content.
ref: quark/writer
name: writer
version: "1.0.0"

config:
  context_window: 100000
  compaction: sliding
  memory_policy: full

capabilities:
  spawn_agents: false
  max_workers: 0
  create_plans: false

required_skills: []

system_prompt: |
  You are a professional writer agent. When given a task:
  1. Review any research or context provided
  2. Structure the content with clear sections
  3. Write in a clear, engaging, and appropriate tone
  4. Proofread and refine before returning

  Return the final written content only, without meta-commentary.
`

const webSearchSkillYAML = `# Web search skill stub — replace endpoint with a real search API wrapper.
ref: quark/web-search
name: web-search
version: "1.0.0"
digest: ""

endpoint: http://127.0.0.1:8090/search

input_schema:
  type: object
  properties:
    query:
      type: string
      description: The search query
  required: [query]

output_schema:
  type: object
  properties:
    results:
      type: array
      items:
        type: object
        properties:
          title: {type: string}
          url: {type: string}
          snippet: {type: string}

config: {}
`

const bashSkillYAML = `# Bash tool — executes a single shell command per request.
ref: quark/bash
name: bash
version: "1.0.0"
digest: ""

endpoint: http://127.0.0.1:8091/run

input_schema:
  type: object
  properties:
    cmd:
      type: string
      description: Shell command to execute
  required: [cmd]

output_schema:
  type: object
  properties:
    output:
      type: string
    exit_code:
      type: integer

config: {}
`

const readSkillYAML = `# Read tool — reads regular text files.
ref: quark/read
name: read
version: "1.0.0"
digest: ""

endpoint: http://127.0.0.1:8093/read

input_schema:
  type: object
  properties:
    path:
      type: string
    start_line:
      type: integer
    end_line:
      type: integer
  required: [path]

output_schema:
  type: object
  properties:
    path:
      type: string
    content:
      type: string
    bytes_read:
      type: integer
    total_lines:
      type: integer

config: {}
`

const writeSkillYAML = `# Write tool — writes and edits regular text files.
ref: quark/write
name: write
version: "1.0.0"
digest: ""

endpoint: http://127.0.0.1:8092/apply

input_schema:
  type: object
  properties:
    path:
      type: string
    operation:
      type: string
    content:
      type: string
    edits:
      type: array
  required: [path]

output_schema:
  type: object
  properties:
    path:
      type: string
    operation:
      type: string
    bytes_written:
      type: integer
    edits_applied:
      type: integer
    content_preview:
      type: string

config: {}
`
