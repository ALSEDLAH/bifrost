// Enterprise API extensions for RTK Query.
//
// The primary org/workspace/VK management uses the existing governance
// API hooks (useGetTeamsQuery, useGetCustomersQuery, etc.) from
// governanceApi.ts. This file extends for enterprise-only endpoints
// that don't exist in the governance handler (RBAC roles, users,
// assignments, admin API keys, etc.).
//
// Cache tags are registered in baseApi.ts.

import {
	AssignRoleRequest,
	AuditLogFilters,
	CreateRoleRequest,
	CreateUserRequest,
	EnterpriseRole,
	EnterpriseRoleAssignment,
	EnterpriseUser,
	GetAuditLogsResponse,
	RbacMe,
	RbacMeta,
	UpdateRoleRequest,
	UpdateUserRequest,
} from "@/lib/types/enterprise";
import { baseApi } from "./baseApi";

function auditQueryParams(f: AuditLogFilters | void): Record<string, string | number> {
	const p: Record<string, string | number> = {};
	if (!f) return p;
	if (f.actor_id) p.actor_id = f.actor_id;
	if (f.action) p.action = f.action;
	if (f.resource_type) p.resource_type = f.resource_type;
	if (f.outcome) p.outcome = f.outcome;
	if (f.from) p.from = f.from;
	if (f.to) p.to = f.to;
	if (f.organization_id) p.organization_id = f.organization_id;
	if (f.limit !== undefined) p.limit = f.limit;
	if (f.offset !== undefined) p.offset = f.offset;
	return p;
}

export const enterpriseApi = baseApi.injectEndpoints({
	endpoints: (builder) => ({
		// ---- RBAC Meta / Me ----
		getRbacMeta: builder.query<RbacMeta, void>({
			query: () => "/rbac/meta",
		}),
		getRbacMe: builder.query<RbacMe, void>({
			query: () => "/rbac/me",
			providesTags: ["Permissions"],
		}),

		// ---- Roles ----
		getRoles: builder.query<{ roles: EnterpriseRole[]; total: number }, void>({
			query: () => "/rbac/roles",
			providesTags: ["Roles"],
		}),
		createRole: builder.mutation<{ role: EnterpriseRole }, CreateRoleRequest>({
			query: (body) => ({ url: "/rbac/roles", method: "POST", body }),
			invalidatesTags: ["Roles"],
		}),
		updateRole: builder.mutation<{ role: EnterpriseRole }, { id: string; data: UpdateRoleRequest }>({
			query: ({ id, data }) => ({ url: `/rbac/roles/${id}`, method: "PATCH", body: data }),
			invalidatesTags: ["Roles"],
		}),
		deleteRole: builder.mutation<{ deleted: boolean }, string>({
			query: (id) => ({ url: `/rbac/roles/${id}`, method: "DELETE" }),
			invalidatesTags: ["Roles"],
		}),

		// ---- Users ----
		getEnterpriseUsers: builder.query<{ users: EnterpriseUser[]; total: number }, void>({
			query: () => "/rbac/users",
			providesTags: ["Users"],
		}),
		createEnterpriseUser: builder.mutation<{ user: EnterpriseUser }, CreateUserRequest>({
			query: (body) => ({ url: "/rbac/users", method: "POST", body }),
			invalidatesTags: ["Users"],
		}),
		updateEnterpriseUser: builder.mutation<{ user: EnterpriseUser }, { id: string; data: UpdateUserRequest }>({
			query: ({ id, data }) => ({ url: `/rbac/users/${id}`, method: "PATCH", body: data }),
			invalidatesTags: ["Users"],
		}),
		deleteEnterpriseUser: builder.mutation<{ deleted: boolean }, string>({
			query: (id) => ({ url: `/rbac/users/${id}`, method: "DELETE" }),
			invalidatesTags: ["Users"],
		}),

		// ---- Assignments ----
		getUserAssignments: builder.query<{ assignments: EnterpriseRoleAssignment[]; total: number }, string>({
			query: (userId) => `/rbac/users/${userId}/assignments`,
			providesTags: (_r, _e, id) => [{ type: "Users", id: `${id}:assignments` }],
		}),
		assignRole: builder.mutation<{ assignment: EnterpriseRoleAssignment }, AssignRoleRequest>({
			query: (body) => ({ url: "/rbac/assignments", method: "POST", body }),
			invalidatesTags: ["Users"],
		}),
		unassignRole: builder.mutation<{ deleted: boolean }, string>({
			query: (id) => ({ url: `/rbac/assignments/${id}`, method: "DELETE" }),
			invalidatesTags: ["Users"],
		}),

		// ---- Audit Logs (US4) ----
		getAuditLogs: builder.query<GetAuditLogsResponse, AuditLogFilters | void>({
			query: (filters) => ({ url: "/audit-logs", params: auditQueryParams(filters) }),
			providesTags: ["AuditLogs"],
		}),
	}),
	overrideExisting: false,
});

export function buildAuditExportUrl(format: "csv" | "json", filters?: AuditLogFilters): string {
	const params = auditQueryParams(filters);
	const query = new URLSearchParams({ format, ...Object.fromEntries(Object.entries(params).map(([k, v]) => [k, String(v)])) });
	return `/api/audit-logs/export?${query.toString()}`;
}

export const {
	useGetRbacMetaQuery,
	useGetRbacMeQuery,
	useGetRolesQuery,
	useCreateRoleMutation,
	useUpdateRoleMutation,
	useDeleteRoleMutation,
	useGetEnterpriseUsersQuery,
	useCreateEnterpriseUserMutation,
	useUpdateEnterpriseUserMutation,
	useDeleteEnterpriseUserMutation,
	useGetUserAssignmentsQuery,
	useAssignRoleMutation,
	useUnassignRoleMutation,
	useGetAuditLogsQuery,
} = enterpriseApi;
