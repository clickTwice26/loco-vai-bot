package rag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LoadFromDirectory reads all markdown files from the specified directory and indexes their sections.
func (e *engine) LoadFromDirectory(ctx context.Context, dirPath string) (int, error) {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return 0, nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read knowledge directory: %w", err)
	}

	indexedCount := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		contentBytes, err := os.ReadFile(filePath)
		if err != nil {
			e.logger.Warn("failed to read knowledge file", "path", filePath, "error", err)
			continue
		}

		category := strings.TrimSuffix(strings.ToLower(entry.Name()), ".md")
		docs := parseMarkdownSections(entry.Name(), category, string(contentBytes))

		for _, doc := range docs {
			if err := e.AddDocument(ctx, doc); err == nil {
				indexedCount++
			}
		}
	}

	e.logger.Info("loaded knowledge base files from disk", "directory", dirPath, "documents_indexed", indexedCount)
	return indexedCount, nil
}

func parseMarkdownSections(fileName, category, text string) []Document {
	var docs []Document
	lines := strings.Split(text, "\n")

	var currentTitle string
	var currentLines []string

	flush := func() {
		content := strings.TrimSpace(strings.Join(currentLines, "\n"))
		if content != "" && currentTitle != "" {
			docID := fmt.Sprintf("%s-%s", category, sanitizeID(currentTitle))
			docs = append(docs, Document{
				ID:        docID,
				Title:     currentTitle,
				Category:  category,
				Content:   content,
				Source:    fileName,
				UpdatedAt: time.Now().UTC(),
			})
		}
		currentLines = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "# ") {
			flush()
			currentTitle = strings.TrimSpace(strings.TrimLeft(trimmed, "# "))
		} else {
			currentLines = append(currentLines, line)
		}
	}
	flush()

	// If no headings found, treat whole file as one doc
	if len(docs) == 0 && strings.TrimSpace(text) != "" {
		docs = append(docs, Document{
			ID:        category + "-main",
			Title:     strings.Title(category),
			Category:  category,
			Content:   strings.TrimSpace(text),
			Source:    fileName,
			UpdatedAt: time.Now().UTC(),
		})
	}

	return docs
}

func sanitizeID(s string) string {
	s = strings.ToLower(s)
	var sb strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '_' {
			sb.WriteRune('-')
		}
	}
	res := strings.Trim(sb.String(), "-")
	if res == "" {
		return "doc"
	}
	return res
}
