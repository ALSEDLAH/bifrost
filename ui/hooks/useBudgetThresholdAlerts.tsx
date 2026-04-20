// Subscribes to budget.threshold.crossed WebSocket events (US8, T055)
// and surfaces them as toasts. Dedup on the wire already happens in
// plugins/governance/tracker_thresholds.go — this hook trusts the
// server and shows every event it receives.

import { useEffect } from "react";
import { toast } from "sonner";
import { useWebSocket } from "./useWebSocket";

interface ThresholdCrossedEvent {
	level?: number;
	budget_id?: string;
	virtual_key?: string;
	team_id?: string;
	customer_id?: string;
	provider?: string;
	current_usage?: number;
	max_limit?: number;
	reset_duration?: string;
}

function formatUsd(n?: number): string {
	if (typeof n !== "number" || isNaN(n)) return "—";
	return `$${n.toFixed(2)}`;
}

export function useBudgetThresholdAlerts() {
	const { subscribe } = useWebSocket();

	useEffect(() => {
		const unsub = subscribe("budget.threshold.crossed", (raw) => {
			const ev = (raw ?? {}) as ThresholdCrossedEvent;
			const level = typeof ev.level === "number" ? ev.level : null;
			if (level === null) return;

			const scope =
				ev.virtual_key ? `VK ${ev.virtual_key.slice(0, 8)}…`
				: ev.team_id ? `Team ${ev.team_id.slice(0, 8)}…`
				: ev.customer_id ? `Customer ${ev.customer_id.slice(0, 8)}…`
				: "Budget";

			const title = `${scope} reached ${level}% of budget`;
			const description = `${formatUsd(ev.current_usage)} of ${formatUsd(ev.max_limit)}` +
				(ev.reset_duration ? ` — resets every ${ev.reset_duration}` : "");

			if (level >= 90) {
				toast.error(title, { description, duration: 10000 });
			} else if (level >= 75) {
				toast.warning(title, { description, duration: 8000 });
			} else {
				toast.info(title, { description, duration: 6000 });
			}
		});
		return unsub;
	}, [subscribe]);
}
