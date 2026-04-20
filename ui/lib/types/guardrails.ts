// Types for /api/guardrails (spec 010).

export type GuardrailProviderType = "openai-moderation" | "regex" | "custom-webhook";
export type GuardrailTrigger = "input" | "output" | "both";
export type GuardrailAction = "block" | "flag" | "log";

export interface GuardrailProvider {
	id: string;
	name: string;
	type: GuardrailProviderType;
	config: string; // JSON string
	enabled: boolean;
	created_at: string;
	updated_at: string;
}

export interface GuardrailRule {
	id: string;
	name: string;
	provider_id: string;
	trigger: GuardrailTrigger;
	action: GuardrailAction;
	pattern: string;
	enabled: boolean;
	created_at: string;
	updated_at: string;
}

export interface GuardrailProviderCreatePayload {
	name: string;
	type: GuardrailProviderType;
	config: Record<string, unknown>;
	enabled?: boolean;
}

export interface GuardrailProviderUpdatePayload {
	name?: string;
	config?: Record<string, unknown>;
	enabled?: boolean;
}

export interface GuardrailRuleCreatePayload {
	name: string;
	provider_id?: string;
	trigger: GuardrailTrigger;
	action: GuardrailAction;
	pattern?: string;
	enabled?: boolean;
}

export type GuardrailRuleUpdatePayload = Partial<GuardrailRuleCreatePayload>;
