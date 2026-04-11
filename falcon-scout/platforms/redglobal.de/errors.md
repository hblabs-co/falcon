# Possible Errors — redglobal.de

Inventory of failure modes that can occur when scraping `redglobal.de`. Levels: **critical** (stops the platform), **high** (data loss for one item), **medium** (partial data), **low** (cosmetic / recoverable).

## Network & HTTP

| # | Error | Source | Cause | Level | Impact |
|---|---|---|---|---|---|
| 1 | DNS resolution failure | `scraper.go` `c.Visit` (scrapePage / Inspect) | redglobal.de unreachable, DNS down, network partition | critical | All polling fails until network recovers; no candidates collected |
| 2 | TCP / TLS handshake failure | `scraper.go` `c.Visit` | Cert expired, server down, firewall block | critical | Same as #1 |
| 3 | HTTP 4xx (403, 404) | `scrapePage` / `Inspect` `OnError` | Bot detection, IP banned, URL changed, page removed | high | Single page or detail returns no data; if 403 persists, full platform blackout |
| 4 | HTTP 5xx | `OnError` | Site outage, rate-limit response | high | Page skipped; logged but candidate lost on this poll cycle |
| 5 | Request timeout | colly default timeout | Slow server, network congestion | medium | Page or detail page lost; retry on next poll |
| 6 | IP rate-limit / soft block | `OnError` | Polling too aggressive | high | Cascading 4xx/5xx; full blackout possible |
| 7 | Captcha / JS challenge page | `scrapePage` | Cloudflare/Akamai bot detection serves HTML challenge | high | `c-card-job` selector returns nothing → page treated as empty → loop stops early |

## HTML Parsing — Listing Page (`scrapePage` / `parseJobCard`)

| # | Error | Source | Cause | Level | Impact |
|---|---|---|---|---|---|
| 8 | Site markup changed: `.c-card-job` selector no longer matches | `scrapePage` | Frontend redesign | critical | **Detected**: page 1 returning 0 candidates raises a `ErrEmptyListing` → categorical error `scrape_listing_empty` is recorded with the HTML snapshot. Deduped via deterministic ID so a markup change produces ONE record with `occurrence_count` instead of one-per-poll. |
| 9 | `.c-card-job__actions a` href empty | `parseJobCard` | Markup variation, A/B test | medium | Candidate silently skipped (returns nil) |
| 10 | href format changed (less than 5 path segments) | `parseJobHref` | URL scheme changed | high | `platformID == ""` → candidate skipped |
| 11 | Missing `.c-card-job__title` / `.c-list-specifications__item--*` | `parseJobCard` | Markup variation | low | Empty strings stored; data quality degraded |
| 12 | `.c-pagination__next` not found when more pages exist | `scrapePage` | Pagination markup changed | medium | `hasNext = false` → loop stops prematurely → newer postings missed |
| 13 | `.c-pagination__next` always present (infinite loop) | `scrapePage` / `scrapeLoop` | Pagination markup bug on site side | medium | Loop bounded only by empty page; could request many pages unnecessarily |
| 14 | Duplicate platformIDs across pages | `scrapeLoop` | Site re-orders results between page fetches | low | Same job processed twice; filter dedupes via existing-in-DB check |

## HTML Parsing — Detail Page (`Inspect`)

| # | Error | Source | Cause | Level | Impact |
|---|---|---|---|---|---|
| 15 | No `script[type="application/ld+json"]` on detail page | `Inspect` | Page changed, removed, or never had structured data | high | Returns `"no JobPosting ld+json found"` error; candidate lost for this cycle |
| 16 | JSON-LD malformed / `Unmarshal` fails | `Inspect` `OnHTML` ld+json handler | Invalid JSON on page | high | Silently skipped (`return` inside callback); falls through to "no JobPosting found" error |
| 17 | JSON-LD `@type` is not `JobPosting` (e.g. `Organization`, `BreadcrumbList`) | `Inspect` | Page has multiple ld+json blocks | low | Correctly skipped; only first matching block kept |
| 18 | Multiple `JobPosting` ld+json blocks with different data | `Inspect` | Site bug | low | Only the first one is taken — may not be the canonical one |
| 19 | `validThrough` not RFC3339 | `jobPostingToProject` | Date format variation | low | Falls back to storing raw string; downstream date logic may break |
| 20 | `ld.URL` empty → no slug extracted | `jobPostingToProject` | Missing field in JSON-LD | low | `Slug` empty in stored Project |
| 21 | `jobLocation.address.addressLocality` missing | `jobPostingToProject` | Remote-only postings | low | `Location` empty |

## Reference ID Extraction

| # | Error | Source | Cause | Level | Impact |
|---|---|---|---|---|---|
| 22 | Reference label markup restructured (e.g. `<strong>` → `<span>`, table, definition list) | `Inspect` `OnHTML "p:has(strong)"` selector | Major frontend redesign — `p:has(strong)` no longer matches the reference row | medium | `ReferenceID` empty — but **detected**: `WarnReferenceIDNotFound` warning persisted with HTML snapshot, queryable via `db.warnings.find({warning_name: "reference_id_not_found"})`. Language variants (Reference / Referenz / Ref) and value-extraction format quirks are already handled. |
| 23 | Multiple `<p><strong>...</strong></p>` matched but first one isn't Reference | `Inspect` | Markup variation | low | Handler iterates and only sets on a recognized reference label; works correctly even if other labeled rows precede it. |
| 24 | `referenceID` collides with another job's reference (rare) | DB layer | Site reuses references | medium | Filter by reference may match wrong project |

## Data Model & Persistence

| # | Error | Source | Cause | Level | Impact |
|---|---|---|---|---|---|
| 25 | `PlatformID` collision (same ID for different jobs) | `scraper.parseJobHref` / `models.go` | URL collision (extremely unlikely) | low | Existing record overwritten on save |
| 26 | `PostedAt` is empty or non-parseable | `parseJobCard` (listing) | Listing markup change | medium | Filter cannot detect "unchanged" → re-saves on every poll |
| 27 | `Project.GetId()` always returns `""` | `models.go` `Project.GetId` | Not implemented | medium | Service-side `NewPersistedProject` always treats as new; relies on `existingID` plumbing which is currently `""` |
| 28 | `Rate` hardcoded to "Negotiable" | `Inspect` (set after successful parse) | Intentional placeholder | low | Real rate (if any) not extracted |
| 29 | `IsRemote` false-negative | `Project.IsRemote` | Only checks `JobLocationType == TELECOMMUTE`; site may use different field | low | Job mis-flagged as on-site |

## Pagination & Loop Control

| # | Error | Source | Cause | Level | Impact |
|---|---|---|---|---|---|
| 30 | `scrapeLoop` exits early because filter marks all as known | `runner.go collectCandidates` | All candidates already in DB | low | Expected behavior, but means a single new posting buried in page 2 is missed if page 1 is fully known. **This is a real risk for redglobal's polling.** |
| 31 | `scrapeLoop` runs forever | `scrapeLoop` | `hasNext == true` and `len(filtered) > 0` for every page | high | Polling cycle hangs; may exhaust quota |
| 32 | Filter callback errors → handler returns `(nil, false)` | `runner.go collectCandidates` | DB query failure during filter | medium | Page's candidates lost; loop stops |

## Concurrency & State

| # | Error | Source | Cause | Level | Impact |
|---|---|---|---|---|---|
| 33 | `Scraper` reused across goroutines | `Scraper` struct | Shared `HTML` field, not goroutine-safe | medium | Data races if multiple inspectors run in parallel — currently `NewCandidateScraper` is per-call, so OK, but easy to break |
| 34 | `colly.Collector` reused across calls | `scrapePage` / `Inspect` | Currently a new collector is built each call (safe) | low | If refactored to reuse, callbacks accumulate and fire multiple times |
| 35 | Polling cycle overlaps with previous cycle | `runner.Poll` | Slow scrape + short interval | medium | Duplicate work, increased load on redglobal |

## Save / Filter Pipeline (handler-based)

| # | Error | Source | Cause | Level | Impact |
|---|---|---|---|---|---|
| 36 | `r.save == nil` (handler not injected) | `runner.process` | Service didn't call `SetSaveHandler` | high | Candidate scraped but never persisted; silently dropped after log |
| 37 | `r.filter == nil` (handler not injected) | `runner.collectCandidates` | Service didn't call `SetFilterHandler` | medium | No filtering → every candidate re-scraped on every poll cycle, hammering both DB and redglobal |
| 38 | `r.save` returns error | `runner.process` | DB write failed, NATS publish failed | high | Logged but candidate is lost for this cycle (no retry queue yet) |
| 39 | `result.Project` cast in service fails | `service.go newSaveFn` | `Project` doesn't satisfy `interfaces.Project` | critical | Every save fails; the `missingMethods` log helps diagnose |

## Configuration / Environment

| # | Error | Source | Cause | Level | Impact |
|---|---|---|---|---|---|
| 40 | `searchURL` query parameters become invalid | `constants.go` | Site changes filter param scheme (`types[0]=0`) | critical | Listing returns wrong category or empty results |
| 41 | `baseURL` hardcoded → no staging/test variant | `constants.go` | Hardcoded | low | Cannot point at a fixture server for tests |
| 42 | `colly.AllowedDomains` mismatch on redirect | `scraper.go` | Site redirects to a CDN or alternate domain | medium | `c.Visit` fails; whole page lost |

## Polling & Rate Limiting

| # | Error | Source | Cause | Level | Impact |
|---|---|---|---|---|---|
| 43 | No `robots.txt` compliance check | (missing) | Not implemented | medium | Legal/ethical risk; possible IP ban |
| 44 | No `If-Modified-Since` / `ETag` support | `scraper.go` | Not implemented | low | Re-downloads unchanged pages, wasting bandwidth |

## Observability

| # | Error | Source | Cause | Level | Impact |
|---|---|---|---|---|---|
| 45 | Errors swallowed in `OnHTML` callbacks (e.g. JSON unmarshal) | `Inspect` ld+json handler | `return` on error without logging | medium | Silent data loss; debugging requires re-running with breakpoints |

---

## Top priorities to fix

All previously top-tier failures (markup drift, UA + delays, RecordError TODO,
PostedAt vs DatePosted) are now mitigated. The remaining items are mostly low /
medium severity edge cases worth revisiting once the platform has more traffic.
