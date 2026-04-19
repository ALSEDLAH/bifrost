// OSS fallback — see workspacesView.tsx sibling for the pattern.
import { Building2 } from "lucide-react";
import ContactUsView from "../views/contactUsView";

export function OrganizationSettingsView() {
	return (
		<div className="w-full">
			<ContactUsView
				className="mx-auto min-h-[80vh]"
				testIdPrefix="organization-settings"
				icon={<Building2 className="h-[5.5rem] w-[5.5rem]" strokeWidth={1} />}
				title="Unlock organization settings"
				description="Configure SSO requirements, break-glass access, default retention, and data residency at the organization level. Part of the Bifrost enterprise license."
				readmeLink="https://docs.getbifrost.ai/enterprise/organizations-workspaces"
			/>
		</div>
	);
}
