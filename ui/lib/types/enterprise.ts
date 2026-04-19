// TypeScript types for the enterprise Admin API surface
// (organizations + workspaces). Matches the OpenAPI contract at
// specs/001-enterprise-parity/contracts/admin-api.openapi.yaml.
//
// Each enterprise feature train (A..E) adds its own section below.

// ─────────────────────────────────────────────────────────────────────
// US1 — Organizations & Workspaces
// ─────────────────────────────────────────────────────────────────────

export interface EnterpriseOrganization {
	id: string;
	name: string;
	is_default: boolean;
	sso_required: boolean;
	break_glass_enabled: boolean;
	default_retention_days: number;
	data_residency_region: string;
	created_at: string; // ISO 8601
	// Convenience flag returned by GET /current — mirrors the
	// deployment-mode default.
	multi_org_enabled?: boolean;
	updated_at?: string;
}

export interface UpdateOrganizationRequest {
	name?: string;
	sso_required?: boolean;
	break_glass_enabled?: boolean;
	default_retention_days?: number;
}

export interface EnterpriseWorkspace {
	id: string;
	organization_id: string;
	name: string;
	slug: string;
	description?: string;
	log_retention_days?: number | null;
	metric_retention_days?: number | null;
	payload_encryption_enabled: boolean;
	created_at: string;
	updated_at: string;
}

export interface CreateWorkspaceRequest {
	name: string;
	slug: string;
	description?: string;
}

export interface PatchWorkspaceRequest {
	name?: string;
	description?: string;
	log_retention_days?: number;
	metric_retention_days?: number;
	payload_encryption_enabled?: boolean;
}
