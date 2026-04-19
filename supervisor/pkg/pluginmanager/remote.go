package pluginmanager

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
)

// Registry location
const (
	registryOwner = "quarkloop"
	registryRepo  = "plugins"
	registryRoot  = "plugins"
)

// ResolveRegistryURL returns the git clone URL for the plugin registry.
func ResolveRegistryURL() string {
	return fmt.Sprintf("https://github.com/%s/%s.git", registryOwner, registryRepo)
}

// GitClone performs a shallow git clone of url into dest.
func GitClone(url, dest string) error {
	_, err := git.PlainClone(dest, false, &git.CloneOptions{
		URL:          url,
		SingleBranch: true,
		Tags:         git.NoTags,
		Depth:        1,
	})
	return err
}

// CopyDir copies a directory tree from src to dst.
func CopyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := CopyDir(s, d); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(s)
			if err != nil {
				return err
			}
			if err := os.WriteFile(d, data, 0644); err != nil {
				return err
			}
		}
	}
	return nil
}

// FixFileModes sets dir permissions to 0755 and file permissions to 0644.
func FixFileModes(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return os.Chmod(path, 0755)
		}
		return os.Chmod(path, 0644)
	})
}

// IsLocalPath checks if ref starts with ./ or / — indicating a local directory.
func IsLocalPath(ref string) bool {
	return strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "/")
}

var (
	httpsRe = regexp.MustCompile(`^https?://`)
	sshRe   = regexp.MustCompile(`^git@`)
	sshPRe  = regexp.MustCompile(`^ssh://`)
	hostRe  = regexp.MustCompile(`^[a-zA-Z0-9.-]+\.[a-z]+/`)
)

// IsGitURL checks if ref looks like a git remote URL.
func IsGitURL(ref string) bool {
	return httpsRe.MatchString(ref) || sshRe.MatchString(ref) || sshPRe.MatchString(ref) || hostRe.MatchString(ref)
}

// DeriveName extracts a directory name from a URL or ref.
func DeriveName(url string) string {
	url = strings.TrimSuffix(url, ".git")
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}

// ParsedRef is a parsed plugin ref with optional version pin.
type ParsedRef struct {
	Name    string
	Version string
}

// ParsePluginRef splits "name@version" syntax.
// E.g. "tool-bash@v1.0.0" → {Name: "tool-bash", Version: "v1.0.0"}
// "tool-bash" → {Name: "tool-bash", Version: ""}
func ParsePluginRef(raw string) ParsedRef {
	if i := strings.LastIndex(raw, "@"); i > 0 {
		return ParsedRef{
			Name:    raw[:i],
			Version: raw[i+1:],
		}
	}
	return ParsedRef{Name: raw}
}

// cloneSingle clones a single git repo into a temp dir under pluginsDir.
// Returns (tmpDir, srcDir, error). Caller should defer os.RemoveAll(tmpDir).
func cloneSingle(url, pluginsDir string) (tmpDir, srcDir string, err error) {
	tmpDir, err = os.MkdirTemp(pluginsDir, ".temp-")
	if err != nil {
		return "", "", fmt.Errorf("create temp dir: %w", err)
	}

	_, err = git.PlainClone(tmpDir, false, &git.CloneOptions{
		URL:          url,
		SingleBranch: true,
		Tags:         git.NoTags,
		Depth:        1,
	})
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", "", fmt.Errorf("git clone: %w", err)
	}

	if err := FixFileModes(tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return "", "", fmt.Errorf("set file modes: %w", err)
	}

	return tmpDir, tmpDir, nil
}

// cloneFromRegistry clones the registry monorepo to a temp dir,
// locates the requested plugin in plugins/<name>/, and returns it.
func cloneFromRegistry(regRoot, name, pluginsDir string) (tmpDir, srcDir string, err error) {
	tmpDir, err = os.MkdirTemp(pluginsDir, ".temp-")
	if err != nil {
		return "", "", fmt.Errorf("create temp dir: %w", err)
	}

	_, err = git.PlainClone(tmpDir, false, &git.CloneOptions{
		URL:          ResolveRegistryURL(),
		SingleBranch: true,
		Tags:         git.NoTags,
		Depth:        1,
	})
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", "", fmt.Errorf("clone registry: %w", err)
	}

	if err := FixFileModes(tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return "", "", fmt.Errorf("set file modes: %w", err)
	}

	srcDir = filepath.Join(tmpDir, regRoot, name)
	if _, err := os.Stat(srcDir); err != nil {
		os.RemoveAll(tmpDir)
		return "", "", fmt.Errorf("plugin %q not found in registry", name)
	}

	return tmpDir, srcDir, nil
}
