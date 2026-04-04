package compiler

import (
	"github.com/xoai/sage-wiki/internal/log"
)

// ExtractImages runs Pass 4: extract and caption images from sources.
// Currently a placeholder — PDF image extraction will be implemented
// when ledongthuc/pdf is fully integrated. For now, this pass is a no-op.
func ExtractImages(projectDir string, outputDir string, sources []SourceInfo) {
	// Count sources that might have images
	pdfCount := 0
	for _, s := range sources {
		if s.Type == "paper" {
			pdfCount++
		}
	}

	if pdfCount > 0 {
		log.Info("Pass 4: image extraction skipped (PDF support pending)", "pdf_sources", pdfCount)
	}
}
