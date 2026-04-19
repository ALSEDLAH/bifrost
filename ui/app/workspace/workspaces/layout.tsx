import { createFileRoute } from "@tanstack/react-router";
import WorkspacesListPage from "./page";

export const Route = createFileRoute("/workspace/workspaces")({
	component: WorkspacesListPage,
});
