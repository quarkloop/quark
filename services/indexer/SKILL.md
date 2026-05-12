# service-indexer

The indexer service stores and retrieves structured GraphRAG data over gRPC.
The production driver is Dgraph, a Go graph database with native
`float32vector` predicates and HNSW vector indexes.

Use `quark.indexer.v1.IndexerService` only after the agent has already parsed
documents, extracted entities/relations, and produced embeddings. The indexer
does not call LLMs, read raw files, or perform OCR.

## RPCs

- `IndexDocument(IndexRequest) -> IndexStatus`
  - Required: `chunk_id`, `text_content`
  - Optional: `embedding`, `entities`, `relations`, `source_metadata`
  - Persists one text chunk, its embedding, entities, and graph edges.

- `GetContext(QueryRequest) -> ContextResponse`
  - Required: `query_vector`
  - Optional: `limit`, `depth`, `filters`
  - Returns ranked chunks, a graph fragment, citations, and a flattened context
    string suitable for an LLM context window.

## Contract Notes

- Query text must be embedded by the agent before `GetContext`.
- Metadata filters are exact key/value matches.
- Entity IDs are stable identifiers; when omitted, the service derives one from
  the entity name.
- The service is safe for concurrent calls and honors request cancellation.
