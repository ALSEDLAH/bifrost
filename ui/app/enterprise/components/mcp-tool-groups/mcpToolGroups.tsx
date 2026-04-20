// Enterprise stub for MCP Tool Groups (spec 002, US2).
// Audit verdict: descoped — see specs/002-expose-hidden-enterprise-stubs/research.md.

import FeatureStatusPanel from "@enterprise/components/panels/featureStatusPanel";

export default function StubView() {
	return (
		<FeatureStatusPanel
			title="MCP Tool Groups"
			description="Group MCP tools for granular access control per team or virtual key. Upstream MCP clients have no grouping column; this is a net-new concept."
			status="descoped"
			trackingLink={{
				href: "/specs/001-enterprise-parity/spec.md#sr-01-reuse-over-new",
				label: "SR-01 · US30 T074 row",
			}}
		/>
	);
}
