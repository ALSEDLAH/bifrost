// Types for /api/adaptive-routing/status (spec 012 phase 1).

export type CircuitState = "closed" | "open" | "half-open";

export interface AdaptiveRoutingEntry {
	provider: string;
	model: string;
	circuit_state: CircuitState;
	ewma_latency_ms: number;
	success_rate: number;
	total_requests: number;
	window_started_at: string;
	window_ended_at: string;
	last_sample_entered: string;
}

export interface AdaptiveRoutingStatus {
	providers: AdaptiveRoutingEntry[];
	window_duration: string;
	refresh_interval: string;
	last_refresh_at: string;
	logstore_present: boolean;
}
