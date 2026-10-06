package rag

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
)

const (
	geminiEmbeddingURL   = "https://generativelanguage.googleapis.com/v1beta/models/text-embedding-004:embedContent"
	defaultEmbeddingDims = 768
)

// Embedder generates dense vector embeddings for texts.
type Embedder interface {
	EmbedText(ctx context.Context, text string) ([]float32, error)
	IsConfigured() bool
}

type geminiEmbedder struct {
	apiKey     string
	httpClient *http.Client
}

// NewGeminiEmbedder creates a new Embedder powered by Gemini text-embedding-004.
func NewGeminiEmbedder(apiKey string) Embedder {
	return &geminiEmbedder{
		apiKey: strings.TrimSpace(apiKey),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (e *geminiEmbedder) IsConfigured() bool {
	return e.apiKey != ""
}

type geminiEmbedPart struct {
	Text string `json:"text"`
}

type geminiEmbedContent struct {
	Parts []geminiEmbedPart `json:"parts"`
}

type geminiEmbedRequest struct {
	Model   string             `json:"model"`
	Content geminiEmbedContent `json:"content"`
}

type geminiEmbedResponse struct {
	Embedding struct {
		Values []float32 `json:"values"`
	} `json:"embedding"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (e *geminiEmbedder) EmbedText(ctx context.Context, text string) ([]float32, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("cannot embed empty text")
	}

	if !e.IsConfigured() {
		return nil, errors.New("GEMINI_API_KEY is not configured for embeddings")
	}

	reqPayload := geminiEmbedRequest{
		Model: "models/text-embedding-004",
		Content: geminiEmbedContent{
			Parts: []geminiEmbedPart{
				{Text: text},
			},
		},
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode embedding request: %w", err)
	}

	endpoint := fmt.Sprintf("%s?key=%s", geminiEmbeddingURL, e.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read embedding response: %w", err)
	}

	var embedResp geminiEmbedResponse
	if err := json.Unmarshal(respBody, &embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode embedding response: %w", err)
	}

	if embedResp.Error != nil {
		return nil, fmt.Errorf("gemini embedding error (code %d): %s", embedResp.Error.Code, embedResp.Error.Message)
	}

	if len(embedResp.Embedding.Values) == 0 {
		return nil, errors.New("received empty embedding vector from Gemini")
	}

	return embedResp.Embedding.Values, nil
}
