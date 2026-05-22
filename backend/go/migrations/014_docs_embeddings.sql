-- 014: RAG knowledge base with pgvector embeddings (R10)
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS docs_embeddings (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    chunk       text NOT NULL,
    embedding   vector(1536) NOT NULL,
    source_file text NOT NULL,
    chunk_idx   int NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_docs_embedding ON docs_embeddings USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
