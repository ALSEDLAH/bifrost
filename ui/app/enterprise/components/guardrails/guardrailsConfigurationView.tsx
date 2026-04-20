// Guardrail Rules view (spec 010 phase 1).

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
import { Textarea } from "@/components/ui/textarea";
import {
	getErrorMessage,
	useCreateGuardrailRuleMutation,
	useDeleteGuardrailRuleMutation,
	useGetGuardrailProvidersQuery,
	useGetGuardrailRulesQuery,
	useUpdateGuardrailRuleMutation,
} from "@/lib/store/apis";
import type { GuardrailAction, GuardrailRule, GuardrailTrigger } from "@/lib/types/guardrails";
import { RbacOperation, RbacResource, useRbac } from "@enterprise/lib";
import { Info, Pencil, Plus, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";

type DialogState =
	| { kind: "closed" }
	| { kind: "new" }
	| { kind: "edit"; rule: GuardrailRule };

export default function GuardrailsConfigurationView() {
	const hasView = useRbac(RbacResource.GuardrailRules, RbacOperation.View);
	const hasCreate = useRbac(RbacResource.GuardrailRules, RbacOperation.Create);
	const hasUpdate = useRbac(RbacResource.GuardrailRules, RbacOperation.Update);
	const hasDelete = useRbac(RbacResource.GuardrailRules, RbacOperation.Delete);

	const { data: rulesData, isLoading } = useGetGuardrailRulesQuery(undefined, { skip: !hasView });
	const { data: providersData } = useGetGuardrailProvidersQuery(undefined, { skip: !hasView });
	const [create, { isLoading: isCreating }] = useCreateGuardrailRuleMutation();
	const [update, { isLoading: isUpdating }] = useUpdateGuardrailRuleMutation();
	const [remove] = useDeleteGuardrailRuleMutation();

	const [mode, setMode] = useState<DialogState>({ kind: "closed" });
	const [name, setName] = useState("");
	const [providerID, setProviderID] = useState<string>("");
	const [trigger, setTrigger] = useState<GuardrailTrigger>("input");
	const [action, setAction] = useState<GuardrailAction>("block");
	const [pattern, setPattern] = useState("");

	const rules = useMemo(() => rulesData?.rules ?? [], [rulesData]);
	const providers = useMemo(() => providersData?.providers ?? [], [providersData]);
	const providerName = (id: string) =>
		providers.find((p) => p.id === id)?.name ?? (id ? "(deleted)" : "—");

	useEffect(() => {
		if (mode.kind === "edit") {
			setName(mode.rule.name);
			setProviderID(mode.rule.provider_id);
			setTrigger(mode.rule.trigger);
			setAction(mode.rule.action);
			setPattern(mode.rule.pattern);
		} else if (mode.kind === "new") {
			setName("");
			setProviderID("");
			setTrigger("input");
			setAction("block");
			setPattern("");
		}
	}, [mode]);

	if (!hasView) return <NoPermissionView entity="guardrail rules" />;

	function reset() {
		setMode({ kind: "closed" });
	}

	async function handleSave() {
		if (!name.trim()) {
			toast.error("Name is required");
			return;
		}
		try {
			if (mode.kind === "new") {
				await create({
					name: name.trim(),
					provider_id: providerID || undefined,
					trigger,
					action,
					pattern,
				}).unwrap();
				toast.success("Rule created");
			} else if (mode.kind === "edit") {
				await update({
					id: mode.rule.id,
					patch: {
						name: name.trim(),
						provider_id: providerID,
						trigger,
						action,
						pattern,
					},
				}).unwrap();
				toast.success("Rule updated");
			}
			reset();
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	}

	async function handleToggle(r: GuardrailRule, enabled: boolean) {
		try {
			await update({ id: r.id, patch: { enabled } }).unwrap();
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	}

	async function handleDelete(r: GuardrailRule) {
		if (!confirm(`Delete rule "${r.name}"?`)) return;
		try {
			await remove({ id: r.id }).unwrap();
			toast.success("Rule deleted");
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	}

	return (
		<div className="mx-auto w-full max-w-5xl space-y-4" data-testid="guardrails-rules-view">
			<div className="flex items-start justify-between">
				<div>
					<h2 className="text-lg font-semibold tracking-tight">Guardrail Rules</h2>
					<p className="text-muted-foreground text-sm">
						Define what to block / flag / log on inference input or output.
					</p>
				</div>
				{hasCreate && (
					<Button onClick={() => setMode({ kind: "new" })}>
						<Plus className="mr-1 h-4 w-4" /> New rule
					</Button>
				)}
			</div>

			<Alert variant="default" className="border-blue-20">
				<Info className="h-4 w-4 text-blue-600" />
				<AlertDescription>
					Phase 1 stores rules. Runtime enforcement (evaluating rules on every inference request) ships in phase 2.
				</AlertDescription>
			</Alert>

			<div className="rounded-md border">
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>Name</TableHead>
							<TableHead>Provider</TableHead>
							<TableHead>Trigger</TableHead>
							<TableHead>Action</TableHead>
							<TableHead>Enabled</TableHead>
							<TableHead className="text-right">Actions</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{isLoading ? (
							<TableRow>
								<TableCell colSpan={6} className="text-muted-foreground py-10 text-center">
									Loading…
								</TableCell>
							</TableRow>
						) : rules.length === 0 ? (
							<TableRow>
								<TableCell
									colSpan={6}
									className="text-muted-foreground py-10 text-center"
									data-testid="guardrails-rules-empty"
								>
									No rules configured.
								</TableCell>
							</TableRow>
						) : (
							rules.map((r) => (
								<TableRow key={r.id}>
									<TableCell className="font-medium">{r.name}</TableCell>
									<TableCell>{providerName(r.provider_id)}</TableCell>
									<TableCell>
										<Badge variant="outline">{r.trigger}</Badge>
									</TableCell>
									<TableCell>
										<Badge variant={r.action === "block" ? "destructive" : "outline"}>{r.action}</Badge>
									</TableCell>
									<TableCell>
										<Switch
											checked={r.enabled}
											disabled={!hasUpdate}
											onCheckedChange={(v) => handleToggle(r, v)}
										/>
									</TableCell>
									<TableCell className="flex justify-end gap-2">
										<Button
											variant="outline"
											size="sm"
											disabled={!hasUpdate}
											onClick={() => setMode({ kind: "edit", rule: r })}
										>
											<Pencil className="h-3 w-3" />
										</Button>
										<Button
											variant="outline"
											size="sm"
											disabled={!hasDelete}
											onClick={() => handleDelete(r)}
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
							{mode.kind === "edit" ? `Edit ${mode.rule.name}` : "New rule"}
						</DialogTitle>
						<DialogDescription>
							A rule fires on input, output, or both. On match, it blocks, flags, or just logs.
						</DialogDescription>
					</DialogHeader>
					<div className="space-y-3">
						<div className="space-y-1">
							<Label htmlFor="gr-name">Name</Label>
							<Input id="gr-name" value={name} onChange={(e) => setName(e.target.value)} />
						</div>
						<div className="space-y-1">
							<Label htmlFor="gr-provider">Provider</Label>
							<Select value={providerID} onValueChange={setProviderID}>
								<SelectTrigger id="gr-provider">
									<SelectValue placeholder="(none — regex-only)" />
								</SelectTrigger>
								<SelectContent>
									<SelectItem value="">(none — regex-only)</SelectItem>
									{providers.map((p) => (
										<SelectItem key={p.id} value={p.id}>
											{p.name} — {p.type}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
						</div>
						<div className="grid grid-cols-2 gap-3">
							<div className="space-y-1">
								<Label htmlFor="gr-trigger">Trigger</Label>
								<Select value={trigger} onValueChange={(v) => setTrigger(v as GuardrailTrigger)}>
									<SelectTrigger id="gr-trigger">
										<SelectValue />
									</SelectTrigger>
									<SelectContent>
										<SelectItem value="input">Input</SelectItem>
										<SelectItem value="output">Output</SelectItem>
										<SelectItem value="both">Both</SelectItem>
									</SelectContent>
								</Select>
							</div>
							<div className="space-y-1">
								<Label htmlFor="gr-action">Action</Label>
								<Select value={action} onValueChange={(v) => setAction(v as GuardrailAction)}>
									<SelectTrigger id="gr-action">
										<SelectValue />
									</SelectTrigger>
									<SelectContent>
										<SelectItem value="block">Block</SelectItem>
										<SelectItem value="flag">Flag</SelectItem>
										<SelectItem value="log">Log</SelectItem>
									</SelectContent>
								</Select>
							</div>
						</div>
						<div className="space-y-1">
							<Label htmlFor="gr-pattern">Pattern</Label>
							<Textarea
								id="gr-pattern"
								rows={3}
								placeholder="e.g. for regex rules: \\b(?:4[0-9]{12}(?:[0-9]{3})?|5[1-5][0-9]{14})\\b"
								value={pattern}
								onChange={(e) => setPattern(e.target.value)}
							/>
							<p className="text-muted-foreground text-xs">
								For regex rules, this is the pattern. For provider rules, optional category filter.
							</p>
						</div>
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
