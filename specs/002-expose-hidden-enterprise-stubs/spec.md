# Feature Specification: Expose Hidden Enterprise Stubs (eliminate "Contact Us" dead-ends)

**Feature Branch**: `002-expose-hidden-enterprise-stubs`
**Created**: 2026-04-20
**Status**: Draft
**Input**: User description: "there are a lot of pages contains 'This feature is a part of the Bifrost enterprise license' and once of the requirements was mentioned to make sure no remain and implement or expose the features if they are hidden"

## Context

Spec 001-enterprise-parity shipped 8 in-scope enterprise stubs backed by real upstream logic. It also descoped 20 other user stories per scope rule **SR-01 "reuse-over-new"** — each descoped page kept its `ContactUsView` fallback with the marketing text *"This feature is a part of the Bifrost enterprise license. We would love to know more about your use case and how we can help you."*

Spec 001 revised **SC-020** to a whitelist model: the CI check only fails on marketing-teaser text for IN-SCOPE stubs. This satisfied the narrow scope but left **21 pages in the enterprise build still rendering a sales pitch** instead of useful content. From the operator's perspective, those pages are dead-ends that look buggy.

This spec tightens the surface by doing two complementary things:

1. **Re-audit** each descoped stub against the current upstream codebase to find any **hidden backing logic** missed in the original Phase 1 scan. If found, expose it (the SR-01 "reuse-over-new" rule still applies — we're looking for missed reuse, not licensing new invention).
2. **Replace remaining marketing teasers** on genuinely-not-built pages with an honest **informational panel** that names the feature, explains the current state (descoped / needs own spec / pending), and links to the tracking artifact. Operators should never see marketing copy inside their own admin product.

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Re-audit each descoped stub for hidden upstream logic (Priority: P1)

As a **release manager** closing out the enterprise-parity feature, I need a systematic audit that confirms every descoped UI stub *genuinely has no upstream backing*, so I know the scope boundary held up under scrutiny and I can defend "descoped" as an informed decision rather than a missed opportunity.

**Why this priority**: The original Phase 1 scan was fast; some descoped decisions may have been too quick. Re-auditing surfaces cheap wins (pages we can fill with tiny reuse work) and prevents carrying SR-01 decisions forward that a deeper look would have overturned.

**Independent Test**: Run the audit script/process against the current working tree. It produces a table with one row per ContactUsView-backed stub listing: upstream symbols searched, upstream endpoints found (if any), upstream storage tables found (if any), decision (expose / partial-expose / no-upstream-surface), and justification.

**Acceptance Scenarios**:

1. **Given** a ContactUsView-backed page (e.g. `alertChannelsView.tsx`), **when** the audit runs, **then** it records which upstream handlers / tables / plugins were inspected and whether any match the page's intent.
2. **Given** the audit finds *partial* upstream backing (e.g. upstream exposes read but not write), **then** the audit flags it for a bounded "reuse what exists" pass rather than a blanket descope.
3. **Given** the audit finds *no* upstream backing, **then** the decision row reads "no upstream surface — informational panel (see US2)" and US2 kicks in for that path.

---

### User Story 2 — Replace marketing teasers with honest informational panels (Priority: P1)

As an **enterprise operator** navigating the Bifrost admin UI, when I click into a feature that isn't built yet, I want to see a clear explanation of what the feature would do, why it's not available right now, and where the tracking work lives — not a "Contact Us" sales pitch that makes my own admin product feel like a demo.

**Why this priority**: Dead-end marketing copy in an already-purchased product is a poor UX signal. A short, truthful panel preserves trust. This is the bulk of the visible change for end users.

**Independent Test**: Navigate to any descoped enterprise route (e.g. `/workspace/alert-channels`, `/workspace/guardrails`) in the enterprise build. The page must render an `InfoPanel` (or equivalent) showing feature name, current state ("descoped — needs its own spec" / "hidden upstream logic surfaced partially" / "pending implementation"), and a link to the tracking artifact. The string "This feature is a part of the Bifrost enterprise license" must not appear in the rendered DOM.

**Acceptance Scenarios**:

1. **Given** a descoped enterprise page with no upstream backing, **when** it renders, **then** an `InfoPanel` shows: feature name, one-line "what would ship" summary, status label, link to tracking issue/spec, and no marketing CTA.
2. **Given** an upstream-partial page (audit found some logic), **when** it renders, **then** it surfaces the available functionality first and the `InfoPanel` only covers the remaining gap.
3. **Given** an operator on any enterprise route under `ui/app/enterprise/components/`, **when** they open browser DevTools and search the rendered DOM, **then** zero matches exist for the marketing string.

---

### User Story 3 — Strengthen the SC-020 CI check to block future regressions (Priority: P2)

As a **maintainer** reviewing pull requests against the enterprise branch, I want the existing `check-sc020-enterprise-stubs.sh` to evolve from a path-whitelist check into a single *zero-tolerance* guard on the full enterprise build: no marketing-teaser string may appear anywhere in the shipped UI bundle regardless of path.

**Why this priority**: Once every stub has been re-audited and informational panels exist everywhere, the per-path whitelist becomes the weakest link — someone adding a new page can unintentionally reintroduce the teaser. A build-wide guard closes that door permanently.

**Independent Test**: Run the evolved check against a clean enterprise build. It must report zero violations and zero whitelisted paths (the whitelist becomes empty once US1 + US2 are applied).

**Acceptance Scenarios**:

1. **Given** a PR introduces a new enterprise page that contains the legacy marketing string, **when** the CI check runs, **then** it fails with a clear message naming the offending file.
2. **Given** the current codebase after US1 + US2 complete, **when** the CI check runs, **then** it reports zero violations and the whitelist in the script is empty.

---

### Edge Cases

- **Upstream partial match (e.g. read but not write)**: the page renders the read UI at top and the InfoPanel at the bottom explaining the write-path gap. Not all-or-nothing.
- **Upstream logic exists but its shape mismatches the UI fallback's type**: the audit flags the adapter work required; if small (<1 day), it's an in-scope reuse; if large, it's descoped and gets a richer InfoPanel.
- **Fallback is used by multiple enterprise pages**: change the fallback once; every consumer inherits the new panel.
- **Operator visiting an informational panel page in a non-enterprise (OSS) build**: the OSS build already shows `ContactUsView` and is separate; this spec targets only the *enterprise* build.
- **New enterprise page added after this spec ships**: the evolved SC-020 CI check catches any reintroduction of the marketing string regardless of path, preventing regressions.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Every enterprise component currently re-exporting a `ContactUsView` fallback MUST be revisited; each MUST have a documented audit row (upstream signals searched, finding, decision).
- **FR-002**: For any descoped stub where the audit finds exposeable upstream logic (full or partial), the UI MUST be updated to expose that logic. "Exposeable" = existing handler, table, or plugin whose intent matches the page's topic, per SR-01 "reuse-over-new" from spec 001.
- **FR-003**: For any descoped stub where the audit confirms no upstream backing exists, the page MUST replace `ContactUsView` with an informational panel that shows: (a) feature name, (b) single-sentence summary of what it would do, (c) status label (one of: `descoped`, `needs-own-spec`, `upstream-partial`, `pending-implementation`), (d) link to the tracking artifact (issue URL, spec path, or SR-01 section reference), (e) no "Contact Us" CTA and no licensing or sales copy.
- **FR-004**: The informational panel MUST be a **new shared component** distinct from `ContactUsView` (e.g. `FeatureStatusPanel`). `ContactUsView` itself MUST remain untouched — it is still the correct component for the OSS fallback build where the feature legitimately is gated behind a commercial license.
- **FR-005**: Existing enterprise routes (under `ui/app/workspace/**` and `/workspace/governance/**`) MUST NOT be changed. The replacement happens inside each stub's component only.
- **FR-006**: The SC-020 CI check MUST evolve from a path-whitelist model to a zero-tolerance guard: the legacy marketing string MUST NOT appear in any file under `ui/app/enterprise/components/` regardless of path. The whitelist field in the check script MUST be removed (or emptied with a comment explaining it's retained only for future additive use).
- **FR-007**: The audit output MUST be committed as `research.md` in this feature's spec directory, with one table row per stub. Future contributors MUST be able to look up any descoped page and see the exact upstream search that produced the "no backing" verdict.
- **FR-008**: When the evolved CI check runs in pre-commit or CI, any regression (a new page reintroducing the marketing string) MUST fail the build with a message naming the offending file path and line.

### Key Entities

- **Enterprise stub**: A component file under `ui/app/enterprise/components/` that currently re-exports its OSS fallback. Today 21 such files are whitelisted.
- **Upstream audit row**: A record `{ path, upstream_symbols_searched, upstream_finding, decision, tracking_link }` documenting the Phase-1-redux scan for one stub.
- **FeatureStatusPanel**: A new shared UI component replacing `ContactUsView` inside the *enterprise* build only. Shows status + tracking link; carries no sales copy.
- **SC-020 check**: The existing `scripts/check-sc020-enterprise-stubs.sh` + `.github/workflows/sc020-enterprise-stubs.yml`. This feature evolves both.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Zero occurrences of the string "This feature is a part of the Bifrost enterprise license" in the full rendered DOM of any route under `/workspace/**` in an enterprise-built deployment (verified by a headless browser scan).
- **SC-002**: Every descoped stub page loads an informational panel (or a real implementation, per US1-sourced finds) in under 200ms from first paint on a modern laptop, with no network fallback required.
- **SC-003**: `scripts/check-sc020-enterprise-stubs.sh` reports **0 violations** and **0 whitelisted paths** after this feature lands. Rerunning on a PR that adds the legacy string anywhere under `ui/app/enterprise/components/` fails the check.
- **SC-004**: `research.md` at the feature spec path contains an audit row for each of the 21 currently-whitelisted stubs, with a clear decision (`expose`, `expose-partial`, `descope → informational-panel`, `needs-own-spec`) and a justification anchored in an upstream code reference (file + line or symbol).
- **SC-005**: At least 3 of the 21 stubs shift from "descoped" to "expose" or "expose-partial" after the audit — i.e., the audit surfaces at least some cheap reuse wins we missed the first time. (If fewer than 3 surface, that's itself a strong signal that SR-01's Phase-1 scan was thorough and the remaining descopes are well-reasoned.)

## Assumptions

- The SR-01 "reuse-over-new" rule from spec 001 remains authoritative. This spec does NOT license new backend features; it only closes the gap between "what's already in the upstream codebase" and "what's surfaced in the enterprise UI".
- Operators of the enterprise build prefer honest informational panels over marketing copy even when the underlying feature genuinely isn't built. The user's framing in the original request treats remaining "Contact Us" copy as a defect.
- The informational panel is a small, shared component; designing it is a few hours of work, not a full UX pass. Copy is terse and factual.
- Tracking links may point to GitHub issues, follow-up spec paths (`specs/NNN-...`), or SR-01 table cells in spec 001 — all three are acceptable anchors.
- The evolved CI check stays source-based (greps `.tsx` files) rather than parsing the minified bundle. Source-based checks are deterministic across build-hash changes and already work today.
- Descoped user stories from spec 001 (US3, US5, US6, US7, US9–US12, US15–US29) stay descoped. This spec does not promote any of them into active implementation; it only replaces their marketing teasers with honest status panels **if** the re-audit confirms no upstream backing.
