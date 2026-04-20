// Enterprise stub for Enterprise SSO Login (spec 002, US2).
// Audit verdict: needs-own-spec — see specs/002-expose-hidden-enterprise-stubs/research.md.

import FeatureStatusPanel from "@enterprise/components/panels/featureStatusPanel";

export default function StubView() {
	return (
		<FeatureStatusPanel
			title="Enterprise SSO Login"
			description="Sign-in page for SAML/OIDC identity providers. Requires an SSO handler and identity-provider integration work."
			status="needs-own-spec"
			trackingLink={{
				href: "/specs/001-enterprise-parity/spec.md#sr-01-reuse-over-new",
				label: "SR-01 · US3 row",
			}}
		/>
	);
}
