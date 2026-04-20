// Types for /api/log-export/connectors (spec 008).

export type LogExportConnectorType = "datadog" | "bigquery";

export interface DatadogConnectorConfig {
	api_key: string;
	site: string; // "datadoghq.com" | "eu.datadoghq.com" | ...
	tags?: Record<string, string>;
}

export interface BigQueryConnectorConfig {
	project_id: string;
	dataset: string;
	table: string;
	credentials_json: string; // service-account JSON pasted in
}

export interface LogExportConnector {
	id: string;
	type: LogExportConnectorType;
	name: string;
	// Raw JSON string. Callers parse into DatadogConnectorConfig /
	// BigQueryConnectorConfig based on `type`.
	config: string;
	enabled: boolean;
	created_at: string;
	updated_at: string;
}

export interface LogExportConnectorCreatePayload {
	type: LogExportConnectorType;
	name: string;
	config: DatadogConnectorConfig | BigQueryConnectorConfig;
	enabled?: boolean;
}

export interface LogExportConnectorUpdatePayload {
	name?: string;
	config?: DatadogConnectorConfig | BigQueryConnectorConfig;
	enabled?: boolean;
}
