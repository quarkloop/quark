package commands

import "testing"

func TestLoadPluginCatalogUsesEmptyCatalogWithoutEnv(t *testing.T) {
	t.Setenv("QUARK_SUPERVISOR_URL", "http://127.0.0.1:7200")
	t.Setenv("QUARK_SPACE", "test-space")
	t.Setenv("QUARK_RUNTIME_PLUGIN_CATALOG", "")

	catalog, err := loadPluginCatalog()
	if err != nil {
		t.Fatalf("load plugin catalog: %v", err)
	}
	if catalog == nil {
		t.Fatal("expected empty supervisor-owned catalog, got nil")
	}
	if !catalog.Empty() {
		t.Fatalf("expected empty catalog, got %+v", catalog)
	}
}
