// Enterprise stub for Guardrail Providers (spec 002, US2).
// Audit verdict: needs-own-spec — see specs/002-expose-hidden-enterprise-stubs/research.md.

import FeatureStatusPanel from "@enterprise/components/panels/featureStatusPanel";

export default function StubView() {
	return (
		<FeatureStatusPanel
			title="Guardrail Providers"
			description="Credentials and connection config for partner guardrail providers (Aporia, Patronus, Pillar, etc.). Depends on the guardrails-central plugin."
			status="needs-own-spec"
			trackingLink={{
				href: "/specs/001-enterprise-parity/spec.md#sr-01-reuse-over-new",
				label: "SR-01 · US6 row",
			}}
		/>
	);
}
