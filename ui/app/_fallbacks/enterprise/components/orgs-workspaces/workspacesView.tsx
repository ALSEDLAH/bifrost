// OSS fallback — shown when app/enterprise/ is absent (Bifrost OSS build).
// Replaced at build time by the real implementation in
// app/enterprise/components/orgs-workspaces/workspacesView.tsx when the
// enterprise directory exists (see ui/vite.config.mts).
import { Network } from "lucide-react";
import ContactUsView from "../views/contactUsView";

export function WorkspacesView() {
	return (
		<div className="w-full">
			<ContactUsView
				className="mx-auto min-h-[80vh]"
				testIdPrefix="workspaces-orgs"
				icon={<Network className="h-[5.5rem] w-[5.5rem]" strokeWidth={1} />}
				title="Unlock multi-workspace isolation"
				description="Bifrost Enterprise gives you workspace-scoped virtual keys, prompts, configs, guardrails, and logs — so every team's data stays separate under one deployment. Part of the Bifrost enterprise license."
				readmeLink="https://docs.getbifrost.ai/enterprise/organizations-workspaces"
			/>
		</div>
	);
}

export function WorkspaceDetailView() {
	return <WorkspacesView />;
}
