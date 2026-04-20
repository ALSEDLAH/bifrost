// Guardrail Providers view (spec 010 phase 1).

import { NoPermissionView } from "@/components/noPermissionView";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import {
	getErrorMessage,
	useCreateGuardrailProviderMutation,
	useDeleteGuardrailProviderMutation,
	useGetGuardrailProvidersQuery,
	useUpdateGuardrailProviderMutation,
} from "@/lib/store/apis";
import type {
	GuardrailProvider,
	GuardrailProviderType,
} from "@/lib/types/guardrails";
import { RbacOperation, RbacResource, useRbac } from "@enterprise/lib";
import { Info, Pencil, Plus, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";

type DialogState =
	| { kind: "closed" }
	| { kind: "new" }
	| { kind: "edit"; provider: GuardrailProvider };

function parseConfig(p: GuardrailProvider): Record<string, unknown> {
	try {
		return JSON.parse(p.config) as Record<string, unknown>;
	} catch {
		return {};
	}
}

export default function GuardrailsProviderView() {
	const hasView = useRbac(RbacResource.GuardrailsProviders, RbacOperation.View);
	const hasCreate = useRbac(RbacResource.GuardrailsProviders, RbacOperation.Create);
	const hasUpdate = useRbac(RbacResource.GuardrailsProviders, RbacOperation.Update);
	const hasDelete = useRbac(RbacResource.GuardrailsProviders, RbacOperation.Delete);

	const { data, isLoading } = useGetGuardrailProvidersQuery(undefined, { skip: !hasView });
	const [create, { isLoading: isCreating }] = useCreateGuardrailProviderMutation();
	const [update, { isLoading: isUpdating }] = useUpdateGuardrailProviderMutation();
	const [remove] = useDeleteGuardrailProviderMutation();

	const [mode, setMode] = useState<DialogState>({ kind: "closed" });
	const [name, setName] = useState("");
	const [type, setType] = useState<GuardrailProviderType>("openai-moderation");
	const [apiKey, setApiKey] = useState("");
	const [baseURL, setBaseURL] = useState("");
	const [webhookURL, setWebhookURL] = useState("");

	const providers = useMemo(() => data?.providers ?? [], [data]);

	useEffect(() => {
		if (mode.kind === "edit") {
			const cfg = parseConfig(mode.provider);
			setName(mode.provider.name);
			setType(mode.provider.type);
			setApiKey((cfg.api_key as string) ?? "");
			setBaseURL((cfg.base_url as string) ?? "");
			setWebhookURL((cfg.url as string) ?? "");
		} else if (mode.kind === "new") {
			setName("");
			setType("openai-moderation");
			setApiKey("");
			setBaseURL("");
			setWebhookURL("");
		}
	}, [mode]);

	if (!hasView) return <NoPermissionView entity="guardrail providers" />;

	function reset() {
		setMode({ kind: "closed" });
	}

	function buildConfig(): Record<string, unknown> {
		switch (type) {
			case "openai-moderation":
				return { api_key: apiKey, ...(baseURL ? { base_url: baseURL } : {}) };
			case "custom-webhook":
				return { url: webhookURL };
			case "regex":
				return {};
			default:
				return {};
		}
	}

	async function handleSave() {
		if (!name.trim()) {
			toast.error("Name is required");
			return;
		}
		const config = buildConfig();
		try {
			if (mode.kind === "new") {
				await create({ name: name.trim(), type, config }).unwrap();
				toast.success("Guardrail provider created");
			} else if (mode.kind === "edit") {
				await update({
					id: mode.provider.id,
					patch: { name: name.trim(), config },
				}).unwrap();
				toast.success("Guardrail provider updated");
			}
			reset();
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	}

	async function handleToggle(p: GuardrailProvider, enabled: boolean) {
		try {
			await update({ id: p.id, patch: { enabled } }).unwrap();
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	}

	async function handleDelete(p: GuardrailProvider) {
		if (!confirm(`Delete provider "${p.name}"?`)) return;
		try {
			await remove({ id: p.id }).unwrap();
			toast.success("Provider deleted");
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	}

	return (
		<div className="mx-auto w-full max-w-5xl space-y-4" data-testid="guardrails-providers-view">
			<div className="flex items-start justify-between">
				<div>
					<h2 className="text-lg font-semibold tracking-tight">Guardrail Providers</h2>
					<p className="text-muted-foreground text-sm">
						Register content-safety providers that rules can reference.
					</p>
				</div>
				{hasCreate && (
					<Button onClick={() => setMode({ kind: "new" })}>
						<Plus className="mr-1 h-4 w-4" /> New provider
					</Button>
				)}
			</div>

			<Alert variant="default" className="border-blue-20">
				<Info className="h-4 w-4 text-blue-600" />
				<AlertDescription>
					Phase 1 stores the configuration. Runtime enforcement (the plugin that actually evaluates every request) ships in phase 2.
				</AlertDescription>
			</Alert>

			<div className="rounded-md border">
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>Name</TableHead>
							<TableHead>Type</TableHead>
							<TableHead>Enabled</TableHead>
							<TableHead className="text-right">Actions</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{isLoading ? (
							<TableRow>
								<TableCell colSpan={4} className="text-muted-foreground py-10 text-center">
									Loading…
								</TableCell>
							</TableRow>
						) : providers.length === 0 ? (
							<TableRow>
								<TableCell
									colSpan={4}
									className="text-muted-foreground py-10 text-center"
									data-testid="guardrails-providers-empty"
								>
									No guardrail providers configured.
								</TableCell>
							</TableRow>
						) : (
							providers.map((p) => (
								<TableRow key={p.id}>
									<TableCell className="font-medium">{p.name}</TableCell>
									<TableCell>
										<Badge variant="outline">{p.type}</Badge>
									</TableCell>
									<TableCell>
										<Switch
											checked={p.enabled}
											disabled={!hasUpdate}
											onCheckedChange={(v) => handleToggle(p, v)}
										/>
									</TableCell>
									<TableCell className="flex justify-end gap-2">
										<Button
											variant="outline"
											size="sm"
											disabled={!hasUpdate}
											onClick={() => setMode({ kind: "edit", provider: p })}
										>
											<Pencil className="h-3 w-3" />
										</Button>
										<Button
											variant="outline"
											size="sm"
											disabled={!hasDelete}
											onClick={() => handleDelete(p)}
										>
											<Trash2 className="h-3 w-3" />
										</Button>
									</TableCell>
								</TableRow>
							))
						)}
					</TableBody>
				</Table>
			</div>

			<Dialog open={mode.kind !== "closed"} onOpenChange={(open) => !open && reset()}>
				<DialogContent className="max-w-lg">
					<DialogHeader>
						<DialogTitle>
							{mode.kind === "edit" ? `Edit ${mode.provider.name}` : "New provider"}
						</DialogTitle>
						<DialogDescription>
							Providers are referenced by rules. Credentials are stored and used once the enforcement plugin ships.
						</DialogDescription>
					</DialogHeader>
					<div className="space-y-3">
						<div className="space-y-1">
							<Label htmlFor="gp-name">Name</Label>
							<Input id="gp-name" value={name} onChange={(e) => setName(e.target.value)} />
						</div>
						<div className="space-y-1">
							<Label htmlFor="gp-type">Type</Label>
							<Select
								value={type}
								onValueChange={(v) => setType(v as GuardrailProviderType)}
								disabled={mode.kind === "edit"}
							>
								<SelectTrigger id="gp-type">
									<SelectValue />
								</SelectTrigger>
								<SelectContent>
									<SelectItem value="openai-moderation">OpenAI Moderation</SelectItem>
									<SelectItem value="regex">Regex (built-in)</SelectItem>
									<SelectItem value="custom-webhook">Custom webhook</SelectItem>
								</SelectContent>
							</Select>
						</div>
						{type === "openai-moderation" && (
							<>
								<div className="space-y-1">
									<Label htmlFor="gp-api-key">API key</Label>
									<Input
										id="gp-api-key"
										type="password"
										value={apiKey}
										onChange={(e) => setApiKey(e.target.value)}
									/>
								</div>
								<div className="space-y-1">
									<Label htmlFor="gp-base-url">Base URL (optional)</Label>
									<Input
										id="gp-base-url"
										placeholder="https://api.openai.com"
										value={baseURL}
										onChange={(e) => setBaseURL(e.target.value)}
									/>
								</div>
							</>
						)}
						{type === "custom-webhook" && (
							<div className="space-y-1">
								<Label htmlFor="gp-webhook">Webhook URL</Label>
								<Input
									id="gp-webhook"
									placeholder="https://internal.svc/guardrails/check"
									value={webhookURL}
									onChange={(e) => setWebhookURL(e.target.value)}
								/>
							</div>
						)}
						{type === "regex" && (
							<p className="text-muted-foreground text-sm">
								Regex providers take their pattern on each rule — no provider config required.
							</p>
						)}
					</div>
					<DialogFooter>
						<Button variant="outline" onClick={reset}>
							Cancel
						</Button>
						<Button onClick={handleSave} disabled={isCreating || isUpdating}>
							{isCreating || isUpdating ? "Saving…" : "Save"}
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
		</div>
	);
}
