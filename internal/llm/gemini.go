package llm

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// geminiProvider implements the Google Gemini API format.
type geminiProvider struct {
	apiKey  string
	baseURL string
}

func newGeminiProvider(apiKey string, baseURL string) *geminiProvider {
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	return &geminiProvider{apiKey: apiKey, baseURL: baseURL}
}

func (p *geminiProvider) Name() string        { return "gemini" }
func (p *geminiProvider) SupportsVision() bool { return true }

func (p *geminiProvider) FormatRequest(messages []Message, opts CallOpts) (*http.Request, error) {
	// Convert messages to Gemini format
	var contents []map[string]any
	var systemInstruction string

	for _, m := range messages {
		if m.Role == "system" {
			systemInstruction = m.Content
			continue
		}

		role := m.Role
		if role == "assistant" {
			role = "model"
		}

		contents = append(contents, map[string]any{
			"role": role,
			"parts": []map[string]string{
				{"text": m.Content},
			},
		})
	}

	body := map[string]any{
		"contents": contents,
	}

	if systemInstruction != "" {
		body["systemInstruction"] = map[string]any{
			"parts": []map[string]string{
				{"text": systemInstruction},
			},
		}
	}

	config := map[string]any{}
	if opts.MaxTokens > 0 {
		config["maxOutputTokens"] = opts.MaxTokens
	}
	if opts.Temperature > 0 {
		config["temperature"] = opts.Temperature
	}
	if len(config) > 0 {
		body["generationConfig"] = config
	}

	model := opts.Model
	if model == "" {
		model = "gemini-2.0-flash"
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", p.baseURL, model, p.apiKey)

	req, err := http.NewRequest("POST", url, jsonBody(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

func (p *geminiProvider) ParseResponse(body []byte) (*Response, error) {
	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		PromptFeedback struct {
			BlockReason string `json:"blockReason"`
		} `json:"promptFeedback"`
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
		UsageMetadata struct {
			TotalTokenCount int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
		ModelVersion string `json:"modelVersion"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("gemini: parse: %w", err)
	}

	// Check for API error
	if result.Error.Message != "" {
		return nil, fmt.Errorf("gemini: API error %d (%s): %s", result.Error.Code, result.Error.Status, result.Error.Message)
	}

	// Check for safety block
	if result.PromptFeedback.BlockReason != "" {
		return nil, fmt.Errorf("gemini: blocked by safety filter: %s", result.PromptFeedback.BlockReason)
	}

	if len(result.Candidates) == 0 {
		raw := string(body)
		if len(raw) > 300 {
			raw = raw[:300] + "..."
		}
		return nil, fmt.Errorf("gemini: no candidates in response. Raw: %s", raw)
	}

	// Handle MAX_TOKENS with empty content (thinking models use tokens internally)
	if len(result.Candidates[0].Content.Parts) == 0 {
		if result.Candidates[0].FinishReason == "MAX_TOKENS" {
			return &Response{
				Content:    "",
				Model:      result.ModelVersion,
				TokensUsed: result.UsageMetadata.TotalTokenCount,
			}, nil
		}
		raw := string(body)
		if len(raw) > 300 {
			raw = raw[:300] + "..."
		}
		return nil, fmt.Errorf("gemini: empty content (finish: %s). Raw: %s", result.Candidates[0].FinishReason, raw)
	}

	var text string
	for _, part := range result.Candidates[0].Content.Parts {
		text += part.Text
	}

	return &Response{
		Content:    text,
		Model:      result.ModelVersion,
		TokensUsed: result.UsageMetadata.TotalTokenCount,
	}, nil
}
