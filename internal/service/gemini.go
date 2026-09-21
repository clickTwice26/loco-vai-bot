package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"localoy-bot/internal/memory"
)

const geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/models"

// GeminiClient represents a client for Google's Gemini API.
type GeminiClient interface {
	GenerateChatResponse(ctx context.Context, systemPrompt string, history []memory.Message, currentMessage memory.Message) (string, error)
}

type geminiClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewGeminiClient creates a new GeminiClient instance.
func NewGeminiClient(apiKey, model string) GeminiClient {
	if strings.TrimSpace(model) == "" {
		model = "gemini-1.5-flash"
	}

	return &geminiClient{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role"` // "user" or "model"
	Parts []geminiPart `json:"parts"`
}

type geminiSystemInstruction struct {
	Parts []geminiPart `json:"parts"`
}

type geminiGenerationConfig struct {
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
	TopP            float64 `json:"topP"`
}

type geminiRequest struct {
	SystemInstruction *geminiSystemInstruction `json:"system_instruction,omitempty"`
	Contents          []geminiContent          `json:"contents"`
	GenerationConfig  *geminiGenerationConfig  `json:"generationConfig,omitempty"`
}

type geminiCandidate struct {
	Content struct {
		Parts []geminiPart `json:"parts"`
	} `json:"content"`
	FinishReason string `json:"finishReason"`
}

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
	Error      *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

// GenerateChatResponse formats conversation history and queries the Gemini API.
func (g *geminiClient) GenerateChatResponse(
	ctx context.Context,
	systemPrompt string,
	history []memory.Message,
	currentMessage memory.Message,
) (string, error) {
	if strings.TrimSpace(g.apiKey) == "" {
		return "", errors.New("GEMINI_API_KEY is not configured")
	}

	// Prepare conversation contents
	contents := make([]geminiContent, 0, len(history)+1)

	for _, msg := range history {
		role := "user"
		text := fmt.Sprintf("[%s]: %s", msg.Author, msg.Content)
		if msg.Role == "model" {
			role = "model"
			text = msg.Content
		}

		contents = append(contents, geminiContent{
			Role: role,
			Parts: []geminiPart{
				{Text: text},
			},
		})
	}

	// Add current message
	contents = append(contents, geminiContent{
		Role: "user",
		Parts: []geminiPart{
			{Text: fmt.Sprintf("[%s]: %s", currentMessage.Author, currentMessage.Content)},
		},
	})

	reqPayload := geminiRequest{
		SystemInstruction: &geminiSystemInstruction{
			Parts: []geminiPart{
				{Text: systemPrompt},
			},
		},
		Contents: contents,
		GenerationConfig: &geminiGenerationConfig{
			Temperature:     0.90,
			MaxOutputTokens: 600,
			TopP:            0.95,
		},
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", fmt.Errorf("failed to encode gemini payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/%s:generateContent?key=%s", geminiBaseURL, g.model, g.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini api request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read gemini response body: %w", err)
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return "", fmt.Errorf("failed to decode gemini response: %w (status: %d)", err, resp.StatusCode)
	}

	if geminiResp.Error != nil {
		return "", fmt.Errorf("gemini api error (code %d): %s", geminiResp.Error.Code, geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("empty response generated from Gemini")
	}

	answer := strings.TrimSpace(geminiResp.Candidates[0].Content.Parts[0].Text)
	return answer, nil
}
