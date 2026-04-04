package prompts

import (
	"strings"
	"testing"
)

func TestAvailable(t *testing.T) {
	names := Available()
	if len(names) == 0 {
		t.Fatal("no templates loaded")
	}

	expected := []string{"summarize_article.txt", "summarize_paper.txt", "extract_concepts.txt", "write_article.txt", "caption_image.txt"}
	for _, exp := range expected {
		found := false
		for _, name := range names {
			if name == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing template %q", exp)
		}
	}
}

func TestRenderSummarize(t *testing.T) {
	result, err := Render("summarize_article", SummarizeData{
		SourcePath: "raw/articles/test.md",
		SourceType: "article",
		MaxTokens:  2000,
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if !strings.Contains(result, "raw/articles/test.md") {
		t.Error("expected source path in output")
	}
	if !strings.Contains(result, "2000") {
		t.Error("expected max tokens in output")
	}
	if !strings.Contains(result, "## Key claims") {
		t.Error("expected Key claims section")
	}
}

func TestRenderWriteArticle(t *testing.T) {
	result, err := Render("write_article", WriteArticleData{
		ConceptName:     "Self-Attention",
		ConceptID:       "self-attention",
		Sources:         "attention-paper, transformer-explainer",
		RelatedConcepts: []string{"multi-head-attention", "positional-encoding"},
		MaxTokens:       4000,
		Confidence:      "high",
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if !strings.Contains(result, "Self-Attention") {
		t.Error("expected concept name")
	}
	if !strings.Contains(result, "[[multi-head-attention]]") {
		t.Error("expected wikilinks in See also")
	}
}

func TestRenderCaption(t *testing.T) {
	result, err := Render("caption_image", CaptionData{
		SourcePath: "raw/papers/figure1.png",
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(result, "raw/papers/figure1.png") {
		t.Error("expected source path")
	}
}
