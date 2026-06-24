package store

import (
        "log"
        "os"
        "path/filepath"
        "strings"

        "github.com/quarkloop/quark/quark-catalog/internal/api"
)

// --- Legacy JSONL migration ---
//
// On first startup, if $STATE_ROOT/systems/ exists (the pre-Catalog
// on-disk layout), the Catalog migrates every systems/<ns>/<sys>/source.ts
// into the systems table and renames the directory to systems.backup/.
// This is a one-shot migration — it never runs once the backup exists.

// MigrateLegacy walks $STATE_ROOT/systems/ and ingests each source.ts
// into the systems table. If the directory does not exist, returns a
// zero MigrationResult without error.
func (s *Store) MigrateLegacy(stateRoot string) (*MigrationResult, error) {
        legacyDir := filepath.Join(stateRoot, "systems")
        if _, err := os.Stat(legacyDir); os.IsNotExist(err) {
                return &MigrationResult{}, nil
        }
        result := &MigrationResult{}
        err := filepath.Walk(legacyDir, func(path string, info os.FileInfo, err error) error {
                if err != nil || info.IsDir() {
                        return err
                }
                if info.Name() != "source.ts" {
                        return nil
                }
                // Path shape: systems/<ns>/<sys>/source.ts
                rel, _ := filepath.Rel(legacyDir, path)
                parts := strings.Split(filepath.Dir(rel), string(filepath.Separator))
                if len(parts) < 2 {
                        return nil
                }
                ns, sysName := parts[0], parts[1]
                data, err := os.ReadFile(path)
                if err != nil {
                        return err
                }
                if err := s.SaveSystem(api.SaveSystemRequest{
                        Namespace: ns, Name: sysName, Source: string(data),
                        State: "ACTIVE", Health: "HEALTHY", Version: 1,
                }); err != nil {
                        log.Printf("[WARN] migration: failed to save %s/%s: %v", ns, sysName, err)
                        return nil // keep going
                }
                result.Systems++
                return nil
        })
        if err != nil {
                return result, err
        }
        // Rename systems/ to systems.backup/ so this never runs again.
        backupDir := filepath.Join(stateRoot, "systems.backup")
        if err := os.Rename(legacyDir, backupDir); err != nil {
                log.Printf("[WARN] migration: could not rename %s to %s: %v", legacyDir, backupDir, err)
        } else if result.Systems > 0 {
                log.Printf("[INFO] Migrated %d systems, renamed %s to %s",
                        result.Systems, legacyDir, backupDir)
        }
        return result, nil
}
