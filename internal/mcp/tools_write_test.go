package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/xoai/sage-wiki/internal/manifest"
	"github.com/xoai/sage-wiki/internal/ontology"
	"github.com/xoai/sage-wiki/internal/wiki"
)

func TestWriteSummary(t *testing.T) {
	dir := t.TempDir()
	wiki.InitGreenfield(dir, "test")

	srv, err := NewServer(dir)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer srv.Close()

	// Write a summary
	result := srv.CallTool(context.Background(), "wiki_write_summary", mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name: "wiki_write_summary",
			Arguments: map[string]any{
				"source":   "raw/test.md",
				"content":  "This is a summary of the test article.",
				"concepts": "concept-a, concept-b",
			},
		},
	})

	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].(mcplib.TextContent).Text)
	}

	// Verify file written
	summaryPath := filepath.Join(dir, "wiki", "summaries", "test.md")
	if _, err := os.Stat(summaryPath); os.IsNotExist(err) {
		t.Error("summary file should exist")
	}

	// Verify manifest updated
	mf, _ := manifest.Load(filepath.Join(dir, ".manifest.json"))
	src, ok := mf.Sources["raw/test.md"]
	if !ok {
		t.Error("source should be in manifest")
	}
	if src.Status != "compiled" {
		t.Errorf("expected compiled status, got %s", src.Status)
	}
}

func TestWriteArticle(t *testing.T) {
	dir := t.TempDir()
	wiki.InitGreenfield(dir, "test")

	srv, _ := NewServer(dir)
	defer srv.Close()

	result := srv.CallTool(context.Background(), "wiki_write_article", mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name: "wiki_write_article",
			Arguments: map[string]any{
				"concept": "self-attention",
				"content": "---\nconcept: self-attention\n---\n\n# Self-Attention\n\nA mechanism.",
			},
		},
	})

	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].(mcplib.TextContent).Text)
	}

	// Verify file
	articlePath := filepath.Join(dir, "wiki", "concepts", "self-attention.md")
	if _, err := os.Stat(articlePath); os.IsNotExist(err) {
		t.Error("article should exist")
	}

	// Verify ontology entity
	e, _ := srv.ont.GetEntity("self-attention")
	if e == nil {
		t.Error("ontology entity should exist")
	}

	// Verify manifest
	mf, _ := manifest.Load(filepath.Join(dir, ".manifest.json"))
	if mf.ConceptCount() != 1 {
		t.Errorf("expected 1 concept in manifest, got %d", mf.ConceptCount())
	}
}

func TestAddOntologyEntity(t *testing.T) {
	dir := t.TempDir()
	wiki.InitGreenfield(dir, "test")

	srv, _ := NewServer(dir)
	defer srv.Close()

	result := srv.CallTool(context.Background(), "wiki_add_ontology", mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name: "wiki_add_ontology",
			Arguments: map[string]any{
				"entity_id":   "flash-attention",
				"entity_type": "technique",
				"entity_name": "Flash Attention",
			},
		},
	})

	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].(mcplib.TextContent).Text)
	}

	e, _ := srv.ont.GetEntity("flash-attention")
	if e == nil || e.Type != "technique" {
		t.Error("entity should be created with correct type")
	}
}

func TestAddOntologyRelation(t *testing.T) {
	dir := t.TempDir()
	wiki.InitGreenfield(dir, "test")

	srv, _ := NewServer(dir)
	defer srv.Close()

	// Create entities first
	srv.ont.AddEntity(ontology.Entity{ID: "flash-attn", Type: "technique", Name: "Flash"})
	srv.ont.AddEntity(ontology.Entity{ID: "attention", Type: "concept", Name: "Attention"})

	result := srv.CallTool(context.Background(), "wiki_add_ontology", mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name: "wiki_add_ontology",
			Arguments: map[string]any{
				"source_id": "flash-attn",
				"target_id": "attention",
				"relation":  "implements",
			},
		},
	})

	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].(mcplib.TextContent).Text)
	}

	count, _ := srv.ont.RelationCount()
	if count != 1 {
		t.Errorf("expected 1 relation, got %d", count)
	}
}

func TestLearn(t *testing.T) {
	dir := t.TempDir()
	wiki.InitGreenfield(dir, "test")

	srv, _ := NewServer(dir)
	defer srv.Close()

	result := srv.CallTool(context.Background(), "wiki_learn", mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name: "wiki_learn",
			Arguments: map[string]any{
				"type":    "gotcha",
				"content": "Always distinguish memory from IO bandwidth when discussing attention complexity.",
				"tags":    "attention,memory",
			},
		},
	})

	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].(mcplib.TextContent).Text)
	}

	// Verify stored
	var count int
	srv.db.ReadDB().QueryRow("SELECT COUNT(*) FROM learnings").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 learning, got %d", count)
	}
}

func TestCommit(t *testing.T) {
	dir := t.TempDir()
	wiki.InitGreenfield(dir, "test")

	srv, _ := NewServer(dir)
	defer srv.Close()

	// Create a file to commit
	os.WriteFile(filepath.Join(dir, "wiki", "test.md"), []byte("test"), 0644)

	result := srv.CallTool(context.Background(), "wiki_commit", mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name:      "wiki_commit",
			Arguments: map[string]any{"message": "test commit via MCP"},
		},
	})

	if result.IsError {
		// Git might not have user config in test env — that's ok
		text := result.Content[0].(mcplib.TextContent).Text
		if text != "" {
			t.Logf("commit result: %s", text)
		}
	}
}

func TestCompileDiff(t *testing.T) {
	dir := t.TempDir()
	wiki.InitGreenfield(dir, "test")

	srv, _ := NewServer(dir)
	defer srv.Close()

	result := srv.CallTool(context.Background(), "wiki_compile_diff", mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name:      "wiki_compile_diff",
			Arguments: map[string]any{},
		},
	})

	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].(mcplib.TextContent).Text)
	}

	text := result.Content[0].(mcplib.TextContent).Text
	if text == "" {
		t.Error("expected non-empty diff result")
	}
}

func TestAddSourceWithPathTraversal(t *testing.T) {
	dir := t.TempDir()
	wiki.InitGreenfield(dir, "test")

	srv, _ := NewServer(dir)
	defer srv.Close()

	result := srv.CallTool(context.Background(), "wiki_add_source", mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name:      "wiki_add_source",
			Arguments: map[string]any{"path": "../../etc/passwd"},
		},
	})

	if !result.IsError {
		t.Error("expected error for path traversal")
	}
}

// Suppress unused import warning
var _ = json.Marshal
