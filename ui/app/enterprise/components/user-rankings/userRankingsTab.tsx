// Enterprise stub for User Rankings (spec 002, US2).
// Audit verdict: descoped — see specs/002-expose-hidden-enterprise-stubs/research.md.

import FeatureStatusPanel from "@enterprise/components/panels/featureStatusPanel";

export default function StubView() {
	return (
		<FeatureStatusPanel
			title="User Rankings"
			description="Top-users dashboard with requests, tokens, and cost per user. Needs a per-user aggregation endpoint upstream."
			status="descoped"
			trackingLink={{
				href: "/specs/001-enterprise-parity/spec.md#sr-01-reuse-over-new",
				label: "SR-01 · US12 row",
			}}
		/>
	);
}
