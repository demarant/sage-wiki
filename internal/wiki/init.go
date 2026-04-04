package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xoai/sage-wiki/internal/config"
	gitpkg "github.com/xoai/sage-wiki/internal/git"
	"github.com/xoai/sage-wiki/internal/log"
	"github.com/xoai/sage-wiki/internal/storage"
)

// InitGreenfield creates a new sage-wiki project from scratch.
func InitGreenfield(dir string, project string) error {
	// Create directories
	dirs := []string{
		filepath.Join(dir, "raw"),
		filepath.Join(dir, "wiki", "summaries"),
		filepath.Join(dir, "wiki", "concepts"),
		filepath.Join(dir, "wiki", "connections"),
		filepath.Join(dir, "wiki", "outputs"),
		filepath.Join(dir, "wiki", "images"),
		filepath.Join(dir, "wiki", "archive"),
		filepath.Join(dir, ".sage"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("init: create %s: %w", d, err)
		}
	}

	// Write config
	cfg := config.Defaults()
	cfg.Project = project
	cfg.Description = fmt.Sprintf("sage-wiki project: %s", project)
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := cfg.Save(cfgPath); err != nil {
		return fmt.Errorf("init: save config: %w", err)
	}

	// Create SQLite DB
	dbPath := filepath.Join(dir, ".sage", "wiki.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		return fmt.Errorf("init: create db: %w", err)
	}
	db.Close()

	// Write .gitignore
	gitignore := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(gitignore, []byte(".sage/\n"), 0644); err != nil {
		return fmt.Errorf("init: write .gitignore: %w", err)
	}

	// Write empty manifest
	manifestPath := filepath.Join(dir, ".manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"version":2,"sources":{},"concepts":{}}`+"\n"), 0644); err != nil {
		return fmt.Errorf("init: write manifest: %w", err)
	}

	// Git init
	if gitpkg.IsAvailable() {
		if err := gitpkg.Init(dir); err != nil {
			log.Warn("git init failed", "error", err)
		}
	}

	log.Info("project initialized", "mode", "greenfield", "dir", dir)
	return nil
}

// InitVaultOverlay initializes sage-wiki on an existing Obsidian vault.
func InitVaultOverlay(dir string, project string, sourceFolders []string, ignoreFolders []string, output string) error {
	if output == "" {
		output = "_wiki"
	}

	// Create output directories
	outputDir := filepath.Join(dir, output)
	subdirs := []string{"summaries", "concepts", "connections", "outputs", "images", "archive"}
	for _, sub := range subdirs {
		if err := os.MkdirAll(filepath.Join(outputDir, sub), 0755); err != nil {
			return fmt.Errorf("init: create %s: %w", sub, err)
		}
	}

	// Create .sage
	if err := os.MkdirAll(filepath.Join(dir, ".sage"), 0755); err != nil {
		return fmt.Errorf("init: create .sage: %w", err)
	}

	// Build config
	cfg := config.Defaults()
	cfg.Project = project
	cfg.Description = fmt.Sprintf("Obsidian vault with sage-wiki: %s", project)
	cfg.Vault = &config.VaultConfig{Root: "."}
	cfg.Output = output

	// Source folders
	cfg.Sources = make([]config.Source, len(sourceFolders))
	for i, sf := range sourceFolders {
		cfg.Sources[i] = config.Source{
			Path:  sf,
			Type:  "article", // default type
			Watch: true,
		}
	}

	// Ignore list (include output dir)
	cfg.Ignore = append(ignoreFolders, output)

	cfgPath := filepath.Join(dir, "config.yaml")
	if err := cfg.Save(cfgPath); err != nil {
		return fmt.Errorf("init: save config: %w", err)
	}

	// Create SQLite DB
	dbPath := filepath.Join(dir, ".sage", "wiki.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		return fmt.Errorf("init: create db: %w", err)
	}
	db.Close()

	// Write manifest
	manifestPath := filepath.Join(dir, ".manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"version":2,"sources":{},"concepts":{}}`+"\n"), 0644); err != nil {
		return fmt.Errorf("init: write manifest: %w", err)
	}

	log.Info("project initialized", "mode", "vault-overlay", "dir", dir, "sources", sourceFolders)
	return nil
}

// ScanVaultFolders scans a directory and returns folder names with file counts.
type FolderInfo struct {
	Name      string
	FileCount int
	HasMD     bool
	HasPDF    bool
}

// ScanFolders lists top-level folders with file statistics.
func ScanFolders(dir string) ([]FolderInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}

	var folders []FolderInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip hidden and system folders
		if strings.HasPrefix(name, ".") || name == "_wiki" {
			continue
		}

		info := FolderInfo{Name: name}
		filepath.WalkDir(filepath.Join(dir, name), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			info.FileCount++
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".md" {
				info.HasMD = true
			} else if ext == ".pdf" {
				info.HasPDF = true
			}
			return nil
		})

		folders = append(folders, info)
	}

	return folders, nil
}
