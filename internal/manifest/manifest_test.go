package manifest

import (
	"path/filepath"
	"testing"
)

func TestNewManifest(t *testing.T) {
	m := New()
	if m.Version != 2 {
		t.Errorf("expected version 2, got %d", m.Version)
	}
	if len(m.Sources) != 0 {
		t.Error("expected empty sources")
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".manifest.json")

	m := New()
	m.AddSource("raw/paper.pdf", "sha256:abc", "paper", 1024)
	m.AddConcept("attention", "wiki/concepts/attention.md", []string{"raw/paper.pdf"})

	if err := m.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.SourceCount() != 1 {
		t.Errorf("expected 1 source, got %d", loaded.SourceCount())
	}
	if loaded.ConceptCount() != 1 {
		t.Errorf("expected 1 concept, got %d", loaded.ConceptCount())
	}

	s := loaded.Sources["raw/paper.pdf"]
	if s.Hash != "sha256:abc" {
		t.Errorf("expected hash sha256:abc, got %q", s.Hash)
	}
	if s.Status != "pending" {
		t.Errorf("expected pending, got %q", s.Status)
	}
}

func TestLoadMissing(t *testing.T) {
	m, err := Load("/nonexistent/path/.manifest.json")
	if err != nil {
		t.Fatalf("Load missing should return empty manifest, got error: %v", err)
	}
	if m.SourceCount() != 0 {
		t.Error("expected empty manifest")
	}
}

func TestMarkCompiled(t *testing.T) {
	m := New()
	m.AddSource("raw/a.md", "sha256:xyz", "article", 500)
	m.MarkCompiled("raw/a.md", "wiki/summaries/a.md", []string{"attention"})

	s := m.Sources["raw/a.md"]
	if s.Status != "compiled" {
		t.Errorf("expected compiled, got %q", s.Status)
	}
	if s.SummaryPath != "wiki/summaries/a.md" {
		t.Errorf("expected summary path, got %q", s.SummaryPath)
	}
}

func TestPendingSources(t *testing.T) {
	m := New()
	m.AddSource("raw/a.md", "h1", "article", 100)
	m.AddSource("raw/b.md", "h2", "article", 200)
	m.MarkCompiled("raw/a.md", "wiki/summaries/a.md", nil)

	pending := m.PendingSources()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending, got %d", len(pending))
	}
	if _, ok := pending["raw/b.md"]; !ok {
		t.Error("expected raw/b.md to be pending")
	}
}

func TestRemoveSource(t *testing.T) {
	m := New()
	m.AddSource("raw/a.md", "h1", "article", 100)
	m.RemoveSource("raw/a.md")
	if m.SourceCount() != 0 {
		t.Error("expected 0 after remove")
	}
}
