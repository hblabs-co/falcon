# Categorical vs Per-Item — redglobal.de

Classification of every error/warning the scraper can emit. **Categorical** =
one Mongo doc per `service:platform:name`, `occurrence_count` tracks frequency.
**Per-item** = one doc per affected job, fresh nanoid each call.

Enforced by what the runner passes as `Candidate`:
- `r.err(..., nil)` → categorical (upsert)
- `r.err(..., c)`   → per-item (insert)

## Rule

Categorical when the problem affects a **shared resource**, gets fixed **once**,
and a second call would fail the same way. Per-item when the problem is about
**one piece of content** and other items can still succeed.

## Errors

| Name | Class | Why |
|---|---|---|
| `scrape_listing_empty` | categorical | Listing markup broke. No specific candidate; affects every poll. |
| `scrape_unauthorized` (401/403) | categorical | Auth/session is dead. Every request will fail until re-auth. |
| `scrape_server_error` (5xx) | per-item | Usually transient; retry worker reattempts the same job later. |
| `scrape_inspect_failed` | per-item | Catch-all; could be a one-off bad job. |
| `scrape_gone` (410) | not recorded | Skip silently; nothing to retry or fix. |

## Warnings

| Name | Class | Why |
|---|---|---|
| `reference_id_not_found` | categorical | Markup change affects all detail pages identically. Find affected jobs via `db.projects.find({platform: "redglobal.de", reference_id: ""})`. |
| `company_metadata_changed` | per-item | Each change is a historical event worth preserving separately. |

## To add later

| Name | Class | Trigger |
|---|---|---|
| `scrape_detail_markup_drift` | categorical | N consecutive `no JobPosting ld+json` failures in one poll. |
| `scrape_blocked` | categorical | 403 or Cloudflare challenge HTML. |
| `scrape_dns_failed` | categorical | DNS error from `c.Visit`. |
| `scrape_tls_failed` | categorical | TLS handshake error from `c.Visit`. |
