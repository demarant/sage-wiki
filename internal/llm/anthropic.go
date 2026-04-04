package llm

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// anthropicProvider implements the Anthropic Messages API format.
type anthropicProvider struct {
	apiKey  string
	baseURL string
}

func newAnthropicProvider(apiKey string, baseURL string) *anthropicProvider {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	return &anthropicProvider{apiKey: apiKey, baseURL: baseURL}
}

func (p *anthropicProvider) Name() string        { return "anthropic" }
func (p *anthropicProvider) SupportsVision() bool { return true }

func (p *anthropicProvider) FormatRequest(messages []Message, opts CallOpts) (*http.Request, error) {
	// Anthropic requires system message separate from messages
	var systemPrompt string
	var apiMessages []map[string]string

	for _, m := range messages {
		if m.Role == "system" {
			systemPrompt = m.Content
			continue
		}
		apiMessages = append(apiMessages, map[string]string{
			"role":    m.Role,
			"content": m.Content,
		})
	}

	maxTokens := opts.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096 // Anthropic requires max_tokens
	}

	body := map[string]any{
		"model":      opts.Model,
		"messages":   apiMessages,
		"max_tokens": maxTokens,
	}
	if systemPrompt != "" {
		body["system"] = systemPrompt
	}
	if opts.Temperature > 0 {
		body["temperature"] = opts.Temperature
	}

	req, err := http.NewRequest("POST", p.baseURL+"/v1/messages", jsonBody(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	return req, nil
}

func (p *anthropicProvider) ParseResponse(body []byte) (*Response, error) {
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Model string `json:"model"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("anthropic: parse: %w", err)
	}

	if len(result.Content) == 0 {
		return nil, fmt.Errorf("anthropic: empty content in response")
	}

	// Concatenate text blocks
	var text string
	for _, block := range result.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}

	return &Response{
		Content:    text,
		Model:      result.Model,
		TokensUsed: result.Usage.InputTokens + result.Usage.OutputTokens,
	}, nil
}
