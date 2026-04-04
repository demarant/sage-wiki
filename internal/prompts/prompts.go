package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed templates/*.txt
var templateFS embed.FS

var templates *template.Template

func init() {
	var err error
	templates, err = template.ParseFS(templateFS, "templates/*.txt")
	if err != nil {
		panic(fmt.Sprintf("prompts: failed to parse templates: %v", err))
	}
}

// Render renders a named template with the given data.
func Render(name string, data any) (string, error) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, name+".txt", data); err != nil {
		return "", fmt.Errorf("prompts.Render(%s): %w", name, err)
	}
	return buf.String(), nil
}

// SummarizeData holds data for summarize templates.
type SummarizeData struct {
	SourcePath string
	SourceType string
	MaxTokens  int
}

// ExtractData holds data for concept extraction template.
type ExtractData struct {
	ExistingConcepts string
	Summaries        string
}

// WriteArticleData holds data for article writing template.
type WriteArticleData struct {
	ConceptName     string
	ConceptID       string
	Sources         string
	RelatedConcepts []string
	ExistingArticle string
	Learnings       string
	Aliases         string
	SourceList      string
	RelatedList     string
	Confidence      string
	MaxTokens       int
}

// CaptionData holds data for image captioning template.
type CaptionData struct {
	SourcePath string
}

// Available returns the names of all loaded templates.
func Available() []string {
	var names []string
	for _, t := range templates.Templates() {
		names = append(names, t.Name())
	}
	return names
}
