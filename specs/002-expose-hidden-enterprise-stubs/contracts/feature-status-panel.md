# Contract — `FeatureStatusPanel`

**Feature**: 002-expose-hidden-enterprise-stubs
**Component path (planned)**: `ui/app/enterprise/components/panels/featureStatusPanel.tsx`

## Purpose

A shared React component that replaces `ContactUsView` **inside the enterprise build only**. Every enterprise stub whose audit verdict is `descope → FeatureStatusPanel` renders a `<FeatureStatusPanel />` at its root with feature-specific props.

## Public interface

```tsx
import FeatureStatusPanel, { type FeatureStatusPanelProps } from "@enterprise/components/panels/featureStatusPanel";

// Example: alertChannelsView.tsx (descoped)
export default function AlertChannelsView() {
  return (
    <FeatureStatusPanel
      title="Alert Channels"
      description="Deliver governance events (budget crossings, rate-limit hits, guardrail denials) to Slack, webhooks, or email."
      status="descoped"
      trackingLink={{
        href: "/specs/001-enterprise-parity/spec.md#sr-01-reuse-over-new",
        label: "SR-01 out-of-scope row",
      }}
    />
  );
}
```

## Rendering contract

When mounted, the component MUST render a single root element with:

- `data-testid="feature-status-panel"`
- A visible heading showing `title`
- Visible body text containing `description`
- A visible status badge whose text matches the `status` label (human-readable form: `Descoped`, `Needs own spec`, `Upstream partial`, `Pending implementation`)
- A clearly clickable link to `trackingLink.href` with the text `trackingLink.label`
- If `alternativeRoute` is set, a second link rendered below the primary with a "See instead: <label>" affordance
- If `icon` is set, render it at 56px in the header area; otherwise pick a default icon per status:
  - `descoped` → `Archive` (lucide-react)
  - `needs-own-spec` → `FileText`
  - `upstream-partial` → `SplitSquareHorizontal`
  - `pending-implementation` → `Clock`

## Forbidden content (contract-tested)

The rendered output MUST NOT contain any of these substrings, verified via Playwright DOM assertion after mount:

- `"This feature is a part of the Bifrost enterprise license"`
- `"Contact Us"` (case-insensitive)
- `"Contact sales"` (case-insensitive)
- `"Upgrade to Enterprise"` (case-insensitive)
- `"would love to know"` (the marketing phrasing from `ContactUsView`)

## Accessibility contract

- Root element has `role="region"` and `aria-labelledby` pointing at the heading's id.
- Status badge uses adequate color contrast; status label is also announced via `aria-label` on the badge so screen readers don't depend on color.
- Links have visible focus rings (inherited from the shared `Button`/`Link` components).

## Layout contract

- Renders inside the existing route's container without forcing `min-h-screen`; pages already provide their own layout. Target visual height: ~300-400px on desktop.
- Responsive down to 320px viewport (mobile enterprise admin is rare but supported).

## Testing contract

- **Unit smoke**: Vitest case mounting the component with each `status` value and asserting the forbidden substrings are absent from `page.innerHTML`.
- **Visual smoke**: one Playwright test per distinct status value, covering the default-icon fallback path.
- **Integration**: existing `scripts/check-sc020-enterprise-stubs.sh` evolves (FR-006) to grep the full enterprise component tree; any PR introducing the forbidden `"This feature is a part of the Bifrost enterprise license"` string fails CI.

## Non-goals

- No internationalization in v1 — English copy only. Add i18n when Bifrost UI globalizes overall, not as part of this feature.
- No animations / transitions. The panel is a static state, not a progress indicator.
- No user-actionable buttons (other than the tracking-link). This is an informational surface.
