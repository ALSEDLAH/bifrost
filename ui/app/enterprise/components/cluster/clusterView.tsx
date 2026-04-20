// Enterprise stub for Cluster Management (spec 002, US2).
// Audit verdict: needs-own-spec — see specs/002-expose-hidden-enterprise-stubs/research.md.

import FeatureStatusPanel from "@enterprise/components/panels/featureStatusPanel";

export default function StubView() {
	return (
		<FeatureStatusPanel
			title="Cluster Management"
			description="Node registration, health, and coordination across a Bifrost cluster. Requires a cluster-registry plugin that doesn't exist upstream yet."
			status="needs-own-spec"
			trackingLink={{
				href: "/specs/001-enterprise-parity/spec.md#sr-01-reuse-over-new",
				label: "SR-01 · US19 row",
			}}
		/>
	);
}
