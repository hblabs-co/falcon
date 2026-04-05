# falcon-scrape-api

Thin HTTP → NATS bridge for on-demand project scraping.

A client submits a URL and target platform; the API publishes a `scrape.requested.{platform}` event to NATS JetStream. The `falcon-scout` instance configured for that platform picks up the event and scrapes the URL through its normal pipeline (inspect → MongoDB → `project.created/updated`).

## Endpoints

### `POST /scrape`

Queue a single URL for immediate scraping.

**Request**
```json
{ "platform": "freelance.de", "url": "https://www.freelance.de/projekt-123-..." }
```

**Response `202 Accepted`**
```json
{ "status": "queued", "platform": "freelance.de", "url": "https://..." }
```

### `GET /health`

Returns `200 { "status": "ok" }`.

## NATS

| Stream   | Subject pattern          | Description                          |
|----------|--------------------------|--------------------------------------|
| `SCRAPE` | `scrape.requested.>`     | Published here on every POST /scrape |
| `SCRAPE` | `scrape.failed`          | Published by falcon-scout on error   |

Each `falcon-scout` replica subscribes to `scrape.requested.{PLATFORM}` with durable consumer `scout-{platform}` (dots replaced with dashes), so requests are load-balanced across replicas of the same platform.

## Configuration

| Variable          | Required | Default | Description                  |
|-------------------|----------|---------|------------------------------|
| `NATS_URL`        | yes      | —       | NATS server URL              |
| `MONGODB_URI`     | yes      | —       | MongoDB connection string     |
| `MONGODB_DATABASE`| yes      | —       | MongoDB database name         |
| `PORT`            | no       | `8082`  | HTTP listen port              |

## Flow

```
Client
  └─ POST /scrape { platform, url }
       └─ falcon-scrape-api
            └─ NATS: scrape.requested.{platform}
                 └─ falcon-scout (PLATFORM=freelance.de)
                      └─ inspect URL → MongoDB → project.created/updated
```
