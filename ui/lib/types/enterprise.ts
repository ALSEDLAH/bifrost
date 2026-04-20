// TypeScript types for enterprise-specific entities.
//
// Org/workspace management uses governance types (Customer, Team) from
// the existing governance API. This file holds types for enterprise-only
// entities that don't exist in the governance layer.

// ---- RBAC (US2) --------------------------------------------------

export interface RbacMeta {
	resources: string[];
	operations: string[];
	builtin_roles: string[];
}

export interface RbacMe {
	organization_id: string;
	workspace_id?: string;
	user_id?: string;
	email?: string;
	display_name?: string;
	scopes: string[];
	permissions: Record<string, string[]>;
}

export type RoleScopeMap = Record<string, string[]>;

export interface EnterpriseRole {
	id: string;
	organization_id: string;
	name: string;
	scopes: RoleScopeMap | null;
	is_builtin: boolean;
	created_at: string;
}

export interface CreateRoleRequest {
	name: string;
	scopes: RoleScopeMap;
}

export interface UpdateRoleRequest {
	name?: string;
	scopes?: RoleScopeMap;
}

export type EnterpriseUserStatus = "active" | "suspended" | "pending";

export interface EnterpriseRoleAssignment {
	id: string;
	user_id: string;
	role_id: string;
	workspace_id?: string;
	assigned_at: string;
	assigned_by?: string;
}

export interface EnterpriseUser {
	id: string;
	organization_id: string;
	email: string;
	display_name?: string;
	status: EnterpriseUserStatus;
	last_login_at?: string | null;
	created_at: string;
	updated_at: string;
	assignments?: EnterpriseRoleAssignment[] | null;
}

export interface CreateUserRequest {
	email: string;
	display_name?: string;
	status?: EnterpriseUserStatus;
}

export interface UpdateUserRequest {
	display_name?: string;
	status?: EnterpriseUserStatus;
}

export interface AssignRoleRequest {
	user_id: string;
	role_id: string;
	workspace_id?: string;
}

// ---- Audit Logs (US4) --------------------------------------------

export type AuditOutcome = "allowed" | "denied" | "error";

export interface AuditEntry {
	id: string;
	organization_id: string;
	workspace_id?: string;
	actor_type: string;
	actor_id?: string;
	actor_display: string;
	actor_ip?: string;
	action: string;
	resource_type: string;
	resource_id?: string;
	outcome: AuditOutcome;
	reason?: string;
	before_json?: string;
	after_json?: string;
	request_id?: string;
	created_at: string;
}

export interface AuditLogFilters {
	actor_id?: string;
	action?: string;
	resource_type?: string;
	outcome?: AuditOutcome;
	from?: string;
	to?: string;
	organization_id?: string;
	limit?: number;
	offset?: number;
}

export interface GetAuditLogsResponse {
	entries: AuditEntry[];
	total: number;
	limit: number;
	offset: number;
}

