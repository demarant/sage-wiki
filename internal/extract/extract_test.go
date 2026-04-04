package extract

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractMarkdown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")

	content := `---
title: Test Article
tags: [attention, transformer]
---

# Self-Attention

Self-attention is a mechanism for computing contextual representations.

## How it works

It uses queries, keys, and values.
`
	os.WriteFile(path, []byte(content), 0644)

	result, err := Extract(path, "article")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if result.Type != "article" {
		t.Errorf("expected article, got %s", result.Type)
	}
	if result.Frontmatter == "" {
		t.Error("expected frontmatter to be extracted")
	}
	if result.Text == "" {
		t.Error("expected body text")
	}
	// Frontmatter should be stripped from text
	if result.Text[:1] == "-" {
		t.Error("frontmatter should be stripped from text")
	}
}

func TestExtractCode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	os.WriteFile(path, []byte("package main\nfunc main() {}"), 0644)

	result, err := Extract(path, "")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if result.Type != "code" {
		t.Errorf("expected code, got %s", result.Type)
	}
}

func TestChunkIfNeededSmall(t *testing.T) {
	content := &SourceContent{
		Text: "Short text that fits in one chunk.",
	}
	ChunkIfNeeded(content, 1000)

	if content.ChunkCount != 1 {
		t.Errorf("expected 1 chunk, got %d", content.ChunkCount)
	}
}

func TestChunkByHeadings(t *testing.T) {
	text := `# Introduction

This is the intro section with plenty of text to make it substantial enough.

## Methods

This is the methods section with details about the approach.

## Results

This is the results section with findings.
`
	content := &SourceContent{Text: text}
	ChunkIfNeeded(content, 20) // Very small token limit to force chunking

	if content.ChunkCount < 2 {
		t.Errorf("expected multiple chunks, got %d", content.ChunkCount)
	}

	// Each chunk should have content
	for i, chunk := range content.Chunks {
		if chunk.Text == "" {
			t.Errorf("chunk %d is empty", i)
		}
	}
}

func TestChunkByParagraphs(t *testing.T) {
	// No headings — should split on double newlines
	text := "Paragraph one with some text.\n\nParagraph two with more text.\n\nParagraph three with even more text."

	content := &SourceContent{Text: text}
	ChunkIfNeeded(content, 10) // Very small limit

	if content.ChunkCount < 2 {
		t.Errorf("expected multiple chunks, got %d", content.ChunkCount)
	}
}

func TestDetectSourceType(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"paper.pdf", "paper"},
		{"article.md", "article"},
		{"notes.txt", "article"},
		{"main.go", "code"},
		{"script.py", "code"},
		{"data.csv", "article"},
	}
	for _, tt := range tests {
		got := DetectSourceType(tt.path)
		if got != tt.expected {
			t.Errorf("DetectSourceType(%s) = %s, want %s", tt.path, got, tt.expected)
		}
	}
}
