## For registry.4the.company >> docker login registry.4the.company
## Then type username and password

## Falcon — multi-service Docker build.
##
## Typical usage:
##   docker buildx bake --push               # builds the default group
##   docker buildx bake falcon-api --push    # a single target
##   REGISTRY=myreg.local TAG=2025-05-08 \
##     docker buildx bake --push             # override registry or tag
##
## Each target maps 1:1 to a stage in the root Dockerfile. Stage name,
## bake target name, and image name all match to keep mental overhead low.

variable "REGISTRY" {
  default = "registry.4the.company"
}

variable "TAG" {
  default = "1.0.0"
}

variable "GO_VERSION" {
  default = "1.26"
}

group "default" {
  // What `bake` builds when no target is specified. Scout is commented
  // out by default — it ships with every platform scraper, so builds
  // are heavier than the other services. Uncomment when you need to
  // roll out scraper changes.
  targets = [
    "falcon-api",
    "falcon-dispatch",
    "falcon-match-engine",
    "falcon-normalizer",
    "falcon-realtime",
    "falcon-signal",
    "falcon-storage",
    "falcon-scout",
  ]
}

// Shared fields — DRY via target inheritance. Individual targets only
// override what they need (usually just `target`).
target "_defaults" {
  context    = "."
  dockerfile = "Dockerfile"
  platforms  = ["linux/amd64", "linux/arm64"]
  push       = true
  provenance = false
  args = {
    GO_VERSION = "${GO_VERSION}"
  }
}

target "falcon-api" {
  inherits = ["_defaults"]
  target   = "falcon-api"
  tags     = ["${REGISTRY}/falcon-api:${TAG}"]
}

target "falcon-dispatch" {
  inherits = ["_defaults"]
  target   = "falcon-dispatch"
  tags     = ["${REGISTRY}/falcon-dispatch:${TAG}"]
}

target "falcon-match-engine" {
  inherits = ["_defaults"]
  target   = "falcon-match-engine"
  tags     = ["${REGISTRY}/falcon-match-engine:${TAG}"]
}

target "falcon-normalizer" {
  inherits = ["_defaults"]
  target   = "falcon-normalizer"
  tags     = ["${REGISTRY}/falcon-normalizer:${TAG}"]
}

target "falcon-realtime" {
  inherits = ["_defaults"]
  target   = "falcon-realtime"
  tags     = ["${REGISTRY}/falcon-realtime:${TAG}"]
}

target "falcon-signal" {
  inherits = ["_defaults"]
  target   = "falcon-signal"
  tags     = ["${REGISTRY}/falcon-signal:${TAG}"]
}

target "falcon-storage" {
  inherits = ["_defaults"]
  target   = "falcon-storage"
  tags     = ["${REGISTRY}/falcon-storage:${TAG}"]
}

target "falcon-scout" {
  inherits = ["_defaults"]
  target   = "falcon-scout"
  tags     = ["${REGISTRY}/falcon-scout:${TAG}"]
}

// Qdrant image with curl baked in, for k8s readiness probes that
// prefer HTTP over exec. Independent release cycle from the Go services.
target "qdrant" {
  inherits = ["_defaults"]
  target   = "qdrant"
  tags     = ["${REGISTRY}/falcon-qdrant:${TAG}"]
  args     = {}
}
