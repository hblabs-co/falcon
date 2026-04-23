# Falcon 🦅

Created in Hamburg with ❤️, April 2026 - Visit https://falcon.hblabs.co

<p align="center">
  <img src="assets/ios-preview/en/p05.png" width="22%" alt="Falcon iOS screenshot 1" />
  <img src="assets/ios-preview/en/p06.png" width="22%" alt="Falcon iOS screenshot 2" />
  <img src="assets/ios-preview/en/p07.png" width="22%" alt="Falcon iOS screenshot 3" />
  <img src="assets/ios-preview/en/p08.png" width="22%" alt="Falcon iOS screenshot 4" />
</p>

<p align="center">
  <a href="https://apps.apple.com/app/falcon-f%C3%BCr-freelancer/id6763169883">
    <img src="https://tools.applemediaservices.com/api/badges/download-on-the-app-store/black/en-us" height="56" alt="Download on the App Store" />
  </a>
</p>

![Architecture](docs/falcon-architecture.png)

## Microservices

**falcon-auth** — Handles authentication and authorization across the entire platform. Issues and validates JWT tokens, manages user registration and login, and acts as the identity provider for all other services.

**falcon-cv-ingest** — Accepts CV uploads in Word format from users. Extracts raw text from the documents, generates vector embeddings via an embeddings API, stores the binary file in MinIO, saves metadata in MongoDB, and stores the vector in Qdrant. Publishes a `cv.indexed` event to NATS when processing is complete.

**falcon-scout** — Continuously scrapes freelance portals (freelance.de, gulp.de, malt.de, etc.) looking for new projects. Stores project data and metadata in MongoDB and publishes a `project.created` event to NATS for every new project detected, or `project.updated` when an existing project changes.

**falcon-dispatch** — Consumes `project.created` and `project.updated` events from NATS. Performs a fast vector similarity search in Qdrant to find users whose CVs are semantically close to the new project description. For each user above the similarity threshold, publishes a `match.pending` message to the match queue in NATS.

**falcon-match-engine** — Consumes `match.pending` messages from NATS. Fetches the full CV text and project description from MongoDB, calls the LLM to produce a detailed match score across six dimensions, and publishes a `match.result` event if the score exceeds the configured threshold.

> ⚠️ **This is the only service that needs horizontal scaling.** Each LLM call takes several seconds. A single project can generate up to N `match.pending` messages (one per candidate). Add replicas of this service to process them in parallel — all pods share the same NATS durable consumer so each message is processed exactly once.

**falcon-signal** — Consumes `match.result` events from NATS and delivers real-time notifications to the matched user via their preferred channel — email, Telegram bot, push notification, or webhook.

## Infrastructure

**MongoDB** — NoSQL document database used as the primary data store across services. Stores users metadata (ingested from CVs), raw project data scraped by falcon-scout, and match results. Chosen for its flexible schema, which accommodates evolving document structures without migrations.

**Qdrant** — Vector database purpose-built for high-performance similarity search. Stores the embeddings generated from CV text and project descriptions. falcon-dispatch queries Qdrant to find semantically similar CV/project pairs in milliseconds, even at large scale. Supports filtering and payload storage alongside vectors.

**NATS JetStream** — Distributed messaging system with persistent, at-least-once delivery guarantees (JetStream layer on top of core NATS). Used as the event bus between all services: `cv.indexed`, `project.created`, `project.updated`, `match.pending`, and `match.result` events flow through it. JetStream provides durable subscriptions and replay, so no events are lost if a consumer is temporarily down.

**Ollama** — Local inference server that runs embedding models on-premise. falcon-cv-ingest uses it to generate vector embeddings from CV text via an OpenAI-compatible API (`/v1/embeddings`). Running embeddings locally means user CV data never leaves the infrastructure, which is a hard requirement under GDPR. The model in use is `bge-m3`, chosen for its strong multilingual performance — relevant because CVs and project descriptions on the platform are predominantly in German.

**MinIO** — S3-compatible object storage deployed on-premises. Stores the original CV binary files (Word documents) uploaded by users. Services access files through the standard S3 API, making it straightforward to swap for AWS S3 or GCS in production without code changes.

## LLM strategy and GDPR

Falcon uses two AI models, with different roles and different GDPR implications:

| Model | Used by | Purpose |
|-------|---------|---------|
| `bge-m3` | `falcon-cv-ingest`, `falcon-dispatch` | Text → vector embeddings |
| `qwen2.5:7b` / `mistral-small-latest` | `falcon-match-engine` | CV/project scoring |

CV and project text are **personal data** under GDPR. Two compliant options:

- **Ollama (dev / on-premise)** — both models run locally via Ollama. Zero data egress. Recommended for development and for deployments where data must stay on-premise.
- **Mistral AI (production)** — French company, EU servers, GDPR-compliant by design. Switch by changing `LLM_URL` + `LLM_MODEL` env vars in `falcon-match-engine`. No code changes needed.

> ⚠️ Do not point `falcon-match-engine` at OpenAI, Anthropic, or any non-EU provider without a DPA review.

## Running Ollama natively on Apple Silicon

Docker on macOS runs inside a Linux VM with no access to the Metal GPU, forcing
CPU-only inference (~23s per embedding). Running Ollama natively uses Metal and
brings that down to under 1s.

```bash
# Install
brew install ollama

# Start the server (runs on http://localhost:11434)
ollama serve

# Pull both models
ollama pull bge-m3        # embeddings — used by cv-ingest and dispatch
ollama pull qwen2.5:7b    # LLM scoring — used by match-engine
```

Ollama will start automatically on login after installation. The rest of the
stack (`docker compose up`) connects to it at `http://host.docker.internal:11434`
— make sure `EMBEDDINGS_URL` in your `.env` points there.

> On Linux with an NVIDIA GPU, use the Docker service in `docker-compose.yml`
> (see the commented block) and add the `nvidia` runtime instead.

## Production Infrastructure (k3s / k8s)

![Infrastructure](docs/k3s-infrastructure.png)

All Falcon microservices and stateful components (MongoDB, Qdrant, NATS, MinIO)
run as standard Kubernetes workloads and can be deployed to any k3s or k8s cluster.

### Ollama on Apple Silicon

Ollama cannot access the Metal GPU from inside a container (no Metal passthrough
exists in any virtualisation layer on macOS). The solution is to run Ollama
**natively on the host** and expose it to the cluster over HTTP.

Recommended setup with a Mac Mini (M-series) as a dedicated inference node:

1. Install and start Ollama natively on the Mac Mini:
   ```bash
   brew install ollama
   ollama pull bge-m3
   brew services start ollama   # starts on boot
   ```
2. Add the Mac Mini as a k3s agent node normally.
3. Create a Kubernetes `Service` + `Endpoints` pointing to `http://<mac-mini-ip>:11434`
   so pods resolve Ollama via a stable in-cluster DNS name.
4. Set `EMBEDDINGS_URL=http://ollama.default.svc.cluster.local/v1/embeddings`
   in the `falcon-cv-ingest` deployment.

This gives full Metal GPU acceleration (<1s per embedding) with zero changes to
the application code. See `docs/k3s-infrastructure.puml` for the full topology.

## Local UIs

| Service | URL | Credentials |
|---------|-----|-------------|
| MinIO console | http://localhost:9001 | `minioadmin` / `minioadmin` |
| Qdrant dashboard | http://localhost:6333/dashboard | — |
| NATS monitoring | http://localhost:8222 | — |