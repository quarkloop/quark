package model

// RegistryEntry is one row in GET /registry.
type RegistryEntry struct {
	URI         string `json:"uri"`
	Category    string `json:"category"`
	Active      bool   `json:"active"`
	Description string `json:"description"`
}
