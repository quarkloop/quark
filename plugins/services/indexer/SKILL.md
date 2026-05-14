# service-indexer

The indexer service stores and retrieves structured GraphRAG data over gRPC.
The production driver is Dgraph, a Go graph database with native
`float32vector` predicates and HNSW vector indexes.

Use `quark.indexer.v1.IndexerService` only after the agent has already parsed
documents, extracted entities/relations, and produced embeddings. The indexer
does not call LLMs, read raw files, or perform OCR.

## Agent Workflows

When the user asks to index PDFs or other documents:

1. Use file tools to read the source content. For PDFs, use `fs` with
   `command=extract_pdf`.
2. Extract a useful, compact chunk for each document or section. Preserve the
   important facts needed for later Q&A.
3. Extract stable entities and relationships from the content. Entity IDs
   should be normalized from entity names and relation endpoints must reuse the
   same IDs as the entity list.
4. Use the configured embedding service plugin on each chunk and pass the
   returned `embeddingRef` as `embeddingRef`.
5. Call `indexer_IndexDocument`. Include source metadata such as source path
   and filename when available.

Indexing is not complete after extraction or embedding. Only tell the user a
document is indexed after `IndexDocument` returns a successful response from the
indexer service. When multiple documents are listed, keep the filenames aligned
with successful `IndexDocument` results and do not finish until every listed
document has one successful persistence result.

When the user asks questions about indexed documents:

1. Use the configured embedding service plugin on the user question.
2. Call `indexer_GetContext` with the query vector, a reasonable limit, and
   graph depth.
3. Answer from the returned `reasoning_context` and cite source metadata when
   available.

Do not invent vectors. Do not answer indexed-document questions from memory
when the indexer service is available.

## RPCs

- `IndexDocument(IndexRequest) -> IndexStatus`
  - Generated service function: `indexer_IndexDocument`
  - Required JSON fields: `chunkId`, `textContent`, `embeddingRef`
  - Optional JSON fields: `embedding`, `entities`, `relations`, `sourceMetadata`
  - Persists one text chunk, its embedding, entities, and graph edges.

- `GetContext(QueryRequest) -> ContextResponse`
  - Generated service function: `indexer_GetContext`
  - Required JSON fields: `queryVectorRef`
  - Optional JSON fields: `queryVector`, `limit`, `depth`, `filters`
  - Returns ranked chunks, a graph fragment, citations, and a flattened
    `reasoningContext` string suitable for an LLM context window.

## Contract Notes

- Query text must be embedded by the agent before `GetContext`.
- Use `embeddingRef` and `queryVectorRef` from `embedding_Embed` instead of
  manually copying vectors through the LLM.
- Use the same embedding dimensions for document chunks and query text.
- Metadata filters are exact key/value matches.
- Entity IDs are stable identifiers; when omitted, the service derives one from
  the entity name.
- The service is safe for concurrent calls and honors request cancellation.
