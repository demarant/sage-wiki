package compiler

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/xoai/sage-wiki/internal/llm"
	"github.com/xoai/sage-wiki/internal/log"
	"github.com/xoai/sage-wiki/internal/manifest"
)

// ExtractedConcept represents a concept identified by the LLM.
type ExtractedConcept struct {
	Name    string   `json:"name"`
	Aliases []string `json:"aliases,omitempty"`
	Sources []string `json:"sources"`
	Type    string   `json:"type"` // concept, technique, claim
}

// ExtractConcepts runs Pass 2: concept extraction from summaries.
// It takes new/updated summaries and the existing concept list,
// asks the LLM to identify and deduplicate concepts.
func ExtractConcepts(
	summaries []SummaryResult,
	existingConcepts map[string]manifest.Concept,
	client *llm.Client,
	model string,
) ([]ExtractedConcept, error) {
	if len(summaries) == 0 {
		return nil, nil
	}

	// Build existing concept list for dedup context
	var existingList []string
	for name := range existingConcepts {
		existingList = append(existingList, name)
	}

	// Build summary texts
	var summaryTexts []string
	for _, s := range summaries {
		if s.Error != nil || s.Summary == "" {
			continue
		}
		summaryTexts = append(summaryTexts, fmt.Sprintf("### Source: %s\n%s", s.SourcePath, s.Summary))
	}

	if len(summaryTexts) == 0 {
		return nil, nil
	}

	prompt := fmt.Sprintf(`Extract concepts from these summaries of recently added/modified sources.

## Existing concepts (do not duplicate — merge with these when appropriate):
%s

## New/updated summaries:
%s

For each concept, provide:
- name: lowercase-hyphenated identifier (e.g., "self-attention")
- aliases: alternative names (e.g., ["scaled dot-product attention"])
- sources: which source file paths mention this concept
- type: one of "concept", "technique", or "claim"

Merge with existing concepts when you detect aliases or synonyms.
Output ONLY a JSON array of objects. No markdown, no explanation.`,
		strings.Join(existingList, ", "),
		strings.Join(summaryTexts, "\n\n---\n\n"),
	)

	resp, err := client.ChatCompletion([]llm.Message{
		{Role: "system", Content: "You are a concept extraction system for a knowledge wiki. Output valid JSON only."},
		{Role: "user", Content: prompt},
	}, llm.CallOpts{Model: model, MaxTokens: 4096})
	if err != nil {
		return nil, fmt.Errorf("concept extraction LLM call: %w", err)
	}

	// Parse JSON response — try to extract JSON array from response
	concepts, err := parseConceptsJSON(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("concept extraction parse: %w", err)
	}

	log.Info("concepts extracted", "count", len(concepts))
	return concepts, nil
}

// parseConceptsJSON extracts a JSON array from the LLM response.
// Handles cases where the LLM wraps JSON in markdown code fences.
func parseConceptsJSON(text string) ([]ExtractedConcept, error) {
	text = strings.TrimSpace(text)

	// Strip markdown code fences if present
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		var jsonLines []string
		inBlock := false
		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				inBlock = !inBlock
				continue
			}
			if inBlock {
				jsonLines = append(jsonLines, line)
			}
		}
		text = strings.Join(jsonLines, "\n")
	}

	// Find the JSON array
	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")
	if start >= 0 && end > start {
		text = text[start : end+1]
	}

	var concepts []ExtractedConcept
	if err := json.Unmarshal([]byte(text), &concepts); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w\nraw: %s", err, text[:min(200, len(text))])
	}

	return concepts, nil
}

