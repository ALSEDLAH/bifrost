// Type surface for the /api/alert-channels endpoints (spec 004).

export type AlertChannelType = "webhook" | "slack";

export interface WebhookConfig {
	url: string;
	method?: "POST" | "PUT";
	headers?: Record<string, string>;
}

export interface SlackConfig {
	webhook_url: string;
}

export interface AlertChannel {
	id: string;
	name: string;
	type: AlertChannelType;
	// Raw JSON; the UI parses into WebhookConfig | SlackConfig based on type.
	config: string;
	enabled: boolean;
	created_at: string;
	updated_at: string;
}

export interface AlertChannelCreatePayload {
	name: string;
	type: AlertChannelType;
	config: WebhookConfig | SlackConfig;
	enabled?: boolean;
}

export interface AlertChannelUpdatePayload {
	name?: string;
	type?: AlertChannelType;
	config?: WebhookConfig | SlackConfig;
	enabled?: boolean;
}
