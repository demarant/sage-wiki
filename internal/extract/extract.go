package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SourceContent holds extracted text from a source file.
type SourceContent struct {
	Path       string
	Type       string // article, paper, code
	Text       string
	Frontmatter string
	Chunks     []Chunk
	ChunkCount int
}

// Chunk represents a section of a large source.
type Chunk struct {
	Index   int
	Text    string
	Heading string // section heading if available
}

// Extract reads and extracts text from a source file.
func Extract(path string, sourceType string) (*SourceContent, error) {
	ext := strings.ToLower(filepath.Ext(path))

	switch {
	case ext == ".md" || ext == ".txt":
		return extractMarkdown(path, sourceType)
	case ext == ".pdf":
		return extractPDF(path)
	case isCodeFile(ext):
		return extractCode(path)
	default:
		return extractMarkdown(path, sourceType) // treat unknown as text
	}
}

// ChunkIfNeeded splits content into chunks if it exceeds maxTokens.
// Uses a rough estimate of 4 chars per token.
func ChunkIfNeeded(content *SourceContent, maxTokens int) {
	estimatedTokens := len(content.Text) / 4
	if estimatedTokens <= maxTokens || maxTokens <= 0 {
		content.Chunks = []Chunk{{Index: 0, Text: content.Text}}
		content.ChunkCount = 1
		return
	}

	// Split markdown by headings
	if strings.Contains(content.Text, "\n## ") || strings.Contains(content.Text, "\n# ") {
		content.Chunks = splitByHeadings(content.Text, maxTokens)
	} else {
		content.Chunks = splitByParagraphs(content.Text, maxTokens)
	}
	content.ChunkCount = len(content.Chunks)
}

func extractMarkdown(path string, sourceType string) (*SourceContent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("extract markdown: %w", err)
	}

	text := string(data)
	var frontmatter string

	// Extract YAML frontmatter
	if strings.HasPrefix(text, "---\n") {
		end := strings.Index(text[4:], "\n---")
		if end >= 0 {
			frontmatter = text[4 : 4+end]
			text = strings.TrimSpace(text[4+end+4:])
		}
	}

	if sourceType == "" || sourceType == "auto" {
		sourceType = "article"
	}

	return &SourceContent{
		Path:        path,
		Type:        sourceType,
		Text:        text,
		Frontmatter: frontmatter,
	}, nil
}

func extractPDF(path string) (*SourceContent, error) {
	// Basic PDF text extraction
	// In M2 we provide a placeholder — full ledongthuc/pdf integration
	// will be added when the dependency is wired. For now, return an error
	// that guides the user.
	return nil, fmt.Errorf("PDF extraction not yet implemented for %s — add .md sources first", filepath.Base(path))
}

func extractCode(path string) (*SourceContent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("extract code: %w", err)
	}

	return &SourceContent{
		Path: path,
		Type: "code",
		Text: string(data),
	}, nil
}

func isCodeFile(ext string) bool {
	codeExts := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true,
		".java": true, ".rs": true, ".c": true, ".cpp": true,
		".rb": true, ".swift": true, ".kt": true,
	}
	return codeExts[ext]
}

// splitByHeadings splits markdown text on heading boundaries.
func splitByHeadings(text string, maxTokens int) []Chunk {
	lines := strings.Split(text, "\n")
	var chunks []Chunk
	var current strings.Builder
	var currentHeading string
	chunkIdx := 0

	flush := func() {
		if current.Len() > 0 {
			chunks = append(chunks, Chunk{
				Index:   chunkIdx,
				Text:    strings.TrimSpace(current.String()),
				Heading: currentHeading,
			})
			chunkIdx++
			current.Reset()
		}
	}

	for _, line := range lines {
		isHeading := strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ")

		// Check if adding this line would exceed limit
		estimatedTokens := (current.Len() + len(line)) / 4
		if estimatedTokens > maxTokens && current.Len() > 0 {
			flush()
		}

		if isHeading && current.Len() > 0 {
			flush()
			currentHeading = stripHeadingPrefix(line)
		} else if isHeading {
			currentHeading = stripHeadingPrefix(line)
		}

		current.WriteString(line)
		current.WriteString("\n")
	}

	flush()
	return chunks
}

// stripHeadingPrefix removes markdown heading markers (# ## ###) from a line.
func stripHeadingPrefix(line string) string {
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	for i < len(line) && line[i] == ' ' {
		i++
	}
	return line[i:]
}

// splitByParagraphs splits on double newlines when no headings exist.
func splitByParagraphs(text string, maxTokens int) []Chunk {
	paragraphs := strings.Split(text, "\n\n")
	var chunks []Chunk
	var current strings.Builder
	chunkIdx := 0
	maxChars := maxTokens * 4

	for _, para := range paragraphs {
		if current.Len()+len(para) > maxChars && current.Len() > 0 {
			chunks = append(chunks, Chunk{
				Index: chunkIdx,
				Text:  strings.TrimSpace(current.String()),
			})
			chunkIdx++
			current.Reset()
		}
		current.WriteString(para)
		current.WriteString("\n\n")
	}

	if current.Len() > 0 {
		chunks = append(chunks, Chunk{
			Index: chunkIdx,
			Text:  strings.TrimSpace(current.String()),
		})
	}

	return chunks
}

// DetectSourceType guesses source type from file extension.
func DetectSourceType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pdf":
		return "paper"
	case ".md", ".txt":
		return "article"
	default:
		if isCodeFile(ext) {
			return "code"
		}
		return "article"
	}
}
