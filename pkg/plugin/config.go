package plugin

const (
	ConfigManifestPath = "manifest_path"
	ConfigPluginDir    = "plugin_dir"
)

func ManifestPathFromConfig(config map[string]any) string {
	if path, ok := config[ConfigManifestPath].(string); ok && path != "" {
		return path
	}
	return "manifest.yaml"
}
