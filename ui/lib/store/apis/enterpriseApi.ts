// RTK Query endpoints for enterprise Admin API.
//
// Matches the contract at
// specs/001-enterprise-parity/contracts/admin-api.openapi.yaml.
//
// Cache tags:
//   "EnterpriseOrganization" — GET /v1/admin/organizations/current
//   "EnterpriseWorkspaces"   — GET /v1/admin/workspaces (list)
//   { type: "EnterpriseWorkspaces", id }  — per-workspace cache

import type {
	CreateWorkspaceRequest,
	EnterpriseOrganization,
	EnterpriseWorkspace,
	PatchWorkspaceRequest,
	UpdateOrganizationRequest,
} from "@/lib/types/enterprise";
import { baseApi } from "./baseApi";

export const enterpriseApi = baseApi.injectEndpoints({
	endpoints: (builder) => ({
		// ─── organizations ────────────────────────────────────────────
		getCurrentOrganization: builder.query<EnterpriseOrganization, void>({
			query: () => "/admin/organizations/current",
			providesTags: ["EnterpriseOrganization"],
		}),

		updateCurrentOrganization: builder.mutation<EnterpriseOrganization, UpdateOrganizationRequest>({
			query: (body) => ({
				url: "/admin/organizations/current",
				method: "PATCH",
				body,
			}),
			invalidatesTags: ["EnterpriseOrganization"],
		}),

		// ─── workspaces ───────────────────────────────────────────────
		getWorkspaces: builder.query<EnterpriseWorkspace[], void>({
			query: () => "/admin/workspaces",
			providesTags: (result) =>
				result
					? [
							...result.map((w) => ({ type: "EnterpriseWorkspaces" as const, id: w.id })),
							{ type: "EnterpriseWorkspaces" as const, id: "LIST" },
						]
					: [{ type: "EnterpriseWorkspaces" as const, id: "LIST" }],
		}),

		getWorkspace: builder.query<EnterpriseWorkspace, string>({
			query: (id) => `/admin/workspaces/${id}`,
			providesTags: (result, error, id) => [{ type: "EnterpriseWorkspaces", id }],
		}),

		createWorkspace: builder.mutation<EnterpriseWorkspace, CreateWorkspaceRequest>({
			query: (body) => ({
				url: "/admin/workspaces",
				method: "POST",
				body,
			}),
			invalidatesTags: [{ type: "EnterpriseWorkspaces", id: "LIST" }],
		}),

		patchWorkspace: builder.mutation<EnterpriseWorkspace, { id: string; body: PatchWorkspaceRequest }>({
			query: ({ id, body }) => ({
				url: `/admin/workspaces/${id}`,
				method: "PATCH",
				body,
			}),
			invalidatesTags: (_res, _err, { id }) => [
				{ type: "EnterpriseWorkspaces", id },
				{ type: "EnterpriseWorkspaces", id: "LIST" },
			],
		}),

		deleteWorkspace: builder.mutation<void, string>({
			query: (id) => ({
				url: `/admin/workspaces/${id}`,
				method: "DELETE",
			}),
			invalidatesTags: (_res, _err, id) => [
				{ type: "EnterpriseWorkspaces", id },
				{ type: "EnterpriseWorkspaces", id: "LIST" },
			],
		}),
	}),
	overrideExisting: false,
});

export const {
	useGetCurrentOrganizationQuery,
	useUpdateCurrentOrganizationMutation,
	useGetWorkspacesQuery,
	useGetWorkspaceQuery,
	useCreateWorkspaceMutation,
	usePatchWorkspaceMutation,
	useDeleteWorkspaceMutation,
} = enterpriseApi;
