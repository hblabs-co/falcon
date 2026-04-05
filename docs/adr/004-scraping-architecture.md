# ADR-004 — Scraping Architecture

**Date:** 2026-04-05
**Status:** Decided
**Services:** falcon-scrape-api (new), falcon-scout

---

## Context

Falcon needs to trigger scraping of freelance platforms on demand — for example when
a user submits a stale URL or when an admin wants to refresh a specific platform.

Two design questions:
1. How is scraping triggered?
2. How does `falcon-scout` scale across multiple platforms without becoming a monolith?

---

## Decision 1 — Trigger via a dedicated API service

A new service `falcon-scrape-api` exposes a lightweight HTTP API. It receives scrape
requests and publishes a `scrape.requested.{platform}` event to NATS. It has no
scraping logic of its own.

This separation keeps `falcon-scout` as a pure NATS consumer with no HTTP surface,
and keeps `falcon-cv-ingest` focused on CV processing only.

```
POST /scrape { platform: "freelance-de", url: "https://..." }
    → falcon-scrape-api
        → NATS: scrape.requested.freelance-de { url, platform, requested_at }
```

---

## Decision 2 — Single codebase, platform configured via env var

`falcon-scout` is one Go service. The active platform is selected at deploy time via
`PLATFORM=freelance-de` (or `gulp-de`, `freelancermap-de`, etc.). The codebase contains
all platform implementations; the env var activates one.

**Why not one service per platform?**
- One repo per platform = duplicated CI/CD, Dockerfiles, go.mod, shared logic
- Platform implementations share 80% of code (HTTP client, NATS publishing, MongoDB storage)
- Adding a new platform = adding a package inside falcon-scout, not a new repository

**Why not a single multi-platform monolith?**
- A single instance scraping all platforms would violate per-platform rate limits
- One platform's failure or slowness would block others
- Platforms need independent scaling — freelance.de may need 5 replicas, gulp.de only 1

The single-codebase-multi-deployment pattern gives the best of both worlds.

---

## NATS stream design

```
Stream: SCRAPE
Subjects: scrape.requested.*    ← wildcard captures all platforms
```

Each platform deploys with its own durable consumer name derived from the platform:

| Platform | PLATFORM env var | NATS subject | Consumer name |
|----------|-----------------|--------------|---------------|
| freelance.de | `freelance-de` | `scrape.requested.freelance-de` | `scout-freelance-de` |
| gulp.de | `gulp-de` | `scrape.requested.gulp-de` | `scout-gulp-de` |
| freelancermap.de | `freelancermap-de` | `scrape.requested.freelancermap-de` | `scout-freelancermap-de` |

### Competing consumers within the same platform

Multiple replicas of the same platform share the same consumer name.
NATS JetStream delivers each message to exactly one replica — no coordination needed.

```
scrape.requested.freelance-de  →  [ scout-freelance-de replica 1 ]
                                   [ scout-freelance-de replica 2 ]  ← only one receives each message
                                   [ scout-freelance-de replica 3 ]
                                   [ scout-freelance-de replica 4 ]
                                   [ scout-freelance-de replica 5 ]

scrape.requested.gulp-de       →  [ scout-gulp-de replica 1 ]       ← completely isolated
                                   [ scout-gulp-de replica 2 ]
```

### Platform isolation

gulp.de replicas never see freelance.de messages and vice versa.
A platform can be down, slow, or banned without affecting other platforms.

---

## Consequences

- `falcon-scrape-api` is a thin HTTP → NATS bridge. It requires no database.
- `falcon-scout` reads `PLATFORM` on startup and subscribes only to `scrape.requested.{PLATFORM}`.
- Consumer name is always `scout-{PLATFORM}` — derived automatically, no manual config.
- Adding a new platform = add a package in falcon-scout + deploy with `PLATFORM=new-platform`.
- The `SCRAPE` stream with `scrape.requested.*` subjects must be initialised before any scout starts.
- `scrape.requested.*` events should include enough context for the scout to act without additional API calls: `url`, `platform`, `requested_at`, and optionally `scrape_type` (full / incremental).
