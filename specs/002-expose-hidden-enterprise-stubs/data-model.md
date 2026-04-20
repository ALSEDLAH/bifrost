# Data Model — Expose Hidden Enterprise Stubs

**Feature**: 002-expose-hidden-enterprise-stubs

This feature is UI-only and introduces **no database tables, no API endpoints, no config-schema additions**. The "data model" here is limited to two TypeScript shapes that govern how the shared component is used.

## 1. `FeatureStatusPanel` props

```ts
export type FeatureStatusLabel =
  | "descoped"           // not planned in the current spec; reviewable under a separate future spec
  | "needs-own-spec"     // acknowledged follow-up; tracking link points to the tracking artifact
  | "upstream-partial"   // some functionality surfaced elsewhere; this path kept for route compatibility
  | "pending-implementation"; // planned and scheduled; tracking link points to the sprint/issue

export interface FeatureStatusPanelProps {
  /** Short, human name of the feature (e.g. "Alert Channels"). */
  title: string;

  /** One-sentence description of what the feature would do when it ships. */
  description: string;

  /** Current state label. Drives the icon + color scheme in the UI. */
  status: FeatureStatusLabel;

  /** Where a curious operator goes to learn more: future spec path,
   *  GitHub issue URL, or SR-01 classification cell in spec 001. */
  trackingLink: {
    href: string;
    /** Short label like "spec 003", "issue #42", "SR-01 out-of-scope row". */
    label: string;
  };

  /** Optional alternate-route hint for vestigial pages — e.g. pointing
   *  operators to /workspace/governance/business-units when they hit
   *  the deprecated organizationSettingsView path. */
  alternativeRoute?: {
    href: string;
    label: string;
  };

  /** Optional lucide-react icon component. Defaults chosen per status. */
  icon?: React.ComponentType<{ className?: string }>;
}
```

**Rules enforced by the component:**

- The string `"This feature is a part of the Bifrost enterprise license"` MUST NOT appear anywhere in the rendered output. Contract-tested in visual-smoke.
- There MUST be no `"Contact Us"` / `"Contact sales"` / `"Upgrade"` CTAs. This panel is admin-internal; selling already happened.
- The rendered panel MUST include exactly one `data-testid="feature-status-panel"` element at its root, so Playwright can assert presence per route.

## 2. `AuditRow` (the shape of each row in `research.md`)

Not persisted to any database — this is the tabular shape used in Phase 0's audit Markdown table.

```ts
interface AuditRow {
  stubPath: string;             // repo-relative, e.g. "ui/app/enterprise/components/alert-channels/alertChannelsView.tsx"
  topic: string;                // human name of what the stub would show
  upstreamHandler: string | null;   // file:line or "none"
  upstreamTable: string | null;     // table name or "none"
  clientHook: string | null;        // RTK hook name if one exists
  fallbackContent: string;      // what the fallback currently renders
  verdict: "expose" | "expose-partial" | "descope → FeatureStatusPanel";
  rationale: string;            // 1-2 sentence justification
}
```

## 3. Relationships & flow

```text
┌────────────────────────┐         audit populates         ┌──────────────────┐
│  20 enterprise stubs   │ ─────────────────────────────── │  research.md     │
│  (ui/app/enterprise/)  │                                 │  (audit table)   │
└──────────────────────┬─┘                                 └────────┬─────────┘
                       │                                            │
                       │              verdict dictates              │
                       ▼                                            ▼
   ┌───────────────────┴──────┐                   ┌─────────────────┴──────────┐
   │ verdict=expose / partial │                   │ verdict=descope           │
   │ (3 stubs)                │                   │ (17 stubs)                 │
   └──────────┬───────────────┘                   └─────────────┬──────────────┘
              │                                                 │
              │ replace stub body with thin wrap                │ replace stub body with
              │ over existing upstream handler                  │ <FeatureStatusPanel .../>
              ▼                                                 ▼
   ┌────────────────────────┐                       ┌───────────────────────────┐
   │  Real UI view mounted  │                       │ Honest informational      │
   │  at existing route     │                       │ panel at existing route   │
   └────────────────────────┘                       └───────────────────────────┘
```

## State transitions

`FeatureStatusPanel` is stateless at component level. Its `status` prop is decided at the call site based on the audit row; changes over time are tracked via PRs (e.g. when a descoped story ships in its own spec, its consumer page flips from `<FeatureStatusPanel status="descoped" />` to the real implementation).

## Validation rules

- `trackingLink.href` MUST be either a relative path starting with `/`, a `specs/...` path, or an absolute URL starting with `https://`. No `mailto:` and no `#` fragments — operators shouldn't be asked to email anyone from inside the admin product.
- `status` MUST be one of the four labels above. Unrecognized values fail TypeScript compilation.
- `description` MUST be ≤160 characters (tweet-length). Longer copy belongs in a linked doc page, not the panel.
- `title` MUST NOT contain the word "Enterprise" or "Pro" — the operator already knows which tier they're on; repeating the tier is marketing noise.
