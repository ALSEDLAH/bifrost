// Enterprise stub for SCIM 2.0 Provisioning (spec 002, US2).
// Audit verdict: descoped — see specs/002-expose-hidden-enterprise-stubs/research.md.

import FeatureStatusPanel from "@enterprise/components/panels/featureStatusPanel";

export default function StubView() {
	return (
		<FeatureStatusPanel
			title="SCIM 2.0 Provisioning"
			description="Automated user/group lifecycle from external identity providers. Requires an RFC 7644 handler."
			status="descoped"
			trackingLink={{
				href: "/specs/001-enterprise-parity/spec.md#sr-01-reuse-over-new",
				label: "SR-01 · US20 row",
			}}
		/>
	);
}
