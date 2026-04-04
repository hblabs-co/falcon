# falcon-match-engine

Consumes `match.pending` events from NATS, scores each CV/project pair using an
LLM, and publishes a `match.result` event for every candidate above the score threshold.

> ⚠️ **This is the only service in the Falcon stack that needs horizontal scaling.**
> Each LLM call takes several seconds. A single new project can generate dozens of
> `match.pending` messages. Add replicas to process them in parallel — all pods share
> the same NATS durable consumer (`match-engine`) so each message is processed
> exactly once, never duplicated.

## Flow

1. **Consume** — receives a `match.pending` event with `cv_id`, `project_id`, `user_id`.
2. **Fetch** — loads `PersistedCV.extracted_text` and `PersistedProject.title + description`
   from MongoDB. No MinIO access, no re-extraction.
3. **Score** — calls the LLM via OpenAI-compatible `/v1/chat/completions` with a
   structured prompt that evaluates six dimensions.
4. **Filter** — discards results below `MATCH_ENGINE_SCORE_THRESHOLD` (default `6.0`).
5. **Publish** — emits `match.result` to NATS with the full scoring breakdown.

## Scoring dimensions

Each dimension is scored 0–10. The overall score is their average.

| Dimension | What it measures |
|-----------|-----------------|
| `skills_match` | How well the candidate's skills cover the project requirements |
| `seniority_fit` | Whether the experience level matches what the project expects |
| `domain_experience` | Prior work in the same industry or problem domain |
| `communication_clarity` | How clearly and professionally the CV is written |
| `project_relevance` | How similar past projects are in scope and type |
| `tech_stack_overlap` | Literal overlap in frameworks, languages, and tools |

## UI labels

| Label | Score range |
|-------|-------------|
| `apply_immediately` | ≥ 8.5 |
| `top_candidate` | ≥ 7.0 |
| `acceptable` | ≥ 5.0 |
| `not_suitable` | < 5.0 |

## LLM compatibility

Works with any OpenAI-compatible endpoint. Switch between local and cloud via env vars — no code changes needed.

| Environment | `LLM_URL` | `LLM_MODEL` |
|-------------|-----------|-------------|
| Local (dev) | `http://localhost:11434` | `qwen2.5:7b` |
| Production | `https://api.mistral.ai` | `mistral-small-latest` |

```bash
# Pull the local model before running in dev
ollama pull qwen2.5:7b
```

## Environment variables

| Variable | Required | Description |
|----------|----------|-------------|
| `MONGODB_URI` | ✅ | MongoDB connection string |
| `MONGODB_DATABASE` | ✅ | Database name |
| `NATS_URL` | ✅ | NATS JetStream URL |
| `LLM_URL` | ✅ | LLM base URL (Ollama or Mistral) |
| `LLM_API_KEY` | ✅ | API key (`ollama` for local) |
| `LLM_MODEL` | ✅ | Model name |
| `MATCH_ENGINE_SCORE_THRESHOLD` | — | Min score to publish match.result, default `6.0` |

## Running locally

```bash
docker compose up -d   # run at root dir
ollama serve           # natively on macOS

cp .env.example .env
go run .
```

## Scaling in production (k3s)

```yaml
replicas: 5   # each pod is an independent worker
```

All replicas consume from the same NATS durable consumer. NATS distributes
`match.pending` messages across them automatically — no coordination needed.
