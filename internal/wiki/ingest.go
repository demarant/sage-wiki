package wiki

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xoai/sage-wiki/internal/config"
	"github.com/xoai/sage-wiki/internal/extract"
	"github.com/xoai/sage-wiki/internal/log"
	"github.com/xoai/sage-wiki/internal/manifest"
)

// IngestResult holds the outcome of an ingest operation.
type IngestResult struct {
	SourcePath string
	Type       string
	Size       int64
}

// IngestURL downloads a URL as markdown and saves to the source folder.
const maxIngestBytes = 50 * 1024 * 1024 // 50MB max download

// SkipSSRFCheck disables SSRF validation. Only for testing.
var SkipSSRFCheck bool

func IngestURL(projectDir string, url string) (*IngestResult, error) {
	// Validate URL scheme
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return nil, fmt.Errorf("ingest: only http/https URLs are supported")
	}

	// Block private/internal IPs (SSRF protection)
	if !SkipSSRFCheck {
		if err := validatePublicURL(url); err != nil {
			return nil, err
		}
	}

	cfg, err := config.Load(filepath.Join(projectDir, "config.yaml"))
	if err != nil {
		return nil, err
	}

	// Download with size limit
	client := http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("ingest: download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ingest: HTTP %d for %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxIngestBytes))
	if err != nil {
		return nil, fmt.Errorf("ingest: read body: %w", err)
	}

	// Convert to markdown-ish format (basic: wrap in frontmatter)
	content := fmt.Sprintf("---\nsource_url: %s\ningested_at: %s\n---\n\n%s",
		url, time.Now().UTC().Format(time.RFC3339), string(body))

	// Find first article-type source folder
	destDir := findSourceFolder(projectDir, cfg, "article")
	if destDir == "" {
		return nil, fmt.Errorf("ingest: no article source folder configured")
	}

	// Generate filename from URL
	filename := slugifyURL(url) + ".md"
	destPath := filepath.Join(destDir, filename)
	relPath, _ := filepath.Rel(projectDir, destPath)

	if err := os.WriteFile(destPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("ingest: write: %w", err)
	}

	// Update manifest
	hash := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(content)))
	mfPath := filepath.Join(projectDir, ".manifest.json")
	mf, err := manifest.Load(mfPath)
	if err != nil {
		return nil, fmt.Errorf("ingest: load manifest: %w", err)
	}
	mf.AddSource(relPath, hash, "article", int64(len(content)))
	if err := mf.Save(mfPath); err != nil {
		return nil, fmt.Errorf("ingest: save manifest: %w", err)
	}

	log.Info("ingested URL", "url", url, "path", relPath)
	return &IngestResult{SourcePath: relPath, Type: "article", Size: int64(len(content))}, nil
}

// IngestPath copies a local file to the appropriate source folder.
func IngestPath(projectDir string, srcPath string) (*IngestResult, error) {
	cfg, err := config.Load(filepath.Join(projectDir, "config.yaml"))
	if err != nil {
		return nil, err
	}

	absPath, err := filepath.Abs(srcPath)
	if err != nil {
		return nil, fmt.Errorf("ingest: invalid path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("ingest: file not found: %w", err)
	}

	srcType := extract.DetectSourceType(absPath)
	destDir := findSourceFolder(projectDir, cfg, srcType)
	if destDir == "" {
		// Fallback to first source folder
		if len(cfg.Sources) > 0 {
			destDir = filepath.Join(projectDir, cfg.Sources[0].Path)
		} else {
			return nil, fmt.Errorf("ingest: no source folder configured")
		}
	}

	os.MkdirAll(destDir, 0755)

	destPath := filepath.Join(destDir, filepath.Base(absPath))
	relPath, _ := filepath.Rel(projectDir, destPath)

	// Copy file
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("ingest: read source: %w", err)
	}
	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return nil, fmt.Errorf("ingest: write dest: %w", err)
	}

	// Update manifest
	hash := fmt.Sprintf("sha256:%x", sha256.Sum256(data))
	mfPath := filepath.Join(projectDir, ".manifest.json")
	mf, err := manifest.Load(mfPath)
	if err != nil {
		return nil, fmt.Errorf("ingest: load manifest: %w", err)
	}
	mf.AddSource(relPath, hash, srcType, info.Size())
	if err := mf.Save(mfPath); err != nil {
		return nil, fmt.Errorf("ingest: save manifest: %w", err)
	}

	log.Info("ingested file", "source", absPath, "dest", relPath)
	return &IngestResult{SourcePath: relPath, Type: srcType, Size: info.Size()}, nil
}

func findSourceFolder(projectDir string, cfg *config.Config, sourceType string) string {
	// Map source types to config types
	typeMap := map[string]string{
		"article": "article",
		"paper":   "paper",
		"code":    "code",
	}
	configType := typeMap[sourceType]

	// First try exact type match
	for _, s := range cfg.Sources {
		if s.Type == configType || s.Type == "auto" {
			return filepath.Join(projectDir, s.Path)
		}
	}

	// Fallback to first source
	if len(cfg.Sources) > 0 {
		return filepath.Join(projectDir, cfg.Sources[0].Path)
	}

	return ""
}

// validatePublicURL blocks requests to private/internal IP ranges.
func validatePublicURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("ingest: invalid URL: %w", err)
	}

	host := parsed.Hostname()

	// Resolve hostname to IP
	ips, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("ingest: DNS lookup failed for %s: %w", host, err)
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("ingest: blocked request to private/internal address %s (%s)", host, ipStr)
		}
	}

	return nil
}

func slugifyURL(rawURL string) string {
	// Remove protocol
	s := strings.TrimPrefix(rawURL, "https://")
	s = strings.TrimPrefix(s, "http://")

	var result strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
		} else {
			result.WriteRune('-')
		}
	}

	slug := result.String()
	// Clean up multiple dashes
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	if len(slug) > 80 {
		slug = slug[:80]
	}
	return slug
}
