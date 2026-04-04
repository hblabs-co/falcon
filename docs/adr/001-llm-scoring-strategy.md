# ADR-001 — LLM Strategy for CV/Project Scoring

**Date:** 2026-04-05
**Status:** In evaluation
**Service:** falcon-match-engine

---

## Context

`falcon-match-engine` needs an LLM to evaluate how well a candidate's CV matches a
project description across six dimensions, returning a structured JSON score.

The key constraints are:
- **GDPR**: CV text and project descriptions are personal/sensitive data. The LLM provider must be EU-based or run on-premise.
- **Structured output**: the model must reliably return valid JSON with a specific schema.
- **Calibration**: scores must reflect real gaps — a lenient model that scores everyone as "top candidate" has no business value.
- **Latency**: each scoring call is triggered by a `match.pending` event. Acceptable range is 5–60s per call since the service scales horizontally.

---

## Options evaluated

### Option A — Ollama local: `qwen2.5:7b`

Tested on MacBook Air (Apple Silicon, 16GB RAM) with `OLLAMA_NUM_CTX=16384`.

**Results:**
- ✅ Zero data egress — fully GDPR compliant
- ✅ No API cost
- ✅ JSON output mostly parseable after adding robust extraction (`{...}` search)
- ❌ **Leniency bias**: model consistently inflated scores. Candidates missing must-have requirements still scored as `top_candidate`
- ❌ `missing_skills` frequently returned as empty even when gaps were obvious
- ❌ `communication_clarity` gave perfect 10s by default
- ❌ Context window defaulted to 4096 tokens — truncated long CVs silently. Required `OLLAMA_NUM_CTX=16384` at server start; the `/v1/chat/completions` endpoint ignores per-request `options.num_ctx`
- ❌ Cold inference ~88s on first call; warm ~15-25s

**Prompt improvements applied:**
- Added explicit calibration rules to system prompt (scores 9-10 are exceptional, not default)
- Added hard rule: if must-have requirement is missing, overall score ≤ 5.0
- Required `negative_points` for any score < 8
- Clarified `missing_skills` empty array means ALL requirements are covered — should be rare
- Restricted `tech_stack_overlap` to explicit matches only (no inferred overlap)

**Outcome:** Improved but model still condescending. 7B params insufficient for rigorous technical evaluation at this complexity level.

---

### Option B — Ollama local: `qwen2.5:14b`

Not yet tested. Estimated requirements: ~10GB RAM, ~30-45s/call on MacBook Air 16GB.

**Expected improvement over 7B:** Significantly better instruction following and calibration.
**Blocker:** MacBook Air 16GB is marginal for 14B — may be too slow for practical use.

**Recommended if:** a Mac Mini M-series (32GB+) is added to the k3s cluster as a dedicated inference node (same setup as bge-m3 for embeddings).

---

### Option C — Mistral AI API: `mistral-small-latest`

**Specs:** 22B parameter model, EU servers (France), GDPR-compliant by design.

- ✅ Strongest instruction following and calibration of options evaluated
- ✅ GDPR compliant — Mistral AI is a French company, servers in EU
- ✅ OpenAI-compatible API — zero code changes, only env vars
- ✅ ~$0.10 / 1M input tokens, ~$0.30 / 1M output tokens
- ✅ Estimated cost: ~$0.001–0.003 per scoring call (CV ~2000 tokens + project ~500 + prompt ~800)
- ❌ Data leaves infrastructure (goes to Mistral EU servers)
- ❌ Requires internet connectivity from k3s cluster

**Switch:** set `LLM_URL=https://api.mistral.ai`, `LLM_MODEL=mistral-small-latest`, `LLM_API_KEY=<key>` — no code changes.

---

### Option D — Mac Mini M-series as dedicated inference node

Add a Mac Mini (M2 Pro / M3, 32–64GB RAM) as a k3s agent node running Ollama natively.

- Runs both `bge-m3` (embeddings) and `qwen2.5:14b` or `qwen2.5:32b` (scoring) with Metal GPU
- Zero data egress, full GDPR compliance
- One-time hardware cost (~€800–1400), zero ongoing API cost
- Already have the pattern from bge-m3 setup (ExternalName K8s Service → Mac Mini IP)

**Recommended for:** high volume production where data sovereignty is a hard requirement and API costs become significant.

---

## Decision

**Current (dev):** Option A — `qwen2.5:7b` via Ollama local.
Sufficient to validate the pipeline end-to-end. Leniency bias is a known limitation.

**Next test:** Option C — Mistral Small API.
Will validate scoring quality improvement. If results are significantly better, adopt as default for production.

**Long-term production path:**
- Low/medium volume → Option C (Mistral API, EU-compliant, low cost)
- High volume or strict data sovereignty → Option D (Mac Mini dedicated inference node)

---

## Consequences

- `falcon-match-engine` must never be pointed at OpenAI, Anthropic, or any non-EU provider without a DPA review.
- The `LLM_URL`, `LLM_API_KEY`, `LLM_MODEL` env vars are the only switch needed between options.
- Score threshold (`MATCH_ENGINE_SCORE_THRESHOLD`) may need recalibration when switching models — a well-calibrated model will produce lower scores for marginal candidates.
