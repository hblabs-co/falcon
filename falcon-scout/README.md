# falcon-scout

Multi-platform job scraper service. Each platform is a self-contained Go module that implements the `Platform` interface. The scout service orchestrates them: injects handlers (save, filter, warn, err), manages poll cycles, retries, metadata refresh, and admin alert publishing.

## Platforms

| Platform | Data source | Detail page | Pagination | Dependencies |
|---|---|---|---|---|
| `redglobal.de` | HTML scraping | Yes (JSON-LD + contact + reference) | Multi-page listing | colly, goquery |
| `contractor.de` | HTML scraping | Yes (contact only) | No (single page) | colly, goquery |
| `solcom.de` | RSS XML feed | No (all data in feed) | No (full feed) | stdlib only |
| `computerfutures.com` | JSON API (sthree.com) | No (all data in API) | Yes (resultFrom offset) | stdlib only |

## Architecture

```
scout/main.go          → registers platforms, starts the service
scout/service.go       → Platform interface, handler injection, poll loop
scout/retry.go         → background retry workers per platform
scout/metadata.go      → hourly metadata refresh (robots.txt, etc.)
scout/alerts.go        → admin alert publishing helper

platforms/<name>/
  constants.go         → source name, company id, urls, hostnames
  models.go            → ProjectCandidate, Project (implements interfaces.Project)
  runner.go            → Platform implementation (poll, process, retry)
  scraper.go / feed.go / api.go  → data fetching (colly, RSS, or JSON API)
  helper.go            → date parsing, location parsing, etc.
```

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `PLATFORMS` | `hblabs.co` | Comma-separated list of platforms to activate |
| `POLL_INTERVAL` | `30s` | How often the main poll runs |
| `BATCH_SIZE` | `10` | Items per batch before a longer pause |
| `BATCH_ITEM_DELAY` | `2s` | Pause between items within a batch |
| `BATCH_BATCH_DELAY` | `10s` | Pause between batches |
| `RETRY_INSPECT_INTERVAL` | `5m` | How often inspect failures are retried |
| `RETRY_SERVER_INTERVAL` | `30m` | How often server errors (5xx) are retried |
| `MAX_RETRY_ATTEMPTS` | `50` | Max retries before escalating to critical |
| `TEST_RETRY` | (unset) | If set, simulates a 500 on the first item for retry testing |

## Adding a new platform

1. Create `platforms/<name>/` with `go.mod`, `constants.go`, `models.go`, `runner.go`, and a data-fetching file
2. Implement all methods of the `Platform` interface (see `scout/service.go`)
3. `Project` struct must satisfy `interfaces.Project` via duck typing (no import needed)
4. Register in `scout/main.go` with `RegisterPlatform(<name>.New())`
5. Add to `PLATFORMS` env var
