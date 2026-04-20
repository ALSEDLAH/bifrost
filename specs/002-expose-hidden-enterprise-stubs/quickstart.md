# Quickstart — Reviewer Walkthrough

**Feature**: 002-expose-hidden-enterprise-stubs
**Use this page to validate the feature end-to-end in under 10 minutes.**

## Prerequisites

- Bifrost enterprise build deployed (e.g. `bifrost-enterprise:build-002` running on `:8088`).
- Browser open to the deployment.
- Terminal with repo checked out at branch `002-expose-hidden-enterprise-stubs`.

## Walkthrough

### 1. Verify the "Contact Us" string is gone (SC-001)

From the terminal:

```bash
bash scripts/check-sc020-enterprise-stubs.sh
# Expect: "Scanned: N stub candidates / Whitelisted: 0 / Violations: 0"
```

The whitelist should be empty. Previously it was 21 entries.

### 2. Visit each of the 3 reuse-win pages

Navigate to each URL and confirm you see a real UI — not a panel.

| Route | Expect |
|---|---|
| `/workspace/governance/access-profiles` | A roles table framed as "access profiles". Should show Owner/Admin/Manager/Member plus any custom roles. |
| `/workspace/adaptive-routing` | An editor of weighted routing targets backed by `/api/governance/routing-rules`. |
| `/workspace/config/api-keys` | Real basic-auth curl example. **No trailing "Scope Based API Keys" teaser block anymore.** |

### 3. Spot-check 3 descoped pages

Navigate to any 3 of these and confirm you see a `FeatureStatusPanel` (NOT a marketing teaser):

- `/workspace/alert-channels`
- `/workspace/guardrails`
- `/workspace/login` (enterprise SSO login teaser)
- `/workspace/scim`
- `/workspace/cluster`

Each should render a panel that shows:

- Feature name (e.g. "Alert Channels")
- One-sentence description of what it would do
- Status badge (`Descoped` / `Needs own spec` / ...)
- A clickable tracking link (to spec 001's SR-01 row or a future spec)
- **No** "Contact Us" / "Upgrade" / "Enterprise license" text

### 4. Open DevTools and verify zero legacy strings (SC-001)

With the browser open on any descoped page:

```js
// In DevTools console:
document.body.innerText.includes("This feature is a part of the Bifrost enterprise license")
// Expect: false
```

Repeat on the dashboard + at least one reuse-win page. All three should return `false`.

### 5. Verify the `data-testid` contract

```js
document.querySelector('[data-testid="feature-status-panel"]')
// On a descoped page: returns the panel element.
// On a reuse-win page: returns null.
```

### 6. Regression: does the enterprise build still work?

Navigate through the spec-001 in-scope surface to confirm no regressions from the UI refactor:

- `/workspace/governance/rbac` — roles + permission matrix.
- `/workspace/governance/users` — user invites + role assignments.
- `/workspace/audit-logs` — filtered table + CSV/JSON export.
- `/workspace/mcp-auth-config` — OAuth status for MCP clients.

All four should render as they did under spec 001.

### 7. Run the headless SC-001 scan (CI parity)

```bash
# Runs Playwright against the deployment, greps rendered DOM for the legacy string.
bash ui/tests/e2e/enterprise/run-sc001-scan.sh http://localhost:8088
# Expect: "0 pages contain the legacy marketing string"
```

## Sign-off

If steps 1–7 all pass, the feature is ready to merge. If any step fails, capture the failing route + any error text and open a bug before merging.
