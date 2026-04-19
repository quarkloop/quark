package space

import (
	"fmt"
	"strings"

	"github.com/quarkloop/supervisor/pkg/api"
	"github.com/quarkloop/supervisor/pkg/pluginmanager"
	"github.com/quarkloop/supervisor/pkg/quarkfile"
)

// Doctor runs semantic checks on the Quarkfile and cross-references its
// plugin references against the installed plugin set. It never returns an
// error — problems are surfaced as DoctorIssue entries.
func Doctor(quarkfileBytes []byte, installed []pluginmanager.InstalledPlugin) api.DoctorResponse {
	out := api.DoctorResponse{OK: true}

	qf, err := parseQuarkfile(quarkfileBytes)
	if err != nil {
		return api.DoctorResponse{
			OK: false,
			Issues: []api.DoctorIssue{{
				Severity: "error",
				Message:  err.Error(),
			}},
		}
	}
	if err := quarkfile.Validate("", qf); err != nil {
		out.OK = false
		out.Issues = append(out.Issues, api.DoctorIssue{
			Severity: "error",
			Message:  fmt.Sprintf("quarkfile: %s", err.Error()),
		})
	}

	installedByName := make(map[string]bool, len(installed))
	for _, p := range installed {
		installedByName[p.Name] = true
	}
	for _, ref := range qf.Plugins {
		name := pluginNameFromRef(ref.Ref)
		if !installedByName[name] {
			out.OK = false
			out.Issues = append(out.Issues, api.DoctorIssue{
				Severity: "error",
				Message:  fmt.Sprintf("plugin %q (ref %q) referenced in Quarkfile but not installed", name, ref.Ref),
			})
		}
	}

	return out
}

// pluginNameFromRef returns the plugin name from a reference string such as
// "quark/tool-bash", "github.com/org/plugin", or a bare "bash".
func pluginNameFromRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if idx := strings.LastIndexByte(ref, '/'); idx >= 0 {
		return ref[idx+1:]
	}
	return ref
}
