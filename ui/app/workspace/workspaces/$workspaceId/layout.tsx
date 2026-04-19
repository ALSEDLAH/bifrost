import { createFileRoute } from "@tanstack/react-router";
import WorkspaceDetailPage from "./page";

export const Route = createFileRoute("/workspace/workspaces/$workspaceId")({
	component: WorkspaceDetailPage,
});
