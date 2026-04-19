# Golden-Set Replay Harness

Validates Constitution **SC-001**: an existing Bifrost v1.5.2 OSS deployment
upgrading to the enterprise-capable build with zero config changes
experiences no behavior change.

## Method

1. **Capture phase** (one-time, on a v1.5.2 deployment):
   - Run a representative 100-request traffic suite against the OSS gateway.
   - Save each request + response pair as a JSONL file under
     `tests/golden/v1.5.2/`.

2. **Replay phase** (CI / pre-release):
   - Boot a fresh Bifrost (any v1.6.x+) with the v1.5.2 config snapshot.
   - For each captured request, replay through the new gateway.
   - Diff response body, status code, headers (excluding date / request-id),
     and metric series names + cardinality.
   - PASS = byte-identical responses + same metric label set.

## Files

- `corpus/` — captured requests (gitignored; populated by the operator's
  capture run; see `scripts/capture-golden.sh`).
- `expected/` — captured responses (paired with corpus by file ID).
- `replay.go` — replay runner (compiled with `go test -tags=golden`).
- `diff.go` — response-comparison logic with header/timestamp normalization.

## Operator runbook

```bash
# Capture (run once against your v1.5.2 production-equivalent deployment)
./scripts/capture-golden.sh --target https://your-bifrost:8080 --requests 100

# Replay (CI step on every release candidate)
make test-golden-replay
```

## CI integration

Wired into `.github/workflows/release-pipeline.yml` (post-Phase-1 task);
fails the release if any response diverges. Configurable allow-list of
deltas via `tests/golden/allowed-deltas.yaml` (e.g., a metric label that
upstream renamed in a backwards-compatible way).
