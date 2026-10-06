package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const redisKBPrefix = "loco:kb:doc:"

type engine struct {
	embedder    Embedder
	redisClient *redis.Client
	logger      *slog.Logger
	mu          sync.RWMutex
	docs        map[string]Document
}

// NewEngine creates a new RAG Engine with in-memory vector search and optional Redis persistence.
func NewEngine(ctx context.Context, embedder Embedder, redisURL string, logger *slog.Logger) Engine {
	eng := &engine{
		embedder: embedder,
		logger:   logger.With("module", "rag-engine"),
		docs:     make(map[string]Document),
	}

	if strings.TrimSpace(redisURL) != "" {
		opts, err := redis.ParseURL(redisURL)
		if err == nil {
			client := redis.NewClient(opts)
			pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			if err := client.Ping(pingCtx).Err(); err == nil {
				eng.redisClient = client
				eng.logger.Info("RAG engine connected to Redis for persistent knowledge base")
				eng.loadFromRedis(ctx)
			} else {
				eng.logger.Warn("RAG could not reach Redis, operating in-memory", "error", err)
			}
			cancel()
		}
	}

	return eng
}

func (e *engine) loadFromRedis(ctx context.Context) {
	if e.redisClient == nil {
		return
	}

	keys, err := e.redisClient.Keys(ctx, redisKBPrefix+"*").Result()
	if err != nil {
		e.logger.Warn("failed to scan knowledge base keys from Redis", "error", err)
		return
	}

	count := 0
	for _, key := range keys {
		val, err := e.redisClient.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var doc Document
		if err := json.Unmarshal([]byte(val), &doc); err == nil && doc.ID != "" {
			e.docs[doc.ID] = doc
			count++
		}
	}

	e.logger.Info("loaded knowledge base documents from Redis", "count", count)
}

// AddDocument embeds the document content and stores it in memory and Redis.
func (e *engine) AddDocument(ctx context.Context, doc Document) error {
	doc.ID = strings.TrimSpace(doc.ID)
	doc.Title = strings.TrimSpace(doc.Title)
	doc.Content = strings.TrimSpace(doc.Content)
	if doc.Category == "" {
		doc.Category = "general"
	}

	if doc.ID == "" {
		doc.ID = fmt.Sprintf("doc-%d", time.Now().UnixNano())
	}
	if doc.UpdatedAt.IsZero() {
		doc.UpdatedAt = time.Now().UTC()
	}

	// Generate embedding if not already present
	if len(doc.Embedding) == 0 && e.embedder != nil && e.embedder.IsConfigured() {
		textToEmbed := fmt.Sprintf("%s\n%s", doc.Title, doc.Content)
		embedCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		emb, err := e.embedder.EmbedText(embedCtx, textToEmbed)
		cancel()

		if err != nil {
			e.logger.Warn("failed to generate embedding for doc, proceeding without vector", "doc_id", doc.ID, "error", err)
		} else {
			doc.Embedding = emb
		}
	}

	e.mu.Lock()
	e.docs[doc.ID] = doc
	e.mu.Unlock()

	// Persist to Redis
	if e.redisClient != nil {
		data, err := json.Marshal(doc)
		if err == nil {
			_ = e.redisClient.Set(ctx, redisKBPrefix+doc.ID, data, 0).Err()
		}
	}

	e.logger.Info("indexed knowledge base document",
		"id", doc.ID,
		"title", doc.Title,
		"category", doc.Category,
		"has_embedding", len(doc.Embedding) > 0,
	)

	return nil
}

// Search calculates semantic cosine similarity and returns top-K matching documents.
func (e *engine) Search(ctx context.Context, query string, topK int, minScore float64) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []SearchResult{}, nil
	}
	if topK <= 0 {
		topK = 3
	}
	if minScore <= 0 {
		minScore = 0.50
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.docs) == 0 {
		return []SearchResult{}, nil
	}

	// 1. If embedder is configured, perform dense semantic vector search
	if e.embedder != nil && e.embedder.IsConfigured() {
		embedCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		queryVec, err := e.embedder.EmbedText(embedCtx, query)
		cancel()

		if err == nil && len(queryVec) > 0 {
			var results []SearchResult
			for _, doc := range e.docs {
				if len(doc.Embedding) == 0 {
					continue
				}
				score := CosineSimilarity(queryVec, doc.Embedding)
				if score >= minScore {
					results = append(results, SearchResult{
						Document: doc,
						Score:    score,
					})
				}
			}

			// Sort by descending score
			sort.Slice(results, func(i, j int) bool {
				return results[i].Score > results[j].Score
			})

			if len(results) > topK {
				results = results[:topK]
			}
			return results, nil
		}
	}

	// 2. Fallback: Keyword search if embeddings are unconfigured or unavailable
	lowerQuery := strings.ToLower(query)
	words := strings.Fields(lowerQuery)
	var keywordResults []SearchResult

	for _, doc := range e.docs {
		fullText := strings.ToLower(fmt.Sprintf("%s %s %s", doc.Title, doc.Category, doc.Content))
		matches := 0
		for _, w := range words {
			if strings.Contains(fullText, w) {
				matches++
			}
		}
		if matches > 0 {
			score := float64(matches) / float64(len(words))
			if score >= 0.25 {
				keywordResults = append(keywordResults, SearchResult{
					Document: doc,
					Score:    score,
				})
			}
		}
	}

	sort.Slice(keywordResults, func(i, j int) bool {
		return keywordResults[i].Score > keywordResults[j].Score
	})

	if len(keywordResults) > topK {
		keywordResults = keywordResults[:topK]
	}

	return keywordResults, nil
}

// RetrieveContext formats retrieved documents into a clean context block for LLM prompt injection.
func (e *engine) RetrieveContext(ctx context.Context, query string, topK int) (string, error) {
	results, err := e.Search(ctx, query, topK, 0.55)
	if err != nil || len(results) == 0 {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("\n[VERIFIED LOCALOY KNOWLEDGE BASE CONTEXT]:\n")
	for idx, res := range results {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s:\n%s\n\n",
			idx+1,
			strings.ToUpper(res.Document.Category),
			res.Document.Title,
			res.Document.Content,
		))
	}

	return sb.String(), nil
}

// GetDocument returns a document by ID.
func (e *engine) GetDocument(id string) (*Document, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	doc, exists := e.docs[id]
	if !exists {
		return nil, false
	}
	return &doc, true
}

// ListDocuments returns all indexed documents.
func (e *engine) ListDocuments() []Document {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]Document, 0, len(e.docs))
	for _, doc := range e.docs {
		result = append(result, doc)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Title < result[j].Title
	})

	return result
}

// DeleteDocument removes a document from memory and Redis.
func (e *engine) DeleteDocument(ctx context.Context, id string) error {
	e.mu.Lock()
	delete(e.docs, id)
	e.mu.Unlock()

	if e.redisClient != nil {
		_ = e.redisClient.Del(ctx, redisKBPrefix+id).Err()
	}

	e.logger.Info("deleted knowledge base document", "id", id)
	return nil
}
