// Large Payload Settings fragment (spec 006).
//
// Renders six numeric inputs + an enable toggle for the large-payload
// thresholds. Parent `clientSettingsView` owns the state and the save
// mutation. RequestThresholdBytes is wired end-to-end (the save handler
// updates `lib.Config.StreamingDecompressThreshold` immediately); the
// other four fields are persisted and read by a future per-request
// middleware — see specs/006-large-payload-settings/spec.md FR-004.

import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import type { LargePayloadSettingsFragmentProps } from "../../../_fallbacks/enterprise/components/large-payload/largePayloadSettingsFragment";

type NumericFieldKey = Exclude<
	keyof import("../../../_fallbacks/enterprise/lib/types/largePayload").LargePayloadConfig,
	"enabled"
>;

interface RowSpec {
	key: NumericFieldKey;
	label: string;
	helper: string;
	live: boolean;
}

const ROWS: RowSpec[] = [
	{
		key: "request_threshold_bytes",
		label: "Request threshold (bytes)",
		helper: "Requests larger than this stream-decompress instead of buffering.",
		live: true,
	},
	{
		key: "response_threshold_bytes",
		label: "Response threshold (bytes)",
		helper: "Responses larger than this stream directly to the client.",
		live: false,
	},
	{
		key: "prefetch_size_bytes",
		label: "Prefetch size (bytes)",
		helper: "Bytes to buffer from a large streaming response for metadata extraction.",
		live: false,
	},
	{
		key: "max_payload_bytes",
		label: "Max payload (bytes)",
		helper: "Hard cap; requests/responses larger than this are rejected.",
		live: false,
	},
	{
		key: "truncated_log_bytes",
		label: "Truncated log (bytes)",
		helper: "Audit log payloads are truncated to this size.",
		live: false,
	},
];

export default function LargePayloadSettingsFragment({
	config,
	onConfigChange,
	controlsDisabled,
}: LargePayloadSettingsFragmentProps) {
	function setEnabled(value: boolean) {
		onConfigChange({ ...config, enabled: value });
	}
	function setNumeric(key: NumericFieldKey, value: number) {
		onConfigChange({ ...config, [key]: value });
	}

	return (
		<div className="space-y-4 rounded-lg border p-4" data-testid="large-payload-settings">
			<div className="flex items-center justify-between">
				<div className="space-y-0.5">
					<Label htmlFor="large-payload-enabled" className="text-sm font-medium">
						Large Payload Optimization
					</Label>
					<p className="text-muted-foreground text-sm">
						Tune request / response thresholds for handling large inference payloads.
						Fields marked <Badge variant="outline" className="mx-1">live</Badge>
						take effect on the next request; others are persisted and activate once
						the per-request middleware ships in a follow-up spec.
					</p>
				</div>
				<Switch
					id="large-payload-enabled"
					checked={config.enabled}
					disabled={controlsDisabled}
					onCheckedChange={setEnabled}
				/>
			</div>
			<div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
				{ROWS.map((row) => (
					<div key={row.key} className="space-y-1">
						<div className="flex items-center gap-2">
							<Label htmlFor={`large-payload-${row.key}`} className="text-sm font-medium">
								{row.label}
							</Label>
							{row.live ? (
								<Badge variant="outline" className="h-5 text-[10px]">live</Badge>
							) : (
								<Badge variant="secondary" className="h-5 text-[10px]">saved</Badge>
							)}
						</div>
						<Input
							id={`large-payload-${row.key}`}
							type="number"
							min={0}
							inputMode="numeric"
							value={config[row.key]}
							disabled={controlsDisabled || !config.enabled}
							onChange={(e) => setNumeric(row.key, Number(e.target.value || 0))}
						/>
						<p className="text-muted-foreground text-xs">{row.helper}</p>
					</div>
				))}
			</div>
		</div>
	);
}
