// Types for /api/scim/config (spec 009).

export interface SCIMConfig {
	enabled: boolean;
	endpoint_url: string;
	token_prefix?: string;
	token_created_at?: string;
}

export interface SCIMRotateResponse {
	token: string;
	token_prefix: string;
	token_created_at: string;
	enabled: boolean;
}
