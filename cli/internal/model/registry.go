package model

// RegistryEntry is one row in GET /registry.
type RegistryEntry struct {
	URI         string `json:"uri"`
	Description string `json:"description"`
}
