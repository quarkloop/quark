package dgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dgraph-io/dgo/v250/protos/api"
	"github.com/quarkloop/services/indexer/pkg/indexer"
)

func (d *Driver) InsertChunk(ctx context.Context, chunk indexer.Chunk) error {
	if err := d.ensureMetadataPredicates(ctx, chunk.Metadata); err != nil {
		return err
	}
	metaJSON, err := json.Marshal(chunk.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	nquads := []byte(chunkMutationNQuads(chunk, string(metaJSON)))
	return d.doMutation(ctx, fmt.Sprintf("insert chunk %s", chunk.ID), func() *api.Request {
		return &api.Request{
			Query: `query chunk($id: string) { c as var(func: eq(quark.chunk_id, $id)) }`,
			Vars:  map[string]string{"$id": chunk.ID},
			Mutations: []*api.Mutation{{
				SetNquads: nquads,
			}},
			CommitNow: true,
		}
	})
}

func (d *Driver) UpsertEntity(ctx context.Context, entity indexer.Entity) error {
	normalized := normalizeEntity(entity)
	nquads := []byte(entityMutationNQuads(normalized))
	return d.doMutation(ctx, fmt.Sprintf("upsert entity %s", normalized.ID), func() *api.Request {
		return &api.Request{
			Query: `query entity($id: string) { e as var(func: eq(quark.entity_id, $id)) }`,
			Vars:  map[string]string{"$id": normalized.ID},
			Mutations: []*api.Mutation{{
				SetNquads: nquads,
			}},
			CommitNow: true,
		}
	})
}

func (d *Driver) LinkChunkEntity(ctx context.Context, chunkID, entityID string) error {
	if chunkID == "" || entityID == "" {
		return nil
	}
	return d.doMutation(ctx, fmt.Sprintf("link chunk %s to entity %s", chunkID, entityID), func() *api.Request {
		return &api.Request{
			Query: `query link($chunk: string, $entity: string) {
  c as var(func: eq(quark.chunk_id, $chunk))
  e as var(func: eq(quark.entity_id, $entity))
}`,
			Vars: map[string]string{"$chunk": chunkID, "$entity": entityID},
			Mutations: []*api.Mutation{{
				SetNquads: []byte("uid(c) <quark.chunk_entity> uid(e) .\n"),
			}},
			CommitNow: true,
		}
	})
}

func (d *Driver) RelateNodes(ctx context.Context, relation indexer.Relation) error {
	if relation.FromID == "" || relation.ToID == "" || relation.Relation == "" {
		return nil
	}
	relationID := relation.FromID + "|" + relation.Relation + "|" + relation.ToID
	nquads := []byte(relationMutationNQuads(relationID, relation.Relation))
	return d.doMutation(ctx, fmt.Sprintf("relate %s -> %s", relation.FromID, relation.ToID), func() *api.Request {
		return &api.Request{
			Query: `query relation($id: string, $from: string, $to: string) {
  r as var(func: eq(quark.relation_id, $id))
  f as var(func: eq(quark.entity_id, $from))
  t as var(func: eq(quark.entity_id, $to))
}`,
			Vars: map[string]string{"$id": relationID, "$from": relation.FromID, "$to": relation.ToID},
			Mutations: []*api.Mutation{{
				SetNquads: nquads,
			}},
			CommitNow: true,
		}
	})
}

func normalizeEntity(entity indexer.Entity) indexer.Entity {
	if entity.ID == "" {
		entity.ID = indexer.EntityIDFromName(entity.Name)
	}
	if entity.Name == "" {
		entity.Name = entity.ID
	}
	if entity.Type == "" {
		entity.Type = "UNKNOWN"
	}
	return entity
}

func chunkMutationNQuads(chunk indexer.Chunk, metaJSON string) string {
	var nquads strings.Builder
	fmt.Fprintf(&nquads, `uid(c) <dgraph.type> "QuarkChunk" .`+"\n")
	fmt.Fprintf(&nquads, "uid(c) <quark.chunk_id> %s .\n", quote(chunk.ID))
	fmt.Fprintf(&nquads, "uid(c) <quark.text_content> %s .\n", quote(chunk.Text))
	fmt.Fprintf(&nquads, "uid(c) <quark.embedding> %s .\n", quote(vectorLiteral(chunk.Vector)))
	fmt.Fprintf(&nquads, "uid(c) <quark.metadata_json> %s .\n", quote(metaJSON))
	for key, value := range chunk.Metadata {
		fmt.Fprintf(&nquads, "uid(c) <%s> %s .\n", metadataPredicate(key), quote(value))
	}
	return nquads.String()
}

func entityMutationNQuads(entity indexer.Entity) string {
	return fmt.Sprintf(`uid(e) <dgraph.type> "QuarkEntity" .
uid(e) <quark.entity_id> %s .
uid(e) <quark.entity_name> %s .
uid(e) <quark.entity_type> %s .
`, quote(entity.ID), quote(entity.Name), quote(entity.Type))
}

func relationMutationNQuads(relationID, name string) string {
	return fmt.Sprintf(`uid(r) <dgraph.type> "QuarkRelation" .
uid(r) <quark.relation_id> %s .
uid(r) <quark.relation_name> %s .
uid(r) <quark.relation_from> uid(f) .
uid(r) <quark.relation_to> uid(t) .
`, quote(relationID), quote(name))
}
