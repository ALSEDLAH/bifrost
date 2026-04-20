// Enterprise stub for Alert Channels (spec 002, US2).
// Audit verdict: descoped — see specs/002-expose-hidden-enterprise-stubs/research.md.

import FeatureStatusPanel from "@enterprise/components/panels/featureStatusPanel";

export default function StubView() {
	return (
		<FeatureStatusPanel
			title="Alert Channels"
			description="Deliver governance events (budget crossings, rate-limit hits, guardrail denials) to Slack, webhooks, or email. Needs its own feature spec to add the alerts plugin."
			status="descoped"
			trackingLink={{
				href: "/specs/001-enterprise-parity/spec.md#sr-01-reuse-over-new",
				label: "SR-01 · US10 row",
			}}
		/>
	);
}
