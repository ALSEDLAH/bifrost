# Upstream Sync Runbook

**Purpose**: Keep the Bifrost enterprise fork mergeable with
`upstream = https://github.com/maximhq/bifrost.git` on a weekly
cadence, without fighting conflicts.

**Governed by**: Constitution [Principle XI](./.specify/memory/constitution.md)
"Upstream-Mergeability". Design in
[specs/001-enterprise-parity/research.md §R-24](./specs/001-enterprise-parity/research.md).

---

## One-time setup

```bash
git remote add upstream https://github.com/maximhq/bifrost.git
git fetch upstream
```

Verify `.github/workflows/upstream-sync.yml` and
`.github/workflows/drift-watcher.yml` are enabled on your fork's
Actions tab.

---

## Weekly merge flow (automated; maintainer reviews)

```
Every Monday 09:00 UTC (GitHub Action "upstream-sync"):

  ┌─────────────────────────────────────────────────────────────────┐
  │ 1. Fetch                                                        │
  │    git fetch upstream                                           │
  └──────────────────────────────┬──────────────────────────────────┘
                                 ▼
  ┌─────────────────────────────────────────────────────────────────┐
  │ 2. Branch                                                       │
  │    git checkout -b sync/upstream-$(date +%Y-%m-%d) main         │
  └──────────────────────────────┬──────────────────────────────────┘
                                 ▼
  ┌─────────────────────────────────────────────────────────────────┐
  │ 3. Merge (no rebase — preserves history)                        │
  │    git merge upstream/main --no-ff                              │
  │                                                                 │
  │    ┌─ Clean merge ─────► goto 4                                 │
  │    │                                                            │
  │    └─ Conflicts ─────────► goto conflict playbook (below)       │
  └──────────────────────────────┬──────────────────────────────────┘
                                 ▼
  ┌─────────────────────────────────────────────────────────────────┐
  │ 4. Gate                                                         │
  │    make check-core-unchanged  (Principle I — always green)      │
  │    make check-imports         (Principle X)                     │
  │    make check-obs-completeness(Principle VI)                    │
  │    make test-core                                               │
  │    make test-plugins                                            │
  │    make test-enterprise                                         │
  │    ./scripts/run-golden-replay.sh  (SC-001: byte-identical OSS) │
  │                                                                 │
  │    ┌─ All green ─────────► goto 5                               │
  │    └─ Any red ────────────► maintainer triages; DO NOT merge    │
  └──────────────────────────────┬──────────────────────────────────┘
                                 ▼
  ┌─────────────────────────────────────────────────────────────────┐
  │ 5. PR                                                           │
  │    Open PR: "sync: upstream/main @ <short-sha>"                 │
  │    Body: commit range + conflict summary + test results         │
  │    Maintainer reviews, merges on green.                         │
  └─────────────────────────────────────────────────────────────────┘
```

A missed week = **maintenance debt**. Two missed weeks = **incident**:
stop new feature work until the backlog is drained.

---

## Conflict playbook

Run through the hot-spot files in order. For each, the rule from
[Principle XI](./.specify/memory/constitution.md) tells you how to
resolve:

| Conflict location | Rule | Resolution |
|-------------------|------|------------|
| `core/**` | I | Upstream always wins. Our side should be empty delta — if it isn't, something is broken; open an issue and back out. |
| `transports/config.schema.json` | XI.3 | Upstream wins on its body. Preserve our single `allOf: [{ $ref: "./config.schema.enterprise.json" }]` anchor. Enterprise additions were made to the overlay file, not this one. |
| `transports/bifrost-http/lib/middleware.go` | XI.4 | Upstream wins on its body. Preserve our single `RegisterEnterpriseMiddleware(chain)` call line. All enterprise middleware lives in `middleware_enterprise.go`. |
| `plugins/governance/main.go` (+ logging, telemetry, otel, semanticcache, prompts) | XI.1 | Upstream wins. Our extensions are sibling files (`budgets.go`, `pii_hook.go`, ...) which never conflict. If you find enterprise code inside `main.go`, that is a Principle XI violation — move it to a sibling and rerun the merge. |
| `framework/configstore/migrations/*` or `framework/logstore/migrations/*` | XI.2 | Upstream's `NNN_*.sql` and our `E###_*.sql` (in `migrations-enterprise/`) never collide. A conflict here means the runner or the naming convention was broken; fix the runner, not the migration. |
| `ui/src/routes.tsx` | XI.4 | Upstream wins. Preserve our single `[...upstreamRoutes, ...enterpriseRoutes]` line. Enterprise routes live in `enterpriseRoutes.ts`. |
| `AGENTS.md` / `CLAUDE.md` | XI.1 | Prefer upstream body; append our enterprise addendum section after their content. |
| `Makefile` | XI.1 | Append-only for enterprise targets (`test-enterprise`, `upstream-sync`, `check-imports`). If upstream reorders, accept their order and re-append our targets at the end. |
| `go.work` | XI.1 | Accept upstream's ordering; add enterprise modules at the end. |
| `helm-charts/bifrost/values.yaml` | XI.3-style | Our enterprise defaults live in `values-enterprise.yaml` and `values-airgapped.yaml`; upstream `values.yaml` stays nearly pristine. |

**When in doubt**: accept upstream, then open a sibling PR that
re-applies any genuinely necessary enterprise modification as an
additive-by-sibling-file change. Don't try to merge "cleverly" in the
conflict itself.

---

## CI drift watcher (`.github/workflows/drift-watcher.yml`)

On every PR, the drift watcher runs:

```bash
git fetch upstream
DIFF_LINES=$(git diff --stat upstream/main -- $(cat .github/drift-watchlist.txt) \
             | tail -1 | awk '{print $4 + $6}')
BASELINE=$(jq -r '.baseline_lines' .github/drift-baseline.json)
THRESHOLD=$((BASELINE + 50))

if [ "$DIFF_LINES" -gt "$THRESHOLD" ] && \
   ! git log -1 --pretty=%B | grep -q '^drift:'; then
  echo "::error::Drift grew from $BASELINE → $DIFF_LINES lines (ceiling $THRESHOLD)."
  echo "Move new additions into a sibling file, or prefix the commit with 'drift:'"
  exit 1
fi
```

The baseline resets weekly after each successful upstream-sync merge.

**Watch list** (`.github/drift-watchlist.txt`):

```
plugins/governance/
plugins/logging/
plugins/telemetry/
plugins/otel/
plugins/semanticcache/
plugins/prompts/
transports/bifrost-http/lib/middleware.go
transports/config.schema.json
AGENTS.md
CLAUDE.md
Makefile
go.work
helm-charts/bifrost/values.yaml
```

---

## Upstream-carried patches registry

Patches we keep on our side while waiting for upstream action. Every
entry must cite the upstream issue/PR and a removal plan. A quarterly
review archives resolved entries.

| Date added | Files | Upstream issue/PR | Reason | Removal plan |
|-----------|-------|-------------------|--------|--------------|
| (none yet) | | | | |

**Process**:

1. When an enterprise change genuinely requires an upstream-file edit,
   open the PR against `maximhq/bifrost` first.
2. If it merges → our patch disappears on next weekly sync; note
   removal in the PR.
3. If it languishes > 90 days → add to this table with
   `Reason: upstream-delay` and a target decision date.
4. If upstream rejects → add `Reason: rejected upstream: <why>` and
   accept we carry it; re-evaluate when upstream refactors the area.

---

## Emergency recovery — "we missed 2+ weeks"

1. **Stop new enterprise feature work.** All PRs pause.
2. Maintainer runs `git fetch upstream` and inspects
   `git log --oneline main..upstream/main` — size the debt.
3. Merge upstream/main into a long-running `sync/catchup-YYYY-MM-DD`
   branch. Resolve conflicts file-by-file using the playbook above.
4. Run the full test matrix. Fix regressions in dedicated commits.
5. Open a single large PR with a conflict summary per file.
6. Merge; reset the drift baseline; resume weekly cadence next
   Monday.

If the catchup merge still fails after 3 working days, escalate to
the maintainers' channel and consider cherry-picking critical upstream
fixes only (bug-fix commits, not feature commits) until the fork can
re-converge.

---

## Upstreaming bias (Principle XI rule 7)

When a primitive added to our fork is not inherently enterprise-gated,
file it upstream first. Good candidates from the current plan:

- `framework/tenancy/` (workspace/org scoping is broadly useful).
- `plugins/audit/` sink (every gateway eventually needs audit).
- `framework/crypto/` envelope (encryption of configstore secrets is
  not enterprise-specific).
- Plugin-module independent versioning conventions.
- Migration-namespace discovery itself (T325).

An accepted upstream PR shrinks our carry delta permanently and is
the single highest-leverage act the maintainer can take.

---

## Quick reference

- **Remote**: `upstream = https://github.com/maximhq/bifrost.git`
- **Cadence**: weekly, Monday 09:00 UTC
- **Merge mode**: `--no-ff` merge (never rebase the fork)
- **Tests**: core + plugins + enterprise + golden-set replay all must
  be green before merging the sync PR
- **Drift ceiling**: baseline + 50 lines across the watchlist; tagged
  commits (`drift:`) are the only exception
- **Escape hatch**: documented, time-bound, registry-tracked
