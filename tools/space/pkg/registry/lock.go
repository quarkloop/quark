// Package registry resolves agent and skill refs to definition files and
// computes their SHA-256 content digests for the lock file.
//
// Resolution order for each ref:
//  1. Local file registry at ~/.quark/registry/<kind>/<name>/<version>.yaml
//  2. Remote HTTPS registry at quarkloop.com (best-effort; falls back silently)
//
// Ref format: [host/]<name>@<version>  (e.g. "quark/supervisor@latest").
// If no version tag is given, "latest" is assumed.
package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/quarkloop/agent/pkg/agent"
	"github.com/quarkloop/agent/pkg/skill"
	"github.com/quarkloop/tools/space/pkg/quarkfile"
)

// Resolver abstracts agent and skill resolution so the Builder can be
// tested with a fake registry.
type Resolver interface {
	ResolveAgent(ref, expectedDigest string) (*agent.Definition, error)
	ResolveSkill(ref, expectedDigest string) (*skill.Definition, error)
}

// LocalClient is the production Resolver implementation.
// It tries the local file registry first, then the remote HTTPS registry.
type LocalClient struct {
	http      *http.Client
	localRoot string
}

// NewLocalClient returns a LocalClient using the default local registry
// directory (~/.quark/registry).
func NewLocalClient() *LocalClient {
	return &LocalClient{
		http:      &http.Client{Timeout: 30 * time.Second},
		localRoot: LocalRegistryDir(),
	}
}

// ─── Agent resolution ─────────────────────────────────────────────────────────

// ResolveAgent fetches an agent definition and verifies its digest.
// If expectedDigest is non-empty and doesn't match, an error is returned.
func (c *LocalClient) ResolveAgent(ref, expectedDigest string) (*agent.Definition, error) {
	data, err := c.fetchRaw("agents", ref)
	if err != nil {
		return nil, fmt.Errorf("resolving agent %s: %w", ref, err)
	}

	if expectedDigest != "" && !strings.HasPrefix(expectedDigest, "sha256:TODO") {
		got := digestOf(data)
		if got != expectedDigest {
			return nil, fmt.Errorf("agent %s digest mismatch: lock=%s got=%s", ref, expectedDigest, got)
		}
	}

	var def agent.Definition
	if err := yaml.Unmarshal(data, &def); err != nil {
		// try JSON fallback
		if err2 := json.Unmarshal(data, &def); err2 != nil {
			return nil, fmt.Errorf("decoding agent %s: %w", ref, err)
		}
	}
	def.Ref = ref
	def.Digest = digestOf(data)
	return &def, nil
}

// ─── Skill resolution ─────────────────────────────────────────────────────────

// ResolveSkill fetches a skill definition and verifies its digest.
func (c *LocalClient) ResolveSkill(ref, expectedDigest string) (*skill.Definition, error) {
	data, err := c.fetchRaw("skills", ref)
	if err != nil {
		return nil, fmt.Errorf("resolving skill %s: %w", ref, err)
	}

	if expectedDigest != "" && !strings.HasPrefix(expectedDigest, "sha256:TODO") {
		got := digestOf(data)
		if got != expectedDigest {
			return nil, fmt.Errorf("skill %s digest mismatch: lock=%s got=%s", ref, expectedDigest, got)
		}
	}

	var def skill.Definition
	if err := yaml.Unmarshal(data, &def); err != nil {
		if err2 := json.Unmarshal(data, &def); err2 != nil {
			return nil, fmt.Errorf("decoding skill %s: %w", ref, err)
		}
	}
	def.Ref = ref
	def.Digest = digestOf(data)
	return &def, nil
}

// ─── Lock helpers ─────────────────────────────────────────────────────────────

// LockAgent resolves ref and returns a LockedAgent entry suitable for
// writing into the lock file. The digest field is a real sha256:<hex> value.
func LockAgent(ref string) (*quarkfile.LockedAgent, error) {
	c := NewLocalClient()
	data, err := c.fetchRaw("agents", ref)
	if err != nil {
		return nil, err
	}
	resolved := resolvedRef(ref, data)
	return &quarkfile.LockedAgent{
		Ref:      ref,
		Resolved: resolved,
		Digest:   digestOf(data),
	}, nil
}

// LockSkill resolves ref and returns a LockedSkill entry with a real digest.
func LockSkill(ref string) (*quarkfile.LockedSkill, error) {
	c := NewLocalClient()
	data, err := c.fetchRaw("skills", ref)
	if err != nil {
		return nil, err
	}
	resolved := resolvedRef(ref, data)
	return &quarkfile.LockedSkill{
		Ref:      ref,
		Resolved: resolved,
		Digest:   digestOf(data),
	}, nil
}

// ─── Internal fetch logic ─────────────────────────────────────────────────────

// fetchRaw loads the raw bytes for a ref, trying local registry first.
//
// Ref format: [host/]<name>@<version>  or  [host/]<name>  (→ @latest)
// Local path: <localRoot>/<kind>/<name>/<version>.yaml
func (c *LocalClient) fetchRaw(kind, ref string) ([]byte, error) {
	name, version := parseRef(ref)

	// 1. Try local registry
	if data, err := c.localFetch(kind, name, version); err == nil {
		return data, nil
	}

	// 2. Remote registry (quarkloop.com) — best-effort, not yet live
	//    Falls back gracefully so local development always works.
	base := "https://quarkloop.com/" + kind
	url := fmt.Sprintf("%s/%s/%s.yaml", base, name, version)
	resp, err := c.http.Get(url)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		var buf []byte
		buf = make([]byte, 0, 4096)
		tmp := make([]byte, 4096)
		for {
			n, rerr := resp.Body.Read(tmp)
			buf = append(buf, tmp[:n]...)
			if rerr != nil {
				break
			}
		}
		if len(buf) > 0 {
			return buf, nil
		}
	}

	return nil, fmt.Errorf("%s %q not found in local registry (~/.quark/registry/%s/%s/%s.yaml) and remote is unreachable",
		kind, ref, kind, name, version)
}

// localFetch reads a definition file from the local registry directory.
func (c *LocalClient) localFetch(kind, name, version string) ([]byte, error) {
	// Strip host prefix if present (e.g. "quarkloop.com/agents/foo" → "foo")
	name = stripHost(name)

	candidates := []string{
		filepath.Join(c.localRoot, kind, name, version+".yaml"),
		filepath.Join(c.localRoot, kind, name, version+".json"),
		// Also check without version subdirectory for simple layouts
		filepath.Join(c.localRoot, kind, name+".yaml"),
		filepath.Join(c.localRoot, kind, name+".json"),
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("not found locally")
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// digestOf returns "sha256:<hex>" for the given bytes.
func digestOf(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}

// parseRef splits "name@version" or "name" into (name, version).
// "latest" is used when no version is specified.
func parseRef(ref string) (name, version string) {
	if idx := strings.LastIndex(ref, "@"); idx >= 0 {
		return ref[:idx], ref[idx+1:]
	}
	return ref, "latest"
}

// resolvedRef produces a fully pinned ref string "name@version".
// It tries to read a version field from the definition YAML.
func resolvedRef(ref string, data []byte) string {
	var raw struct {
		Version string `yaml:"version" json:"version"`
	}
	_ = yaml.Unmarshal(data, &raw)
	_ = json.Unmarshal(data, &raw)
	name, version := parseRef(ref)
	if raw.Version != "" {
		version = raw.Version
	}
	return name + "@" + version
}

// stripHost removes a hostname prefix if the name looks like a URL path.
// "quarkloop.com/agents/supervisor" → "supervisor"
func stripHost(name string) string {
	parts := strings.Split(name, "/")
	// If first part looks like a hostname (contains a dot), drop it
	if len(parts) > 1 && strings.Contains(parts[0], ".") {
		parts = parts[1:]
	}
	return strings.Join(parts, "/")
}
