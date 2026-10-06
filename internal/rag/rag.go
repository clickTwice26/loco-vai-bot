package rag

import (
	"context"
	"time"
)

// Document represents an entry in the Localoy Knowledge Base.
type Document struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Category  string    `json:"category"` // "team", "systems", "product", "general"
	Content   string    `json:"content"`
	Embedding []float32 `json:"embedding,omitempty"`
	Source    string    `json:"source"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SearchResult pairs a matched document with its cosine similarity score.
type SearchResult struct {
	Document Document `json:"document"`
	Score    float64  `json:"score"`
}

// Engine defines the Knowledge Base & RAG service.
type Engine interface {
	AddDocument(ctx context.Context, doc Document) error
	Search(ctx context.Context, query string, topK int, minScore float64) ([]SearchResult, error)
	RetrieveContext(ctx context.Context, query string, topK int) (string, error)
	GetDocument(id string) (*Document, bool)
	ListDocuments() []Document
	DeleteDocument(ctx context.Context, id string) error
	LoadFromDirectory(ctx context.Context, dirPath string) (int, error)
}
