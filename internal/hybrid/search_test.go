package hybrid

import (
	"path/filepath"
	"testing"

	"github.com/xoai/sage-wiki/internal/memory"
	"github.com/xoai/sage-wiki/internal/storage"
	"github.com/xoai/sage-wiki/internal/vectors"
)

func setupTest(t *testing.T) (*Searcher, *memory.Store, *vectors.Store) {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	mem := memory.NewStore(db)
	vec := vectors.NewStore(db)
	return NewSearcher(mem, vec), mem, vec
}

func TestBM25OnlySearch(t *testing.T) {
	searcher, mem, _ := setupTest(t)

	mem.Add(memory.Entry{ID: "e1", Content: "attention mechanism in transformers", Tags: []string{"attention"}, ArticlePath: "a.md"})
	mem.Add(memory.Entry{ID: "e2", Content: "convolutional neural networks", Tags: []string{"cnn"}, ArticlePath: "b.md"})

	results, err := searcher.Search(SearchOpts{
		Query: "attention transformer",
		Limit: 10,
	}, nil) // nil queryVec = BM25 only
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if results[0].ID != "e1" {
		t.Errorf("expected e1 first, got %s", results[0].ID)
	}
	if results[0].VectorRank != 0 {
		t.Error("vector rank should be 0 in BM25-only mode")
	}
}

func TestHybridRRFFusion(t *testing.T) {
	searcher, mem, vec := setupTest(t)

	// e1: strong BM25 match (contains query terms)
	mem.Add(memory.Entry{ID: "e1", Content: "attention mechanism in transformers architecture", Tags: []string{"attention"}, ArticlePath: "a.md"})
	// e2: weak BM25 but should have good vector similarity
	mem.Add(memory.Entry{ID: "e2", Content: "self-attention scaled dot product", Tags: []string{"attention"}, ArticlePath: "b.md"})
	// e3: irrelevant
	mem.Add(memory.Entry{ID: "e3", Content: "database indexing strategies", Tags: []string{"database"}, ArticlePath: "c.md"})

	// Add vectors — e2 is closest to query vector
	vec.Upsert("e1", []float32{0.5, 0.5, 0.0})
	vec.Upsert("e2", []float32{0.9, 0.1, 0.0})
	vec.Upsert("e3", []float32{0.0, 0.0, 1.0})

	queryVec := []float32{1.0, 0.0, 0.0}

	results, err := searcher.Search(SearchOpts{
		Query: "attention transformer",
		Limit: 10,
	}, queryVec)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}

	// Both e1 and e2 should be in top results
	topIDs := map[string]bool{}
	for _, r := range results {
		topIDs[r.ID] = true
	}
	if !topIDs["e1"] || !topIDs["e2"] {
		t.Error("expected both e1 and e2 in results")
	}

	// RRF scores should be positive
	for _, r := range results {
		if r.RRFScore <= 0 {
			t.Errorf("expected positive RRF score, got %f for %s", r.RRFScore, r.ID)
		}
	}
}

func TestTagBoost(t *testing.T) {
	tests := []struct {
		entryTags []string
		boostTags []string
		expected  float64
	}{
		{nil, nil, 0},
		{[]string{"a", "b"}, []string{"a"}, 0.03},
		{[]string{"a", "b"}, []string{"a", "b"}, 0.06},
		{[]string{"a", "b", "c", "d", "e", "f"}, []string{"a", "b", "c", "d", "e", "f"}, 0.15}, // capped
	}
	for _, tt := range tests {
		got := tagBoost(tt.entryTags, tt.boostTags)
		if got != tt.expected {
			t.Errorf("tagBoost(%v, %v) = %f, want %f", tt.entryTags, tt.boostTags, got, tt.expected)
		}
	}
}

func TestSearchWithTagFilter(t *testing.T) {
	searcher, mem, _ := setupTest(t)

	mem.Add(memory.Entry{ID: "e1", Content: "attention mechanism", Tags: []string{"attention"}, ArticlePath: "a.md"})
	mem.Add(memory.Entry{ID: "e2", Content: "attention optimization", Tags: []string{"attention", "perf"}, ArticlePath: "b.md"})

	results, _ := searcher.Search(SearchOpts{
		Query: "attention",
		Tags:  []string{"perf"},
		Limit: 10,
	}, nil)

	if len(results) != 1 {
		t.Fatalf("expected 1 result with tag filter, got %d", len(results))
	}
	if results[0].ID != "e2" {
		t.Errorf("expected e2, got %s", results[0].ID)
	}
}

func TestBoostTagsApplied(t *testing.T) {
	searcher, mem, _ := setupTest(t)

	mem.Add(memory.Entry{ID: "e1", Content: "attention mechanism", Tags: []string{"concept"}, ArticlePath: "a.md"})
	mem.Add(memory.Entry{ID: "e2", Content: "attention mechanism detail", Tags: []string{"concept", "important"}, ArticlePath: "b.md"})

	results, _ := searcher.Search(SearchOpts{
		Query:     "attention",
		BoostTags: []string{"important"},
		Limit:     10,
	}, nil)

	if len(results) < 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// e2 should be boosted above e1 due to tag match
	if results[0].ID != "e2" {
		t.Errorf("expected e2 first (tag boost), got %s", results[0].ID)
	}
}
