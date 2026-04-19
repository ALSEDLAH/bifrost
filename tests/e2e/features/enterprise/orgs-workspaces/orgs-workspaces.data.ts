// Test-data factory for enterprise orgs-workspaces E2E specs.

export interface WorkspaceData {
	name: string;
	slug: string;
	description?: string;
}

let counter = 0;

export function createWorkspaceData(overrides: Partial<WorkspaceData> = {}): WorkspaceData {
	counter += 1;
	const stamp = Date.now();
	const base: WorkspaceData = {
		name: `E2E Test Workspace ${stamp}-${counter}`,
		slug: `e2e-ws-${stamp}-${counter}`,
		description: "Created by Playwright E2E suite; safe to delete.",
	};
	return { ...base, ...overrides };
}
