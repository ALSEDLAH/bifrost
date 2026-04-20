// Enterprise stub for Workspaces (spec 002, US2).
// Audit verdict: upstream-partial — see specs/002-expose-hidden-enterprise-stubs/research.md.

import FeatureStatusPanel from "@enterprise/components/panels/featureStatusPanel";

export default function StubView() {
	return (
		<FeatureStatusPanel
			title="Workspaces"
			description="Workspace management is the Teams page in governance. This route is retained for alias resolution only."
			status="upstream-partial"
			trackingLink={{
				href: "/specs/001-enterprise-parity/spec.md#sr-01-reuse-over-new",
				label: "SR-01 · US1 row",
			}}
			alternativeRoute={{
				href: "/workspace/governance/teams",
				label: "Teams",
			}}
		/>
	);
}
