package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/xoai/sage-wiki/internal/embed"
	"github.com/xoai/sage-wiki/internal/llm"
	"github.com/xoai/sage-wiki/internal/log"
	"github.com/xoai/sage-wiki/internal/memory"
	"github.com/xoai/sage-wiki/internal/ontology"
	"github.com/xoai/sage-wiki/internal/vectors"
)

// ArticleResult holds the output of writing a concept article.
type ArticleResult struct {
	ConceptName string
	ArticlePath string
	Error       error
}

// WriteArticles runs Pass 3: write concept articles with ontology edges.
func WriteArticles(
	projectDir string,
	outputDir string,
	concepts []ExtractedConcept,
	client *llm.Client,
	model string,
	maxTokens int,
	maxParallel int,
	memStore *memory.Store,
	vecStore *vectors.Store,
	ontStore *ontology.Store,
	embedder embed.Embedder,
) []ArticleResult {
	if maxParallel <= 0 {
		maxParallel = 4
	}

	results := make([]ArticleResult, len(concepts))
	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup

	for i, concept := range concepts {
		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, c ExtractedConcept) {
			defer wg.Done()
			defer func() { <-sem }()

			result := writeOneArticle(projectDir, outputDir, c, client, model, maxTokens, memStore, vecStore, ontStore, embedder)
			results[idx] = result

			if result.Error != nil {
				log.Error("write article failed", "concept", c.Name, "error", result.Error)
			} else {
				log.Info("article written", "concept", c.Name, "path", result.ArticlePath)
			}
		}(i, concept)
	}

	wg.Wait()
	return results
}

func writeOneArticle(
	projectDir string,
	outputDir string,
	concept ExtractedConcept,
	client *llm.Client,
	model string,
	maxTokens int,
	memStore *memory.Store,
	vecStore *vectors.Store,
	ontStore *ontology.Store,
	embedder embed.Embedder,
) ArticleResult {
	result := ArticleResult{ConceptName: concept.Name}

	// Check for existing article
	articlePath := filepath.Join(outputDir, "concepts", concept.Name+".md")
	absPath := filepath.Join(projectDir, articlePath)
	var existingContent string
	if data, err := os.ReadFile(absPath); err == nil {
		existingContent = string(data)
	}

	// Build prompt
	relatedNames := findRelatedConcepts(concept)
	prompt := buildArticlePrompt(concept, existingContent, relatedNames)

	resp, err := client.ChatCompletion([]llm.Message{
		{Role: "system", Content: "You are a wiki author writing comprehensive, precise articles for a personal knowledge base. Use YAML frontmatter and [[wikilinks]]."},
		{Role: "user", Content: prompt},
	}, llm.CallOpts{Model: model, MaxTokens: maxTokens})
	if err != nil {
		result.Error = fmt.Errorf("llm call: %w", err)
		return result
	}

	articleContent := resp.Content

	// Ensure frontmatter exists
	if !strings.HasPrefix(articleContent, "---") {
		articleContent = buildFrontmatter(concept) + "\n\n" + articleContent
	}

	// Write article file
	articleDir := filepath.Join(projectDir, outputDir, "concepts")
	os.MkdirAll(articleDir, 0755)

	if err := os.WriteFile(absPath, []byte(articleContent), 0644); err != nil {
		result.Error = fmt.Errorf("write file: %w", err)
		return result
	}
	result.ArticlePath = articlePath

	// Create ontology entity
	entityType := ontology.TypeConcept
	if concept.Type == "technique" {
		entityType = ontology.TypeTechnique
	} else if concept.Type == "claim" {
		entityType = ontology.TypeClaim
	}

	if err := ontStore.AddEntity(ontology.Entity{
		ID:          concept.Name,
		Type:        entityType,
		Name:        formatConceptName(concept.Name),
		ArticlePath: articlePath,
	}); err != nil {
		log.Error("failed to create ontology entity", "concept", concept.Name, "error", err)
	}

	// Create source citation relations
	for _, src := range concept.Sources {
		// Create source entity if not exists
		if err := ontStore.AddEntity(ontology.Entity{
			ID:   src,
			Type: ontology.TypeSource,
			Name: filepath.Base(src),
		}); err != nil {
			log.Warn("failed to create source entity", "source", src, "error", err)
		}
		if err := ontStore.AddRelation(ontology.Relation{
			ID:       concept.Name + "-cites-" + sanitizeID(src),
			SourceID: concept.Name,
			TargetID: src,
			Relation: ontology.RelCites,
		}); err != nil {
			log.Warn("failed to create cites relation", "concept", concept.Name, "source", src, "error", err)
		}
	}

	// Index in FTS5
	if err := memStore.Add(memory.Entry{
		ID:          "concept:" + concept.Name,
		Content:     articleContent,
		Tags:        append([]string{entityType}, concept.Aliases...),
		ArticlePath: articlePath,
	}); err != nil {
		log.Error("failed to index article", "concept", concept.Name, "error", err)
	}

	// Generate embedding
	if embedder != nil {
		vec, err := embedder.Embed(articleContent)
		if err != nil {
			log.Warn("embedding failed for article", "concept", concept.Name, "error", err)
		} else {
			vecStore.Upsert("concept:"+concept.Name, vec)
		}
	}

	return result
}

func buildArticlePrompt(concept ExtractedConcept, existing string, related []string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Write a comprehensive wiki article about: %s\n\n", formatConceptName(concept.Name)))
	b.WriteString(fmt.Sprintf("Concept ID: %s\n", concept.Name))

	if len(concept.Aliases) > 0 {
		b.WriteString(fmt.Sprintf("Also known as: %s\n", strings.Join(concept.Aliases, ", ")))
	}

	b.WriteString(fmt.Sprintf("Sources: %s\n", strings.Join(concept.Sources, ", ")))

	if len(related) > 0 {
		b.WriteString(fmt.Sprintf("Related concepts: %s\n", strings.Join(related, ", ")))
	}

	if existing != "" {
		b.WriteString("\n## Existing article (update/expand):\n")
		b.WriteString(existing)
		b.WriteString("\n")
	}

	b.WriteString(`
Write the article with:
1. YAML frontmatter (concept, aliases, sources, confidence)
2. ## Definition — clear, precise
3. ## How it works — technical explanation
4. ## Variants — known variants if any
5. ## See also — [[wikilinks]] to related concepts

Use [[concept-name]] wikilinks for cross-references.`)

	return b.String()
}

func buildFrontmatter(concept ExtractedConcept) string {
	aliases := quoteYAMLList(concept.Aliases)
	sources := quoteYAMLList(concept.Sources)

	return fmt.Sprintf(`---
concept: %s
aliases: %s
sources: %s
confidence: medium
created_at: %s
---`, concept.Name, aliases, sources, timeNow())
}

// quoteYAMLList produces a YAML list with properly quoted values.
func quoteYAMLList(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = fmt.Sprintf("%q", item)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func formatConceptName(name string) string {
	words := strings.Split(name, "-")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func findRelatedConcepts(concept ExtractedConcept) []string {
	// Related concepts are discovered during extraction as co-occurrences
	// For now, return empty — the ontology will be populated as articles are written
	return nil
}

func sanitizeID(s string) string {
	return strings.NewReplacer("/", "-", "\\", "-", ".", "-", " ", "-").Replace(s)
}
