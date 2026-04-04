package plugincmd

import (
	"slices"

	"github.com/quarkloop/cli/pkg/plugin"
	"github.com/quarkloop/cli/pkg/quarkfile"
)

// QuarkAdd appends ref to the Quarkfile's plugins list if not already present.
func QuarkAdd(spaceDir, ref string) error {
	qf, err := quarkfile.Load(spaceDir)
	if err != nil {
		return err
	}
	for _, p := range qf.Plugins {
		if p.Ref == ref {
			return nil
		}
	}
	qf.Plugins = append(qf.Plugins, quarkfile.PluginRef{Ref: ref})
	return quarkfile.Save(spaceDir, qf)
}

// QuarkRemove removes entries matching the given plugin name from the Quarkfile.
func QuarkRemove(spaceDir, pluginName string) error {
	qf, err := quarkfile.Load(spaceDir)
	if err != nil {
		return err
	}
	filtered := slices.DeleteFunc(qf.Plugins, func(p quarkfile.PluginRef) bool {
		return plugin.DeriveName(p.Ref) == pluginName
	})
	if len(filtered) == len(qf.Plugins) {
		return nil
	}
	qf.Plugins = filtered
	return quarkfile.Save(spaceDir, qf)
}

func QuarkLoadAndList(spaceDir string) ([]string, error) {
	qf, err := quarkfile.Load(spaceDir)
	if err != nil {
		return nil, err
	}
	refs := make([]string, len(qf.Plugins))
	for i, p := range qf.Plugins {
		refs[i] = p.Ref
	}
	return refs, nil
}
