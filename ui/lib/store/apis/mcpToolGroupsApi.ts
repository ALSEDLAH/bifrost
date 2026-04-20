import type {
	MCPToolGroup,
	MCPToolGroupCreatePayload,
	MCPToolGroupUpdatePayload,
} from "@/lib/types/mcpToolGroups";
import { baseApi } from "./baseApi";

// RTK surface for /api/mcp/tool-groups (spec 005).

export const mcpToolGroupsApi = baseApi.injectEndpoints({
	endpoints: (builder) => ({
		getMCPToolGroups: builder.query<{ groups: MCPToolGroup[] }, void>({
			query: () => ({ url: "/mcp/tool-groups" }),
			providesTags: ["MCPToolGroups"],
		}),
		createMCPToolGroup: builder.mutation<MCPToolGroup, MCPToolGroupCreatePayload>({
			query: (body) => ({
				url: "/mcp/tool-groups",
				method: "POST",
				body,
			}),
			invalidatesTags: ["MCPToolGroups"],
		}),
		updateMCPToolGroup: builder.mutation<
			MCPToolGroup,
			{ id: string; patch: MCPToolGroupUpdatePayload }
		>({
			query: ({ id, patch }) => ({
				url: `/mcp/tool-groups/${encodeURIComponent(id)}`,
				method: "PATCH",
				body: patch,
			}),
			invalidatesTags: ["MCPToolGroups"],
		}),
		deleteMCPToolGroup: builder.mutation<void, { id: string }>({
			query: ({ id }) => ({
				url: `/mcp/tool-groups/${encodeURIComponent(id)}`,
				method: "DELETE",
			}),
			invalidatesTags: ["MCPToolGroups"],
		}),
	}),
});

export const {
	useGetMCPToolGroupsQuery,
	useCreateMCPToolGroupMutation,
	useUpdateMCPToolGroupMutation,
	useDeleteMCPToolGroupMutation,
} = mcpToolGroupsApi;
