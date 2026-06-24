package model

// RegistryEntry is one row in GET /registry.
type RegistryEntry struct {
	URI         string `json:"uri"`
	Active      bool   `json:"active"`
	Description string `json:"description"`
}
