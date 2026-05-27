package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ChatMessage represents standard OpenAI-compatible role-based chat payloads
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest defines standard model inputs sent to /chat/completions
type ChatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

// ChatCompletionResponse parses OpenAI-compliant output parameters and usage metadata
type ChatCompletionResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int32 `json:"prompt_tokens"`
		CompletionTokens int32 `json:"completion_tokens"`
	} `json:"usage"`
}

// OpenLLMClient manages interactions with standard OpenAI-compatible REST platforms
type OpenLLMClient struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

// NewOpenLLMClient builds a lightweight instance with customizable transport properties
func NewOpenLLMClient(baseURL, apiKey, defaultModel string, timeout time.Duration) *OpenLLMClient {
	if baseURL == "" {
		baseURL = "http://localhost:8080/v1"
	}
	if defaultModel == "" {
		defaultModel = "active-model"
	}
	return &OpenLLMClient{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   defaultModel,
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Generate handles REST calls with direct VPC path routing, OIDC/API key tokens, split-role prompt arrays, and custom durations
func (c *OpenLLMClient) Generate(ctx context.Context, systemPrompt, userPrompt, modelOverride string) (string, int32, int32, error) {
	targetModel := c.Model
	if modelOverride != "" {
		targetModel = modelOverride
	}

	messages := make([]ChatMessage, 0, 2)
	if systemPrompt != "" {
		messages = append(messages, ChatMessage{Role: "system", Content: systemPrompt})
	}
	if userPrompt != "" {
		messages = append(messages, ChatMessage{Role: "user", Content: userPrompt})
	}

	reqPayload := ChatCompletionRequest{
		Model:    targetModel,
		Messages: messages,
		Stream:   false,
	}

	reqBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to marshal request payload: %w", err)
	}

	url := c.BaseURL
	if strings.HasSuffix(url, "/") {
		url += "chat/completions"
	} else {
		url += "/chat/completions"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBytes))
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to execute Open-LLM request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, 0, fmt.Errorf("Open-LLM gateway returned non-OK status: %d, body: %s", resp.StatusCode, string(respBytes))
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return "", 0, 0, fmt.Errorf("failed to unmarshal Open-LLM response: %w, raw response: %s", err, string(respBytes))
	}

	if len(chatResp.Choices) == 0 {
		return "", 0, 0, fmt.Errorf("Open-LLM response returned no choices, raw response: %s", string(respBytes))
	}

	return chatResp.Choices[0].Message.Content, chatResp.Usage.PromptTokens, chatResp.Usage.CompletionTokens, nil
}
