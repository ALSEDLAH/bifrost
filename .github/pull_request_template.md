## Summary

Briefly explain the purpose of this PR and the problem it solves.

## Changes

- What was changed and why
- Any notable design decisions or trade-offs

## Type of change

- [ ] Bug fix
- [ ] Feature
- [ ] Refactor
- [ ] Documentation
- [ ] Chore/CI

## Affected areas

- [ ] Core (Go)
- [ ] Transports (HTTP)
- [ ] Providers/Integrations
- [ ] Plugins
- [ ] UI (React)
- [ ] Docs

## How to test

Describe the steps to validate this change. Include commands and expected outcomes.

```sh
# Core/Transports
go version
go test ./...

# UI
cd ui
pnpm i || npm i
pnpm test || npm test
pnpm build || npm run build
```

If adding new configs or environment variables, document them here.

## Screenshots/Recordings

If UI changes, add before/after screenshots or short clips.

## Breaking changes

- [ ] Yes
- [ ] No

If yes, describe impact and migration instructions.

## Related issues

Link related issues and discussions. Example: Closes #123

## Security considerations

Note any security implications (auth, secrets, PII, sandboxing, etc.).

## Checklist

- [ ] I read `docs/contributing/README.md` and followed the guidelines
- [ ] I added/updated tests where appropriate
- [ ] I updated documentation where needed
- [ ] I verified builds succeed (Go and UI)
- [ ] I verified the CI pipeline passes locally if applicable

## Constitution Compliance (enterprise PRs only — skip for upstream-only changes)

The Bifrost enterprise constitution lives at `.specify/memory/constitution.md`.
Confirm each principle this PR may touch:

- [ ] **I. Core Immutability** — no edits under `core/**`
- [ ] **II. Non-Breaking** — new config fields optional with safe defaults; existing plugin hook signatures stable
- [ ] **III. Plugin-First** — feature lives under `plugins/<name>/` or `framework/<subsystem>/`
- [ ] **IV. Config-Driven Gating** — feature toggle via `config.json`; no build tags
- [ ] **V. Multi-Tenancy First** — new tables carry `organization_id` / `workspace_id`
- [ ] **VI. Observability** — OTEL span + Prometheus metric + audit emit all wired
- [ ] **VII. Security by Default** — secrets encrypted at rest, TLS required, redaction hooked
- [ ] **VIII. Test Coverage** — integration tests on real PostgreSQL/vectorstore; Playwright for UI
- [ ] **IX. Docs & Schema Sync** — `config.schema.enterprise.json` updated, MDX in `docs/enterprise/`, changelog entry in affected modules
- [ ] **X. Dependency Hierarchy** — no reverse imports; plugin module independent
- [ ] **XI. Upstream-Mergeability** — additive-by-sibling-file; `E###_` migration IDs; schema overlay; hook anchors only on upstream files


