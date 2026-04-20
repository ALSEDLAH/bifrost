// Enterprise stub for Prompt Deployments (spec 002, US2).
// Audit verdict: descoped — see specs/002-expose-hidden-enterprise-stubs/research.md.

import FeatureStatusPanel from "@enterprise/components/panels/featureStatusPanel";

export default function StubView() {
	return (
		<FeatureStatusPanel
			title="Prompt Deployments"
			description="A/B splitting, canary rollouts, and a production-version pointer for prompts. Extends the existing prompts plugin; needs its own spec."
			status="descoped"
			trackingLink={{
				href: "/specs/001-enterprise-parity/spec.md#sr-01-reuse-over-new",
				label: "SR-01 · US14-deployments row",
			}}
		/>
	);
}
