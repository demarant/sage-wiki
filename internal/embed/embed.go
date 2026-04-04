package embed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/xoai/sage-wiki/internal/log"
)

// Embedder generates vector embeddings from text.
type Embedder interface {
	Embed(text string) ([]float32, error)
	Dimensions() int
	Name() string
}

// Default embedding models per provider.
var defaultModels = map[string]string{
	"openai":  "text-embedding-3-small",
	"gemini":  "text-embedding-004",
	"voyage":  "voyage-3-lite",
	"mistral": "mistral-embed",
}

// Default dimensions per model.
var defaultDimensions = map[string]int{
	"text-embedding-3-small": 1536,
	"text-embedding-004":     768,
	"voyage-3-lite":          1024,
	"mistral-embed":          1024,
	"nomic-embed-text":       768,
}

// NewCascade auto-detects the best available embedding provider.
// Tier 1: Provider embedding API (if available).
// Tier 2: Ollama local (if running).
// Returns nil if no embedding provider is available.
func NewCascade(provider string, apiKey string, baseURL string) Embedder {
	// Tier 1: Provider embedding API
	if model, ok := defaultModels[provider]; ok && apiKey != "" {
		dims := defaultDimensions[model]
		embedder := &APIEmbedder{
			provider: provider,
			model:    model,
			apiKey:   apiKey,
			baseURL:  baseURL,
			dims:     dims,
		}
		log.Info("embedding provider detected", "tier", 1, "provider", provider, "model", model, "dims", dims)
		return embedder
	}

	// Tier 2: Ollama local
	if ollamaAvailable() {
		log.Info("embedding provider detected", "tier", 2, "provider", "ollama", "model", "nomic-embed-text", "dims", 768)
		return &OllamaEmbedder{
			model: "nomic-embed-text",
			dims:  768,
		}
	}

	log.Warn("no embedding provider available — vector search disabled. Install Ollama or configure an embedding-capable provider.")
	return nil
}

// APIEmbedder calls a provider's embedding API.
type APIEmbedder struct {
	provider string
	model    string
	apiKey   string
	baseURL  string
	dims     int
	client   http.Client
}

func (e *APIEmbedder) Name() string     { return fmt.Sprintf("%s/%s", e.provider, e.model) }
func (e *APIEmbedder) Dimensions() int  { return e.dims }

func (e *APIEmbedder) Embed(text string) ([]float32, error) {
	url := e.embeddingURL()

	body, _ := json.Marshal(map[string]any{
		"model": e.model,
		"input": text,
	})

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embed: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	e.setAuthHeader(req)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embed: API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("embed: decode response: %w", err)
	}

	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("embed: empty embedding in response")
	}

	return result.Data[0].Embedding, nil
}

func (e *APIEmbedder) embeddingURL() string {
	base := e.baseURL
	if base == "" {
		switch e.provider {
		case "openai":
			base = "https://api.openai.com/v1"
		case "gemini":
			base = "https://generativelanguage.googleapis.com/v1beta"
		case "voyage":
			base = "https://api.voyageai.com/v1"
		case "mistral":
			base = "https://api.mistral.ai/v1"
		}
	}
	return base + "/embeddings"
}

func (e *APIEmbedder) setAuthHeader(req *http.Request) {
	switch e.provider {
	case "gemini":
		// Gemini uses query param
		q := req.URL.Query()
		q.Set("key", e.apiKey)
		req.URL.RawQuery = q.Encode()
	default:
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}
}

// OllamaEmbedder uses a local Ollama instance.
type OllamaEmbedder struct {
	model  string
	dims   int
	client http.Client
}

func (e *OllamaEmbedder) Name() string     { return fmt.Sprintf("ollama/%s", e.model) }
func (e *OllamaEmbedder) Dimensions() int  { return e.dims }

func (e *OllamaEmbedder) Embed(text string) ([]float32, error) {
	body, _ := json.Marshal(map[string]any{
		"model":  e.model,
		"prompt": text,
	})

	resp, err := e.client.Post("http://localhost:11434/api/embeddings", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama embed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama embed: %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ollama embed: decode: %w", err)
	}

	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("ollama embed: empty embedding")
	}

	e.dims = len(result.Embedding)
	return result.Embedding, nil
}

// ollamaAvailable probes localhost:11434 for a running Ollama instance.
func ollamaAvailable() bool {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
