// Enterprise stub for BigQuery Log Export (spec 002, US2).
// Audit verdict: descoped — see specs/002-expose-hidden-enterprise-stubs/research.md.

import FeatureStatusPanel from "@enterprise/components/panels/featureStatusPanel";

export default function StubView() {
	return (
		<FeatureStatusPanel
			title="BigQuery Log Export"
			description="Stream Bifrost request logs to BigQuery. Needs its own spec to add the logexport plugin + BigQuery sink."
			status="descoped"
			trackingLink={{
				href: "/specs/001-enterprise-parity/spec.md#sr-01-reuse-over-new",
				label: "SR-01 · US11 row",
			}}
		/>
	);
}
