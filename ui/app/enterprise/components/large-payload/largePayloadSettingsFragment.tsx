// Enterprise stub for Large Payload Settings (spec 002, US2).
// Audit verdict: upstream-partial — see specs/002-expose-hidden-enterprise-stubs/research.md.
//
// Preserves the original fragment signature so callers like
// clientSettingsView.tsx compile unchanged; props are ignored because
// the runtime settings endpoint is out of v1 scope.

import FeatureStatusPanel from "@enterprise/components/panels/featureStatusPanel";
import type { LargePayloadSettingsFragmentProps } from "../../../_fallbacks/enterprise/components/large-payload/largePayloadSettingsFragment";

export default function LargePayloadSettingsFragment(_props: LargePayloadSettingsFragmentProps) {
	return (
		<FeatureStatusPanel
			title="Large Payload Settings"
			description="Streaming threshold is configured at deploy time via Config.StreamingDecompressThreshold, not per-workspace. A runtime settings endpoint would be net-new."
			status="upstream-partial"
			trackingLink={{
				href: "/specs/001-enterprise-parity/spec.md#sr-01-reuse-over-new",
				label: "SR-01 · US30 T076 row",
			}}
		/>
	);
}
