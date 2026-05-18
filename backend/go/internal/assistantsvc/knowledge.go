// Package assistantsvc — RAG knowledge base for documentation search.
//
// M4.5: Simple keyword-based search over docs/. Future: vector embeddings + Milvus.
package assistantsvc

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// DocEntry is a single indexed document.
type DocEntry struct {
	Title   string
	Path    string
	Content string
}

// KnowledgeBase indexes docs/ for fast retrieval.
type KnowledgeBase struct {
	mu    sync.RWMutex
	docs  []DocEntry
	ready bool
}

// NewKnowledgeBase creates an empty knowledge base.
func NewKnowledgeBase() *KnowledgeBase {
	return &KnowledgeBase{}
}

// Load scans the docs/ directory and indexes all markdown files.
func (kb *KnowledgeBase) Load(docsRoot string) error {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	kb.docs = nil
	err := filepath.Walk(docsRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		// Extract first heading as title
		content := string(data)
		title := info.Name()
		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(line, "# ") {
				title = strings.TrimPrefix(line, "# ")
				break
			}
		}
		kb.docs = append(kb.docs, DocEntry{
			Title:   title,
			Path:    path,
			Content: content,
		})
		return nil
	})
	if err == nil {
		kb.ready = true
	}
	return err
}

// Search finds documents matching the query (simple keyword match).
// Returns up to 5 most relevant results.
func (kb *KnowledgeBase) Search(query string) []DocEntry {
	kb.mu.RLock()
	defer kb.mu.RUnlock()

	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return nil
	}

	type scored struct {
		entry DocEntry
		score int
	}
	var results []scored

	for _, doc := range kb.docs {
		lower := strings.ToLower(doc.Content)
		score := 0
		for _, t := range terms {
			score += strings.Count(lower, t)
		}
		// Bonus for title match
		for _, t := range terms {
			score += strings.Count(strings.ToLower(doc.Title), t) * 3
		}
		if score > 0 {
			results = append(results, scored{entry: doc, score: score})
		}
	}

	// Sort by score descending (simple bubble for small N)
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	limit := 5
	if len(results) < limit {
		limit = len(results)
	}
	out := make([]DocEntry, limit)
	for i := 0; i < limit; i++ {
		out[i] = results[i].entry
	}
	return out
}

// Status returns whether the knowledge base is loaded.
func (kb *KnowledgeBase) Status() string {
	kb.mu.RLock()
	defer kb.mu.RUnlock()
	if kb.ready {
		return "ready"
	}
	return "not_loaded"
}
