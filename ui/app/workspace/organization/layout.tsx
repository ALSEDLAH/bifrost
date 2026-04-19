import { createFileRoute } from "@tanstack/react-router";
import OrganizationSettingsPage from "./page";

export const Route = createFileRoute("/workspace/organization")({
	component: OrganizationSettingsPage,
});
