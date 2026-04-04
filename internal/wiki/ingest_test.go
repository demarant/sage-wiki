package wiki

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/xoai/sage-wiki/internal/manifest"
)

func TestIngestLocalFile(t *testing.T) {
	dir := t.TempDir()
	InitGreenfield(dir, "test")

	// Create a source file to ingest
	srcFile := filepath.Join(t.TempDir(), "article.md")
	os.WriteFile(srcFile, []byte("# Test Article\nSome content."), 0644)

	result, err := IngestPath(dir, srcFile)
	if err != nil {
		t.Fatalf("IngestPath: %v", err)
	}

	if result.Type != "article" {
		t.Errorf("expected article type, got %s", result.Type)
	}

	// Verify file was copied
	destPath := filepath.Join(dir, result.SourcePath)
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("ingested file should exist at destination")
	}

	// Verify manifest updated
	mf, _ := manifest.Load(filepath.Join(dir, ".manifest.json"))
	if mf.SourceCount() != 1 {
		t.Errorf("expected 1 source in manifest, got %d", mf.SourceCount())
	}
}

func TestIngestURL(t *testing.T) {
	dir := t.TempDir()
	InitGreenfield(dir, "test")

	// Disable SSRF check for test (mock server is on localhost)
	SkipSSRFCheck = true
	defer func() { SkipSSRFCheck = false }()

	// Mock web server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("# Web Article\n\nContent from the web."))
	}))
	defer server.Close()

	result, err := IngestURL(dir, server.URL+"/test-article")
	if err != nil {
		t.Fatalf("IngestURL: %v", err)
	}

	if result.Type != "article" {
		t.Errorf("expected article, got %s", result.Type)
	}

	// Verify file exists
	destPath := filepath.Join(dir, result.SourcePath)
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("ingested URL should be saved as file")
	}
}

func TestSlugifyURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://example.com/article", "example-com-article"},
		{"http://blog.test/post/123", "blog-test-post-123"},
	}
	for _, tt := range tests {
		got := slugifyURL(tt.input)
		if got != tt.expected {
			t.Errorf("slugifyURL(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
