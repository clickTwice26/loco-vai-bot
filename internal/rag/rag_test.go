package rag

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
)

type mockEmbedder struct{}

func (m *mockEmbedder) EmbedText(ctx context.Context, text string) ([]float32, error) {
	// Simple deterministic embedding simulation based on word length
	vec := make([]float32, 4)
	if strings.Contains(strings.ToLower(text), "shagato") {
		vec[0] = 1.0
	}
	if strings.Contains(strings.ToLower(text), "partner") {
		vec[1] = 1.0
	}
	if strings.Contains(strings.ToLower(text), "auth") {
		vec[2] = 1.0
	}
	vec[3] = 0.5
	return vec, nil
}

func (m *mockEmbedder) IsConfigured() bool {
	return true
}

func TestRAGEngine(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	embedder := &mockEmbedder{}
	eng := NewEngine(context.Background(), embedder, "", logger)

	ctx := context.Background()

	// 1. Add Documents
	doc1 := Document{
		ID:       "team-shagato",
		Title:    "Shagato",
		Category: "team",
		Content:  "Shagato is the Lead Engineer and platform architect at Localoy.",
	}
	doc2 := Document{
		ID:       "sys-partner",
		Title:    "Partner Backend",
		Category: "systems",
		Content:  "The Partner Backend runs on port 4000 and hydrates cards.",
	}

	if err := eng.AddDocument(ctx, doc1); err != nil {
		t.Fatalf("failed to add doc1: %v", err)
	}
	if err := eng.AddDocument(ctx, doc2); err != nil {
		t.Fatalf("failed to add doc2: %v", err)
	}

	// 2. List Documents
	docs := eng.ListDocuments()
	if len(docs) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(docs))
	}

	// 3. Search
	results, err := eng.Search(ctx, "Tell me about Shagato", 2, 0.40)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected search results for Shagato, got 0")
	}
	if results[0].Document.ID != "team-shagato" {
		t.Errorf("expected top result to be team-shagato, got %s", results[0].Document.ID)
	}

	// 4. RetrieveContext
	ragCtx, err := eng.RetrieveContext(ctx, "Who works on Partner Backend?", 2)
	if err != nil {
		t.Fatalf("context retrieval failed: %v", err)
	}
	if !strings.Contains(ragCtx, "Partner Backend") {
		t.Errorf("expected context to mention Partner Backend, got: %s", ragCtx)
	}

	// 5. Delete Document
	if err := eng.DeleteDocument(ctx, "team-shagato"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if len(eng.ListDocuments()) != 1 {
		t.Errorf("expected 1 document after delete")
	}
}
