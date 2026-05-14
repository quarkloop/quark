//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/quarkloop/e2e/utils"
	embeddingv1 "github.com/quarkloop/pkg/serviceapi/gen/quark/embedding/v1"
	indexerv1 "github.com/quarkloop/pkg/serviceapi/gen/quark/indexer/v1"
	"github.com/quarkloop/pkg/serviceapi/servicekit"
	"github.com/quarkloop/supervisor/pkg/api"
)

func TestAgentIndexesITCompanyMarkdownDocuments(t *testing.T) {
	indexerAddr := fmt.Sprintf("127.0.0.1:%d", utils.ReservePort(t))
	embeddingAddr := fmt.Sprintf("127.0.0.1:%d", utils.ReservePort(t))
	workingDir := t.TempDir()
	documentsDir := filepath.Join(workingDir, "company-records")
	documents := copyMarkdownDocuments(t, documentsDir, []markdownDocumentFixture{
		{
			Name:   "Aurora cloud migration invoice",
			Source: filepath.Join(utils.QuarkRoot(t), "e2e", "testdata", "it-company-documents", "invoice_2026_aurora_cloud_migration.md"),
			Query:  "invoice INV-2026-014 Northwind Retail Aurora cloud migration",
			Want:   []string{"INV-2026-014", "Northwind Retail GmbH", "EUR 18,450.00"},
		},
		{
			Name:   "Workstation equipment receipt",
			Source: filepath.Join(utils.QuarkRoot(t), "e2e", "testdata", "it-company-documents", "receipt_2026_workstation_equipment.md"),
			Query:  "receipt RCPT-2026-042 ByteWorks workstation equipment total paid",
			Want:   []string{"RCPT-2026-042", "ByteWorks Supply GmbH", "EUR 4,872.65"},
		},
		{
			Name:   "QuarkOps product catalog",
			Source: filepath.Join(utils.QuarkRoot(t), "e2e", "testdata", "it-company-documents", "product_catalog_quark_ops_platform.md"),
			Query:  "QuarkOps Observability Starter SKU monthly price SLA",
			Want:   []string{"QOP-OBS-START", "EUR 1,200.00", "99.9%"},
		},
		{
			Name:   "Acme managed IT support contract",
			Source: filepath.Join(utils.QuarkRoot(t), "e2e", "testdata", "it-company-documents", "support_contract_acme_managed_it.md"),
			Query:  "Acme Manufacturing Sentinel Managed IT contract renewal response target",
			Want:   []string{"MSA-ACME-2026-01", "Sentinel Managed IT", "4-hour"},
		},
	})

	embedding := utils.EmbeddingOptions{
		Plugin:     "embedding",
		Mode:       "local",
		Provider:   "local",
		Model:      "local-hash-v1",
		Dimensions: 32,
	}
	env := utils.StartE2E(t, true, utils.StartOptions{
		WorkingDir: workingDir,
		Embedding:  embedding,
		SupervisorEnv: map[string]string{
			"QUARK_INDEXER_ADDR":   indexerAddr,
			"QUARK_EMBEDDING_ADDR": embeddingAddr,
		},
		BeforeRuntime: func(t *testing.T, setup utils.RuntimeSetup, bins utils.BuiltBinaries) {
			t.Helper()
			dgraphAddr := utils.StartDgraph(t)
			startIndexerServiceAt(t, bins.Indexer, dgraphAddr, indexerAddr)
			startEmbeddingServiceAt(t, bins.Embedding, embeddingAddr, embedding)
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	indexSession, err := env.Sup.CreateSession(ctx, env.Space, api.CreateSessionRequest{
		Type:  api.SessionTypeChat,
		Title: "it-company-markdown-index",
	})
	if err != nil {
		t.Fatalf("create index session: %v", err)
	}
	utils.WaitForAgentSession(t, env, indexSession.ID, 10*time.Second)

	indexTrace := utils.PostMessageTraceWithOptions(t, ctx, env, indexSession.ID, indexMarkdownDirectoryPrompt(documentsDir, len(documents)), utils.MessageTraceOptions{
		Label:          "index IT company markdown documents",
		OverallTimeout: 6 * time.Minute,
		IdleTimeout:    90 * time.Second,
	})
	utils.Logf(t, "markdown index reply: %s", indexTrace.Text)
	writeArtifact(t, workingDir, "markdown-agent-index-reply.txt", indexTrace.Text)
	writeArtifact(t, workingDir, "markdown-agent-index-tools.txt", strings.Join(indexTrace.ToolStarts, "\n"))
	writeTraceArtifact(t, workingDir, "markdown-agent-index-tool-events.json", indexTrace)

	assertToolStarted(t, indexTrace, "fs")
	assertToolStarted(t, indexTrace, "embedding_Embed")
	assertToolStarted(t, indexTrace, "indexer_IndexDocument")
	assertNoToolErrors(t, indexTrace, "embedding_Embed", "indexer_IndexDocument")
	assertToolSuccessCount(t, indexTrace, "indexer_IndexDocument", len(documents))
	for _, document := range documents {
		if !containsText(indexTrace.Text, document.Filename) {
			t.Fatalf("markdown index confirmation missing filename %q:\n%s", document.Filename, indexTrace.Text)
		}
	}
	verifyPersistedMarkdownIndexState(t, ctx, workingDir, indexerAddr, embeddingAddr, documents)

	querySession, err := env.Sup.CreateSession(ctx, env.Space, api.CreateSessionRequest{
		Type:  api.SessionTypeChat,
		Title: "it-company-markdown-query",
	})
	if err != nil {
		t.Fatalf("create query session: %v", err)
	}
	utils.WaitForAgentSession(t, env, querySession.ID, 10*time.Second)

	queryTrace := utils.PostMessageTraceWithOptions(t, ctx, env, querySession.ID, indexedMarkdownQuestionPrompt(), utils.MessageTraceOptions{
		Label:          "query IT company markdown index",
		OverallTimeout: 4 * time.Minute,
		IdleTimeout:    90 * time.Second,
	})
	utils.Logf(t, "markdown query reply: %s", queryTrace.Text)
	writeArtifact(t, workingDir, "markdown-agent-query-reply.txt", queryTrace.Text)
	writeArtifact(t, workingDir, "markdown-agent-query-tools.txt", strings.Join(queryTrace.ToolStarts, "\n"))
	writeTraceArtifact(t, workingDir, "markdown-agent-query-tool-events.json", queryTrace)

	assertToolStarted(t, queryTrace, "embedding_Embed")
	assertToolStarted(t, queryTrace, "indexer_GetContext")
	assertNoToolErrors(t, queryTrace, "embedding_Embed", "indexer_GetContext")
	if contains(queryTrace.ToolStarts, "fs") {
		t.Fatalf("markdown query re-read source files instead of using the index; starts=%v", queryTrace.ToolStarts)
	}
	assertAnswerContains(t, queryTrace.Text,
		"INV-2026-014",
		"Northwind Retail GmbH",
		"RCPT-2026-042",
		"EUR 4,872.65",
		"QOP-OBS-START",
		"Sentinel Managed IT",
	)
	assertAnswerContainsAny(t, queryTrace.Text, "4-hour", "Critical incidents")
}

type markdownDocumentFixture struct {
	Name   string
	Source string
	Query  string
	Want   []string
}

type indexedMarkdownDocument struct {
	Name     string
	Path     string
	Filename string
	Query    string
	Want     []string
}

func copyMarkdownDocuments(t *testing.T, dir string, fixtures []markdownDocumentFixture) []indexedMarkdownDocument {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir markdown fixture dir: %v", err)
	}
	out := make([]indexedMarkdownDocument, 0, len(fixtures))
	for _, fixture := range fixtures {
		filename := filepath.Base(fixture.Source)
		dst := filepath.Join(dir, filename)
		copyTestFile(t, fixture.Source, dst)
		out = append(out, indexedMarkdownDocument{
			Name:     fixture.Name,
			Path:     dst,
			Filename: filename,
			Query:    fixture.Query,
			Want:     append([]string(nil), fixture.Want...),
		})
	}
	return out
}

func indexMarkdownDirectoryPrompt(directory string, documentCount int) string {
	return fmt.Sprintf(`Please index the Markdown documents in this company records directory for later structured Q&A:

%s

Treat every .md file under that directory as one user upload batch. Required workflow: first call fs list on the directory with recursive=true and include_hash=true. Then read every discovered .md file with fs read. For each Markdown file, create one compact searchable text chunk that preserves document identifiers, companies, products, dates, prices, totals, response targets, and source filename. Call embedding_Embed without setting dimensions for that chunk, then immediately call indexer_IndexDocument with the same file's embeddingRef before moving to the next file. In sourceMetadata include filename, path, relative_path when known, document_type, source_hash when known, embedding_provider, embedding_model, embedding_dimensions, and embedding_content_hash from the embedding result. Do not rename files, restructure the directory, write sidecars, or call services directly outside tools. Do not send a final answer until there are %d successful indexer_IndexDocument results, one for each Markdown file. Reply briefly with the filenames that are ready for questions.`, directory, documentCount)
}

func indexedMarkdownQuestionPrompt() string {
	return `Search the indexed IT company documents and answer from the indexed context only.

Return a concise structured answer covering:
- Which invoice is for Northwind Retail GmbH, what work was billed, and what total is due?
- Which receipt came from ByteWorks Supply GmbH, what equipment was purchased, and what was the total paid?
- Which QuarkOps catalog item has SKU QOP-OBS-START, and what are its monthly price and SLA?
- Which support contract covers Acme Manufacturing AG, what plan is it on, and what is the critical incident response target?

Do not re-read the source files. First embed this question with embedding_Embed. Then call indexer_GetContext with queryVectorRef, limit 10, and depth 1. Use the returned reasoningContext only, and include source filenames when available.`
}

func verifyPersistedMarkdownIndexState(t *testing.T, ctx context.Context, artifactDir, indexerAddr, embeddingAddr string, documents []indexedMarkdownDocument) {
	t.Helper()
	embeddingConn, err := servicekit.Dial(ctx, embeddingAddr)
	if err != nil {
		t.Fatalf("dial embedding service for markdown verification: %v", err)
	}
	defer embeddingConn.Close()
	indexerConn, err := servicekit.Dial(ctx, indexerAddr)
	if err != nil {
		t.Fatalf("dial indexer service for markdown verification: %v", err)
	}
	defer indexerConn.Close()

	embeddingClient := embeddingv1.NewEmbeddingServiceClient(embeddingConn)
	indexerClient := indexerv1.NewIndexerServiceClient(indexerConn)
	report := make([]map[string]any, 0, len(documents))
	for _, document := range documents {
		embedding, err := embeddingClient.Embed(ctx, &embeddingv1.EmbedRequest{
			Input: document.Query,
		})
		if err != nil {
			t.Fatalf("embed markdown verification query for %s: %v", document.Filename, err)
		}
		resp, err := indexerClient.GetContext(ctx, &indexerv1.QueryRequest{
			QueryVector: embedding.GetVector(),
			Limit:       1,
			Depth:       1,
			Filters:     map[string]string{"filename": document.Filename},
		})
		if err != nil {
			t.Fatalf("query persisted markdown index for %s: %v", document.Filename, err)
		}
		if len(resp.GetChunks()) == 0 {
			t.Fatalf("persisted markdown index state missing chunk for %s", document.Filename)
		}
		chunk := resp.GetChunks()[0]
		if got := chunk.GetMetadata()["filename"]; got != document.Filename {
			t.Fatalf("persisted markdown chunk filename = %q, want %q: %+v", got, document.Filename, chunk.GetMetadata())
		}
		if chunk.GetEmbeddingMetadata().GetDimensions() != embedding.GetDimensions() {
			t.Fatalf("persisted markdown embedding dimensions for %s = %d, want %d", document.Filename, chunk.GetEmbeddingMetadata().GetDimensions(), embedding.GetDimensions())
		}
		for _, want := range document.Want {
			if !containsText(chunk.GetText(), want) && !containsText(resp.GetReasoningContext(), want) {
				t.Fatalf("persisted markdown context for %s missing %q:\nchunk=%s\ncontext=%s", document.Filename, want, chunk.GetText(), resp.GetReasoningContext())
			}
		}
		report = append(report, map[string]any{
			"filename":           document.Filename,
			"chunk_id":           chunk.GetId(),
			"score":              chunk.GetScore(),
			"embedding_provider": chunk.GetEmbeddingMetadata().GetProvider(),
			"embedding_model":    chunk.GetEmbeddingMetadata().GetModel(),
			"embedding_dims":     chunk.GetEmbeddingMetadata().GetDimensions(),
			"metadata":           chunk.GetMetadata(),
			"context_confidence": resp.GetContextPackage().GetConfidence(),
		})
	}
	writeJSONArtifact(t, artifactDir, "markdown-direct-index-state.json", report)
}
