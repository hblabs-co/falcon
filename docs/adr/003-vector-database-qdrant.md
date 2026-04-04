# ADR-003 — Vector Database: Qdrant

**Date:** 2026-04-05
**Status:** Decided
**Service:** falcon-cv-ingest, falcon-dispatch

---

## Context

Falcon needs a vector database to store CV embeddings and perform fast similarity
searches when a new project arrives. The database must:

- Store high-dimensional float vectors (1024 dims, bge-m3 output)
- Support cosine similarity search with a score threshold filter
- Store payload metadata alongside vectors (`cv_id`, `user_id`, `filename`)
- Run on-premise (GDPR — CV embeddings derived from personal data)
- Be deployable as a Docker container / k3s StatefulSet

---

## Options considered

### Qdrant
Open-source, written in Rust, purpose-built for vector similarity search.
REST and gRPC API. Supports filtering on payload fields, named vectors,
quantization, and on-disk storage. Docker image available. Active development.

### pgvector (PostgreSQL extension)
Adds vector similarity search to PostgreSQL. Good if the team already operates
Postgres. Slower than Qdrant at large scale (pure SQL query planner, no HNSW
optimisation by default). Would require running a separate Postgres instance or
sharing the existing one.

### Weaviate
Open-source, feature-rich, supports multi-modal and hybrid search. More complex
to operate than Qdrant. Larger resource footprint. Good for semantic + keyword
hybrid search, which Falcon does not need at this stage.

### Milvus
Enterprise-grade, horizontally scalable vector DB. Significantly more complex to
deploy (depends on etcd, MinIO internally). Overkill for the current scale.

### Chroma
Lightweight, Python-native. No production-grade persistence guarantees. Not
suitable for a Go service stack.

---

## Decision: Qdrant

**Reasons:**

1. **Purpose-built** — no impedance mismatch. Every feature exists for vector search.
2. **Performance** — Rust core with HNSW indexing. Sub-millisecond search at millions of vectors.
3. **Simple REST API** — no SDK required. `falcon-dispatch` and `falcon-cv-ingest` use plain HTTP via `ownhttp.Client`.
4. **Payload storage** — stores `cv_id`, `user_id`, `filename` alongside each vector. No join to MongoDB needed during search.
5. **Score threshold filtering** — native `score_threshold` parameter in the search API. Dispatch filters candidates in a single call.
6. **On-premise** — Docker image, k3s StatefulSet. Zero data egress.
7. **Point ID constraint** — Qdrant requires UUID or uint64 as point IDs (not arbitrary strings). Falcon uses `uuid.New().String()` as `qdrant_id`, stored separately from the nanoid CV ID in MongoDB.

**Trade-off accepted:** Qdrant is an additional stateful component to operate alongside MongoDB and NATS. This is justified by the fact that MongoDB does not support vector similarity search at the required performance level.

---

## Consequences

- Each CV generates one Qdrant point with a UUID ID and payload `{cv_id, user_id, filename}`.
- The `qdrant_id` is stored in MongoDB's `cvs` collection for cross-reference.
- Collection must be created before first use — `falcon-cv-ingest` calls `EnsureCollection` on startup.
- Vector dimension is fixed at 1024 (bge-m3). Changing the embedding model requires recreating the collection and re-indexing all CVs.
- Qdrant does not store the original text — only the vector. The full CV text lives in MongoDB (`extracted_text` field on `PersistedCV`).
