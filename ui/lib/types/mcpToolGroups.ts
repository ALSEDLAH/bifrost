// Types for /api/mcp/tool-groups (spec 005).

export interface MCPToolRef {
	mcp_client_id: string;
	tool_name: string;
}

export interface MCPToolGroup {
	id: string;
	name: string;
	description: string;
	// JSON-encoded string of MCPToolRef[] (backend stores as text column).
	tools: string;
	created_at: string;
	updated_at: string;
}

export function parseToolRefs(group: MCPToolGroup): MCPToolRef[] {
	try {
		const parsed = JSON.parse(group.tools) as unknown;
		if (!Array.isArray(parsed)) return [];
		return parsed as MCPToolRef[];
	} catch {
		return [];
	}
}

export interface MCPToolGroupCreatePayload {
	name: string;
	description?: string;
	tools: MCPToolRef[];
}

export interface MCPToolGroupUpdatePayload {
	name?: string;
	description?: string;
	tools?: MCPToolRef[];
}
