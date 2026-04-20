// Enterprise stub for Central Guardrails (spec 002, US2).
// Audit verdict: needs-own-spec — see specs/002-expose-hidden-enterprise-stubs/research.md.

import FeatureStatusPanel from "@enterprise/components/panels/featureStatusPanel";

export default function StubView() {
	return (
		<FeatureStatusPanel
			title="Central Guardrails"
			description="Organization-wide guardrail policies (regex, LLM-based, partner-integrated). Requires a guardrails-central plugin that doesn't exist upstream yet."
			status="needs-own-spec"
			trackingLink={{
				href: "/specs/001-enterprise-parity/spec.md#sr-01-reuse-over-new",
				label: "SR-01 · US6 row",
			}}
		/>
	);
}
