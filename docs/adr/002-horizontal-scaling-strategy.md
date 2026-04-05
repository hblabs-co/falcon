# ADR-002 — Horizontal Scaling Strategy

**Date:** 2026-04-05
**Status:** Decided
**Service:** All

---

## Context

Falcon is a pipeline of microservices connected via NATS JetStream. As the number
of CVs and projects grows, some services will experience higher load than others.
The question is: which services need horizontal scaling, and which do not?

---

## Analysis per service

### falcon-scout
`falcon-scout` is a single codebase configured via the `PLATFORM` environment variable
to scrape a specific platform (freelance.de, gulp.de, freelancermap.de, etc.).
Each platform deployment is independent — it consumes `scrape.requested.{platform}`
events from NATS and runs scraping jobs for that platform only.

Scaling works **per platform**: if freelance.de needs more throughput, add replicas
with `PLATFORM=freelance-de`. Those replicas share the NATS consumer `scout-freelance-de`
so each scrape request is processed by exactly one replica. gulp.de replicas are
completely isolated and do not interfere.

This is fundamentally different from running more replicas of a monolithic scout — each
platform's replica group competes only within its own message stream.

**Decision: scales horizontally per platform. See ADR-004 for the full scraping architecture.**

### falcon-cv-ingest
Handles HTTP uploads and async CV processing. Each CV is processed independently
but the volume of simultaneous uploads is low (users upload their own CV once).
Embedding via Ollama adds latency but it is per-user, not bulk.
**Decision: single replica sufficient. Scale only if HTTP concurrency becomes a bottleneck.**

### falcon-dispatch
Consumes `project.created` / `project.updated` events. One event per project change.
The total number of project events is low relative to match events. Each dispatch
call embeds a project description and queries Qdrant — fast operations (~2-3s total).
**Decision: single replica sufficient.**

### falcon-match-engine
Consumes `match.pending` events. A single new project can generate N events — one
per candidate above the Qdrant similarity threshold. Each event requires an LLM call
that takes 5–60s depending on the model and hardware. This is the only service where
throughput is directly proportional to the number of candidates in the system.

With 1000 CVs indexed and a new project, dispatch publishes up to 1000 `match.pending`
messages. A single match-engine pod processes them sequentially → total time = 1000 × 30s = ~8 hours.
With 10 pods → ~48 minutes. With 50 pods → ~10 minutes.

**Decision: scale horizontally. This is the only service that needs multiple replicas.**

### falcon-signal
Consumes `match.result` and sends notifications (email, Telegram, push). Notification
delivery is fast and I/O bound. Volume equals match results, which is a filtered
subset of match.pending (score threshold). No heavy computation.
**Decision: single replica sufficient.**

### falcon-auth
Stateless JWT validation. CPU-bound but trivial. Standard horizontal scaling applies
only if the service becomes an HTTP bottleneck — not expected at this scale.
**Decision: single replica sufficient.**

---

## Decision

**Two services require horizontal scaling:**

- `falcon-match-engine` — scales by adding replicas (all share one NATS consumer per stream)
- `falcon-scout` — scales per platform (each platform group shares one NATS consumer for that platform)

All other services run as single replicas.

## Implementation

NATS JetStream durable consumers handle competing consumers natively.
All `falcon-match-engine` replicas share the consumer name `match-engine`.
NATS delivers each `match.pending` message to exactly one pod — no coordination needed.

```yaml
# k3s deployment
replicas: 5  # adjust based on LLM throughput and acceptable latency
```

## Consequences

- No service mesh or coordination layer needed for scaling.
- Scaling `falcon-match-engine` is the single operational lever for pipeline throughput.
- When switching from Ollama (slow) to Mistral API (fast), fewer replicas are needed.
- Score threshold (`MATCH_ENGINE_SCORE_THRESHOLD`) indirectly controls load — a higher
  threshold means fewer `match.pending` events reach the LLM.
