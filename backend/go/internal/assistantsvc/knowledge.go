// Package assistantsvc — RAG knowledge base with pgvector embeddings (R10).
//
// Upgrades the M4.5 keyword-based search to vector similarity search using
// pgvector's cosine distance. Documents are chunked, embedded via OpenAI, and
// indexed in the docs_embeddings table.
package assistantsvc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/alfq/backend/go/internal/common/db/pg"
)

// DocEntry is a single indexed document.
type DocEntry struct {
	Title   string
	Path    string
	Content string
}

// KnowledgeBase provides keyword fallback + pgvector RAG search.
type KnowledgeBase struct {
	mu     sync.RWMutex
	docs   []DocEntry
	ready  bool
	pg     *pg.Pool
	router *Router // for embeddings
}

// NewKnowledgeBase creates an empty knowledge base.
func NewKnowledgeBase() *KnowledgeBase {
	return &KnowledgeBase{}
}

// WithPG enables pgvector-based RAG search.
func (kb *KnowledgeBase) WithPG(p *pg.Pool) *KnowledgeBase {
	kb.pg = p
	return kb
}

// WithRouter sets the LLM router for generating embeddings.
func (kb *KnowledgeBase) WithRouter(r *Router) *KnowledgeBase {
	kb.router = r
	return kb
}

// Load scans the docs/ directory and indexes all markdown files.
// If pgvector is available, also indexes embeddings.
func (kb *KnowledgeBase) Load(docsRoot string) error {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	kb.docs = nil
	filepath.Walk(docsRoot, func(path string, info os.FileInfo, err error) error {
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
	kb.ready = true
	return nil
}

// IndexEmbeddings chunks all loaded docs and inserts embeddings into pgvector.
// Call this once after Load, or on document updates.
func (kb *KnowledgeBase) IndexEmbeddings(ctx context.Context) error {
	if kb.pg == nil || kb.router == nil {
		return fmt.Errorf("knowledge: pg or router not configured")
	}

	kb.mu.RLock()
	docs := make([]DocEntry, len(kb.docs))
	copy(docs, kb.docs)
	kb.mu.RUnlock()

	for _, doc := range docs {
		chunks := chunkText(doc.Content, 500, 50)
		for i, chunk := range chunks {
			// Check if already indexed
			var exists bool
			err := kb.pg.QueryRow(ctx,
				`SELECT EXISTS(SELECT 1 FROM docs_embeddings WHERE source_file=$1 AND chunk_idx=$2)`,
				doc.Path, i).Scan(&exists)
			if err == nil && exists {
				continue
			}

			// Generate embedding
			vec, err := kb.router.Embed(ctx, chunk)
			if err != nil {
				return fmt.Errorf("knowledge: embed %s chunk %d: %w", doc.Path, i, err)
			}

			// Convert to pgvector format: [0.1, 0.2, ...]
			vecStr := vectorToString(vec)
			_, err = kb.pg.Exec(ctx,
				`INSERT INTO docs_embeddings (chunk, embedding, source_file, chunk_idx)
				 VALUES ($1, $2::vector, $3, $4)
				 ON CONFLICT DO NOTHING`,
				chunk, vecStr, doc.Path, i)
			if err != nil {
				return fmt.Errorf("knowledge: insert embedding: %w", err)
			}
		}
	}
	return nil
}

// Search performs RAG retrieval: keyword fallback if pgvector unavailable,
// otherwise cosine similarity search via pgvector.
func (kb *KnowledgeBase) Search(ctx context.Context, query string) []DocEntry {
	// Try pgvector search first
	if kb.pg != nil && kb.router != nil {
		results, err := kb.vectorSearch(ctx, query)
		if err == nil && len(results) > 0 {
			return results
		}
	}

	// Fallback to keyword search
	return kb.keywordSearch(query)
}

// vectorSearch uses pgvector cosine similarity to find relevant chunks.
func (kb *KnowledgeBase) vectorSearch(ctx context.Context, query string) ([]DocEntry, error) {
	vec, err := kb.router.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	vecStr := vectorToString(vec)

	rows, err := kb.pg.Query(ctx,
		`SELECT chunk, source_file FROM docs_embeddings
		 ORDER BY embedding <=> $1::vector
		 LIMIT 3`, vecStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DocEntry
	for rows.Next() {
		var chunk, source string
		if err := rows.Scan(&chunk, &source); err != nil {
			continue
		}
		results = append(results, DocEntry{
			Title:   filepath.Base(source),
			Path:    source,
			Content: chunk,
		})
	}
	return results, rows.Err()
}

// keywordSearch is the fallback keyword-based search.
func (kb *KnowledgeBase) keywordSearch(query string) []DocEntry {
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
		for _, t := range terms {
			score += strings.Count(strings.ToLower(doc.Title), t) * 3
		}
		if score > 0 {
			results = append(results, scored{entry: doc, score: score})
		}
	}

	// Sort descending
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	limit := 3
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

// chunkText splits text into overlapping chunks of maxChars with overlap.
func chunkText(text string, maxChars, overlap int) []string {
	if maxChars <= 0 {
		return []string{text}
	}
	var chunks []string
	runes := []rune(text)
	step := maxChars - overlap
	if step <= 0 {
		step = maxChars
	}
	for i := 0; i < len(runes); i += step {
		end := i + maxChars
		if end > len(runes) {
			end = len(runes)
		}
		chunk := string(runes[i:end])
		if utf8.ValidString(chunk) {
			chunks = append(chunks, chunk)
		}
	}
	return chunks
}

// vectorToString converts a []float32 embedding to pgvector literal format.
func vectorToString(vec []float32) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range vec {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf("%.8f", v))
	}
	sb.WriteByte(']')
	return sb.String()
}
