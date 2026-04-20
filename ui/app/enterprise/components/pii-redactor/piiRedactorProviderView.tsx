// Enterprise stub for PII Redactor Providers (spec 002, US2).
// Audit verdict: descoped — see specs/002-expose-hidden-enterprise-stubs/research.md.

import FeatureStatusPanel from "@enterprise/components/panels/featureStatusPanel";

export default function StubView() {
	return (
		<FeatureStatusPanel
			title="PII Redactor Providers"
			description="Credentials for PII detection providers (Presidio, partner services). Depends on the pii-redactor plugin."
			status="descoped"
			trackingLink={{
				href: "/specs/001-enterprise-parity/spec.md#sr-01-reuse-over-new",
				label: "SR-01 · US7 row",
			}}
		/>
	);
}
