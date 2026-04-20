// Datadog connector view (spec 008).
//
// v1 is config storage only — the log-forwarding plugin that actually
// consumes these credentials ships in a follow-up spec. The page says so.

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
	getErrorMessage,
	useCreateLogExportConnectorMutation,
	useDeleteLogExportConnectorMutation,
	useGetLogExportConnectorsQuery,
	useUpdateLogExportConnectorMutation,
} from "@/lib/store/apis";
import type { DatadogConnectorConfig, LogExportConnector } from "@/lib/types/logExportConnectors";
import { Info } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";

const DEFAULTS: DatadogConnectorConfig = {
	api_key: "",
	site: "datadoghq.com",
};

function mask(v: string) {
	if (!v) return "";
	if (v.length <= 6) return "••••";
	return `${v.slice(0, 4)}…${v.slice(-2)}`;
}

export default function DatadogConnectorView() {
	const { data, isLoading } = useGetLogExportConnectorsQuery({ type: "datadog" });
	const [create, { isLoading: isCreating }] = useCreateLogExportConnectorMutation();
	const [update, { isLoading: isUpdating }] = useUpdateLogExportConnectorMutation();
	const [remove] = useDeleteLogExportConnectorMutation();

	const existing: LogExportConnector | undefined = data?.connectors?.[0];
	const initial = useMemo<DatadogConnectorConfig>(() => {
		if (!existing) return DEFAULTS;
		try {
			return { ...DEFAULTS, ...(JSON.parse(existing.config) as DatadogConnectorConfig) };
		} catch {
			return DEFAULTS;
		}
	}, [existing]);

	const [name, setName] = useState(existing?.name ?? "Datadog");
	const [form, setForm] = useState<DatadogConnectorConfig>(initial);
	const [enabled, setEnabled] = useState<boolean>(existing?.enabled ?? true);
	useEffect(() => {
		setName(existing?.name ?? "Datadog");
		setForm(initial);
		setEnabled(existing?.enabled ?? true);
	}, [existing, initial]);

	async function handleSave() {
		if (!form.api_key && !existing) {
			toast.error("API key is required");
			return;
		}
		try {
			if (existing) {
				const nextConfig = form.api_key ? form : { ...form, api_key: initial.api_key };
				await update({
					id: existing.id,
					patch: { name, config: nextConfig, enabled },
				}).unwrap();
			} else {
				await create({
					type: "datadog",
					name,
					config: form,
					enabled,
				}).unwrap();
			}
			toast.success("Datadog connector saved");
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	}

	async function handleDelete() {
		if (!existing) return;
		if (!confirm("Delete the Datadog connector configuration?")) return;
		try {
			await remove({ id: existing.id }).unwrap();
			toast.success("Datadog connector removed");
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	}

	return (
		<div className="mx-auto w-full max-w-3xl space-y-4" data-testid="datadog-connector-view">
			<div className="flex items-start justify-between">
				<div>
					<h2 className="text-lg font-semibold tracking-tight">Datadog</h2>
					<p className="text-muted-foreground text-sm">
						Forward Bifrost logs to your Datadog account.
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
					<Label htmlFor="datadog-name">Connector name</Label>
					<Input id="datadog-name" value={name} onChange={(e) => setName(e.target.value)} />
				</div>
				<div className="space-y-1">
					<Label htmlFor="datadog-api-key">API key</Label>
					<Input
						id="datadog-api-key"
						type="password"
						placeholder={existing ? mask(initial.api_key) : "DD_API_KEY"}
						value={form.api_key}
						onChange={(e) => setForm({ ...form, api_key: e.target.value })}
					/>
					{existing ? (
						<p className="text-muted-foreground text-xs">
							Current value: {mask(initial.api_key)} — leave blank to keep.
						</p>
					) : null}
				</div>
				<div className="space-y-1">
					<Label htmlFor="datadog-site">Site</Label>
					<Input
						id="datadog-site"
						placeholder="datadoghq.com"
						value={form.site}
						onChange={(e) => setForm({ ...form, site: e.target.value })}
					/>
					<p className="text-muted-foreground text-xs">
						e.g. <code>datadoghq.com</code>, <code>eu.datadoghq.com</code>, <code>us3.datadoghq.com</code>
					</p>
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
