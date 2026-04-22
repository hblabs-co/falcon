# syntax=docker.io/docker/dockerfile:1
# Multi-stage build for every Go service + the vendored Qdrant image.
# Each service is an independent bake target (see docker-bake.hcl) so CI
# can push just one at a time without rebuilding the rest.

# Global ARG — must come BEFORE the first FROM so it's in scope for any
# stage that references ${GO_VERSION} in its own FROM. Stages that need
# to consume the value inside RUN/COPY must re-declare `ARG GO_VERSION`
# after their FROM (none of ours do — only the FROM uses it).
ARG GO_VERSION=1.26

# ─────────────────────────────────────────────────────────────────────────
# Qdrant — a curl-enabled Qdrant image for dev / k8s healthchecks.
# Kept from the original Dockerfile so compose and prod share a tag.
# ─────────────────────────────────────────────────────────────────────────
FROM qdrant/qdrant:latest AS qdrant
RUN apt-get update \
 && apt-get install -y --no-install-recommends curl \
 && rm -rf /var/lib/apt/lists/*


# ─────────────────────────────────────────────────────────────────────────
# Shared Go builder for every Falcon service. Layout (post-COPY):
#   /src/common/                — hblabs.co/falcon/common
#   /src/modules/<name>/        — hblabs.co/falcon/modules/<name>
#   /src/<service>/             — hblabs.co/falcon/<service>
#
# All service go.mod files use `replace ../common` and similar, so we
# must copy those local deps BEFORE running `go mod download`. Build
# caches are mounted so repeat builds reuse the GOCACHE and GOMODCACHE.
# ─────────────────────────────────────────────────────────────────────────
FROM golang:${GO_VERSION}-alpine AS main-builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    apk add --no-cache git

# Shared modules — needed by every service via replace directives.
COPY common/           /src/common/
COPY modules/          /src/modules/

# Per-service sources. Copying whole trees is fine; the build cache
# keys off file contents, not the COPY instruction itself.
COPY falcon-api/          /src/falcon-api/
COPY falcon-dispatch/     /src/falcon-dispatch/
COPY falcon-match-engine/ /src/falcon-match-engine/
COPY falcon-normalizer/   /src/falcon-normalizer/
COPY falcon-realtime/     /src/falcon-realtime/
COPY falcon-signal/       /src/falcon-signal/
COPY falcon-storage/      /src/falcon-storage/
# falcon-scout is a Go workspace (go.work at falcon-scout/), binding
# the main `scout/` binary together with every platform scraper in
# `platforms/<domain>/`. We copy the whole tree so go.work can resolve
# every module at build time.
COPY falcon-scout/        /src/falcon-scout/

# Each service gets a `go mod download` and a build. Splitting into
# separate RUN steps lets BuildKit cache per-service when only one
# changes. CGO disabled everywhere since we only ship static binaries.
ENV CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH}

WORKDIR /src/falcon-api
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod tidy && \
    go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/falcon-api .

WORKDIR /src/falcon-dispatch
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod tidy && \
    go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/falcon-dispatch .

WORKDIR /src/falcon-match-engine
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod tidy && \
    go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/falcon-match-engine .

WORKDIR /src/falcon-normalizer
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod tidy && \
    go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/falcon-normalizer .

WORKDIR /src/falcon-realtime
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod tidy && \
    go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/falcon-realtime .

WORKDIR /src/falcon-signal
# No -trimpath for signal: email/config.go uses runtime.Caller(0) to
# locate the assets/ folder at startup. -trimpath would strip the
# absolute build path to a module-relative string, which breaks the
# filesystem lookup in the alpine runtime. The copy below lands the
# assets at the same /src/... path the compiled binary remembers.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod tidy && \
    go build -buildvcs=false -ldflags="-s -w" -o /out/falcon-signal .

WORKDIR /src/falcon-storage
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod tidy && \
    go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/falcon-storage .

# falcon-scout — built from the `scout/` subfolder. The parent's go.work
# wires in every platform module (`platforms/<domain>/`) so colly-based
# scrapers resolve without needing individual go mod download calls.
# GOFLAGS=-mod=mod tells the toolchain to respect the workspace.
WORKDIR /src/falcon-scout/scout
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/falcon-scout .


# ─────────────────────────────────────────────────────────────────────────
# Per-service runtime images. All share the same shape: alpine base,
# non-root user, ca-certificates so outbound TLS works (APNs, Mistral,
# MinIO, etc.). Each exposes a relevant port via metadata only — the
# actual listen happens in the binary.
# ─────────────────────────────────────────────────────────────────────────
FROM alpine:3.20 AS falcon-api
RUN addgroup -S app && adduser -S app -G app && apk add --no-cache ca-certificates
COPY --from=main-builder /out/falcon-api /app
USER app
EXPOSE 8080
ENTRYPOINT ["/app"]

FROM alpine:3.20 AS falcon-dispatch
RUN addgroup -S app && adduser -S app -G app && apk add --no-cache ca-certificates
COPY --from=main-builder /out/falcon-dispatch /app
USER app
ENTRYPOINT ["/app"]

FROM alpine:3.20 AS falcon-match-engine
RUN addgroup -S app && adduser -S app -G app && apk add --no-cache ca-certificates
COPY --from=main-builder /out/falcon-match-engine /app
USER app
ENTRYPOINT ["/app"]

FROM alpine:3.20 AS falcon-normalizer
RUN addgroup -S app && adduser -S app -G app && apk add --no-cache ca-certificates
COPY --from=main-builder /out/falcon-normalizer /app
USER app
ENTRYPOINT ["/app"]

FROM alpine:3.20 AS falcon-realtime
RUN addgroup -S app && adduser -S app -G app && apk add --no-cache ca-certificates
COPY --from=main-builder /out/falcon-realtime /app
USER app
EXPOSE 8090
ENTRYPOINT ["/app"]

FROM alpine:3.20 AS falcon-signal
RUN addgroup -S app && adduser -S app -G app && apk add --no-cache ca-certificates
COPY --from=main-builder /out/falcon-signal /app
# email/config.go reads assets at startup from the path packageDir()
# returns — which is the directory of email/config.go at compile time:
# /src/falcon-signal/email. Mirror that layout in the runtime image
# so os.ReadFile(filepath.Join(dir, asset.File)) succeeds.
COPY --from=main-builder --chown=app:app /src/falcon-signal/email/assets /src/falcon-signal/email/assets
USER app
ENTRYPOINT ["/app"]

FROM alpine:3.20 AS falcon-storage
RUN addgroup -S app && adduser -S app -G app && apk add --no-cache ca-certificates
COPY --from=main-builder /out/falcon-storage /app
USER app
ENTRYPOINT ["/app"]

FROM alpine:3.20 AS falcon-scout
RUN addgroup -S app && adduser -S app -G app && apk add --no-cache ca-certificates
COPY --from=main-builder /out/falcon-scout /app
USER app
ENTRYPOINT ["/app"]
