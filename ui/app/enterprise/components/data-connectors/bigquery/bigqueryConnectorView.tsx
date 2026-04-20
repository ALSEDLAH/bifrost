// BigQuery connector view (spec 008).
//
// v1 is config storage only — the log-forwarding plugin that actually
// consumes these credentials ships in a follow-up spec.

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import {
	getErrorMessage,
	useCreateLogExportConnectorMutation,
	useDeleteLogExportConnectorMutation,
	useGetLogExportConnectorsQuery,
	useUpdateLogExportConnectorMutation,
} from "@/lib/store/apis";
import type { BigQueryConnectorConfig, LogExportConnector } from "@/lib/types/logExportConnectors";
import { Info } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";

const DEFAULTS: BigQueryConnectorConfig = {
	project_id: "",
	dataset: "bifrost_logs",
	table: "inference_logs",
	credentials_json: "",
};

function summary(v: string) {
	if (!v) return "";
	try {
		const obj = JSON.parse(v) as { client_email?: string };
		return obj.client_email ? `service-account: ${obj.client_email}` : "credentials JSON stored";
	} catch {
		return "credentials JSON stored";
	}
}

export default function BigQueryConnectorView() {
	const { data, isLoading } = useGetLogExportConnectorsQuery({ type: "bigquery" });
	const [create, { isLoading: isCreating }] = useCreateLogExportConnectorMutation();
	const [update, { isLoading: isUpdating }] = useUpdateLogExportConnectorMutation();
	const [remove] = useDeleteLogExportConnectorMutation();

	const existing: LogExportConnector | undefined = data?.connectors?.[0];
	const initial = useMemo<BigQueryConnectorConfig>(() => {
		if (!existing) return DEFAULTS;
		try {
			return { ...DEFAULTS, ...(JSON.parse(existing.config) as BigQueryConnectorConfig) };
		} catch {
			return DEFAULTS;
		}
	}, [existing]);

	const [name, setName] = useState(existing?.name ?? "BigQuery");
	const [form, setForm] = useState<BigQueryConnectorConfig>(initial);
	const [enabled, setEnabled] = useState<boolean>(existing?.enabled ?? true);
	useEffect(() => {
		setName(existing?.name ?? "BigQuery");
		setForm(initial);
		setEnabled(existing?.enabled ?? true);
	}, [existing, initial]);

	async function handleSave() {
		if (!form.project_id || !form.dataset || !form.table) {
			toast.error("project_id, dataset, and table are required");
			return;
		}
		if (!existing && !form.credentials_json) {
			toast.error("Service-account credentials JSON is required");
			return;
		}
		try {
			if (existing) {
				const nextConfig = form.credentials_json
					? form
					: { ...form, credentials_json: initial.credentials_json };
				await update({
					id: existing.id,
					patch: { name, config: nextConfig, enabled },
				}).unwrap();
			} else {
				await create({
					type: "bigquery",
					name,
					config: form,
					enabled,
				}).unwrap();
			}
			toast.success("BigQuery connector saved");
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	}

	async function handleDelete() {
		if (!existing) return;
		if (!confirm("Delete the BigQuery connector configuration?")) return;
		try {
			await remove({ id: existing.id }).unwrap();
			toast.success("BigQuery connector removed");
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	}

	return (
		<div className="mx-auto w-full max-w-3xl space-y-4" data-testid="bigquery-connector-view">
			<div className="flex items-start justify-between">
				<div>
					<h2 className="text-lg font-semibold tracking-tight">BigQuery</h2>
					<p className="text-muted-foreground text-sm">
						Stream Bifrost logs into a BigQuery table.
						<Badge variant="secondary" className="ml-2 h-5 text-[10px]">config only</Badge>
					</p>
				</div>
				<Switch
					checked={enabled}
					onCheckedChange={setEnabled}
					aria-label="Enable connector"
				/>
			</div>

			<Alert variant="default" className="border-blue-20">
				<Info className="h-4 w-4 text-blue-600" />
				<AlertDescription>
					Credentials are stored but log forwarding activates once the enterprise log-export plugin ships (tracked in spec 008 phase 2).
				</AlertDescription>
			</Alert>

			<div className="space-y-3 rounded-lg border p-4">
				<div className="space-y-1">
					<Label htmlFor="bq-name">Connector name</Label>
					<Input id="bq-name" value={name} onChange={(e) => setName(e.target.value)} />
				</div>
				<div className="grid grid-cols-1 gap-3 md:grid-cols-3">
					<div className="space-y-1">
						<Label htmlFor="bq-project">Project ID</Label>
						<Input
							id="bq-project"
							value={form.project_id}
							onChange={(e) => setForm({ ...form, project_id: e.target.value })}
						/>
					</div>
					<div className="space-y-1">
						<Label htmlFor="bq-dataset">Dataset</Label>
						<Input
							id="bq-dataset"
							value={form.dataset}
							onChange={(e) => setForm({ ...form, dataset: e.target.value })}
						/>
					</div>
					<div className="space-y-1">
						<Label htmlFor="bq-table">Table</Label>
						<Input
							id="bq-table"
							value={form.table}
							onChange={(e) => setForm({ ...form, table: e.target.value })}
						/>
					</div>
				</div>
				<div className="space-y-1">
					<Label htmlFor="bq-creds">Service-account credentials JSON</Label>
					<Textarea
						id="bq-creds"
						rows={6}
						placeholder={
							existing
								? `Leave blank to keep — ${summary(initial.credentials_json)}`
								: `{\n  "type": "service_account",\n  "project_id": "...",\n  ...\n}`
						}
						value={form.credentials_json}
						onChange={(e) => setForm({ ...form, credentials_json: e.target.value })}
					/>
					{existing ? (
						<p className="text-muted-foreground text-xs">Current: {summary(initial.credentials_json)}</p>
					) : null}
				</div>
			</div>

			<div className="flex justify-between">
				<Button
					variant="outline"
					size="sm"
					onClick={handleDelete}
					disabled={!existing || isLoading}
				>
					Remove
				</Button>
				<Button onClick={handleSave} disabled={isCreating || isUpdating}>
					{existing ? "Save changes" : "Create connector"}
				</Button>
			</div>
		</div>
	);
}
