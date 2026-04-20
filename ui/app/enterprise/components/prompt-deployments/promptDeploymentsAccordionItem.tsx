// Enterprise stub for Prompt Deployment Accordion Item (spec 002, US2).
// Audit verdict: descoped — see specs/002-expose-hidden-enterprise-stubs/research.md.

import FeatureStatusPanel from "@enterprise/components/panels/featureStatusPanel";

export type SettingsSidebarSection = "parameters" | "deployments";

function PromptDeploymentsAccordionItem(_props: { activeSection: SettingsSidebarSection | undefined }) {
	return (
		<FeatureStatusPanel
			title="Prompt Deployment Accordion Item"
			description="Accordion companion for the Prompt Deployments editor. Shipping alongside that feature."
			status="descoped"
			trackingLink={{
				href: "/specs/001-enterprise-parity/spec.md#sr-01-reuse-over-new",
				label: "SR-01 · US14-deployments row",
			}}
		/>
	);
}

// Preserve both the default export and the named export that the
// existing caller (components/prompts/fragments/settingsPanel.tsx)
// imports. Harmless — same component either way.
export { PromptDeploymentsAccordionItem };
export default PromptDeploymentsAccordionItem;
