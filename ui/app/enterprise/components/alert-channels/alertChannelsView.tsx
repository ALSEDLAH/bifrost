// Alert Channels view (spec 004).
//
// Lists configured webhook/Slack destinations for governance events
// (budget.threshold.crossed today; more event types later). Supports
// create, enable/disable toggle, test-dispatch, and delete.

import { NoPermissionView } from "@/components/noPermissionView";
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
	useCreateAlertChannelMutation,
	useDeleteAlertChannelMutation,
	useGetAlertChannelsQuery,
	useTestAlertChannelMutation,
	useUpdateAlertChannelMutation,
} from "@/lib/store/apis";
import type {
	AlertChannel,
	AlertChannelType,
} from "@/lib/types/alertChannels";
import { RbacOperation, RbacResource, useRbac } from "@enterprise/lib";
import { Plus, Send, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import { toast } from "sonner";

function parseConfig(ch: AlertChannel): Record<string, unknown> {
	try {
		return JSON.parse(ch.config) as Record<string, unknown>;
	} catch {
		return {};
	}
}

function channelSummary(ch: AlertChannel): string {
	const cfg = parseConfig(ch);
	if (ch.type === "slack") return (cfg.webhook_url as string) ?? "";
	if (ch.type === "webhook") return (cfg.url as string) ?? "";
	return "";
}

export default function AlertChannelsView() {
	const hasView = useRbac(RbacResource.AlertChannels, RbacOperation.View);
	const hasCreate = useRbac(RbacResource.AlertChannels, RbacOperation.Create);
	const hasUpdate = useRbac(RbacResource.AlertChannels, RbacOperation.Update);
	const hasDelete = useRbac(RbacResource.AlertChannels, RbacOperation.Delete);

	const { data, isLoading } = useGetAlertChannelsQuery(undefined, { skip: !hasView });
	const [createChannel, { isLoading: isCreating }] = useCreateAlertChannelMutation();
	const [updateChannel] = useUpdateAlertChannelMutation();
	const [deleteChannel] = useDeleteAlertChannelMutation();
	const [testChannel] = useTestAlertChannelMutation();

	const [dialogOpen, setDialogOpen] = useState(false);
	const [form, setForm] = useState<{ name: string; type: AlertChannelType; url: string }>({
		name: "",
		type: "slack",
		url: "",
	});

	const channels = useMemo(() => data?.channels ?? [], [data]);

	if (!hasView) return <NoPermissionView entity="alert channels" />;

	async function handleCreate() {
		if (!form.name.trim() || !form.url.trim()) {
			toast.error("Name and URL are required");
			return;
		}
		try {
			await createChannel({
				name: form.name.trim(),
				type: form.type,
				config:
					form.type === "slack"
						? { webhook_url: form.url.trim() }
						: { url: form.url.trim() },
				enabled: true,
			}).unwrap();
			toast.success("Alert channel created");
			setDialogOpen(false);
			setForm({ name: "", type: "slack", url: "" });
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	}

	async function handleToggle(ch: AlertChannel, enabled: boolean) {
		try {
			await updateChannel({ id: ch.id, patch: { enabled } }).unwrap();
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	}

	async function handleTest(ch: AlertChannel) {
		try {
			await testChannel({ id: ch.id }).unwrap();
			toast.success(`Test dispatched to "${ch.name}" — check your destination`);
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	}

	async function handleDelete(ch: AlertChannel) {
		if (!confirm(`Delete alert channel "${ch.name}"?`)) return;
		try {
			await deleteChannel({ id: ch.id }).unwrap();
			toast.success("Alert channel deleted");
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	}

	return (
		<div className="mx-auto w-full max-w-5xl space-y-4" data-testid="alert-channels-view">
			<div className="flex items-start justify-between">
				<div>
					<h2 className="text-lg font-semibold tracking-tight">Alert Channels</h2>
					<p className="text-muted-foreground text-sm">
						Configure Slack and webhook destinations for governance events.
						Budget threshold crossings (50 / 75 / 90%) fan out to every enabled channel.
					</p>
				</div>
				{hasCreate && (
					<Button onClick={() => setDialogOpen(true)} data-testid="alert-channels-new">
						<Plus className="mr-1 h-4 w-4" /> New channel
					</Button>
				)}
			</div>

			<div className="rounded-md border">
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>Name</TableHead>
							<TableHead>Type</TableHead>
							<TableHead>Destination</TableHead>
							<TableHead>Enabled</TableHead>
							<TableHead className="text-right">Actions</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{isLoading ? (
							<TableRow>
								<TableCell colSpan={5} className="text-muted-foreground py-10 text-center">
									Loading…
								</TableCell>
							</TableRow>
						) : channels.length === 0 ? (
							<TableRow>
								<TableCell
									colSpan={5}
									className="text-muted-foreground py-10 text-center"
									data-testid="alert-channels-empty"
								>
									No alert channels configured.
								</TableCell>
							</TableRow>
						) : (
							channels.map((ch) => (
								<TableRow key={ch.id} data-testid={`alert-channels-row-${ch.id}`}>
									<TableCell className="font-medium">{ch.name}</TableCell>
									<TableCell>
										<Badge variant="outline">{ch.type}</Badge>
									</TableCell>
									<TableCell className="text-muted-foreground max-w-[280px] truncate text-sm">
										{channelSummary(ch)}
									</TableCell>
									<TableCell>
										<Switch
											checked={ch.enabled}
											disabled={!hasUpdate}
											onCheckedChange={(v) => handleToggle(ch, v)}
											aria-label={`Toggle ${ch.name}`}
										/>
									</TableCell>
									<TableCell className="flex justify-end gap-2">
										<Button
											variant="outline"
											size="sm"
											onClick={() => handleTest(ch)}
											data-testid={`alert-channels-test-${ch.id}`}
										>
											<Send className="mr-1 h-3 w-3" /> Test
										</Button>
										<Button
											variant="outline"
											size="sm"
											disabled={!hasDelete}
											onClick={() => handleDelete(ch)}
											data-testid={`alert-channels-delete-${ch.id}`}
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

			<Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>New alert channel</DialogTitle>
						<DialogDescription>
							Paste an incoming-webhook URL. For Slack, generate one under{" "}
							<b>Slack → Your app → Incoming Webhooks</b>.
						</DialogDescription>
					</DialogHeader>
					<div className="space-y-3">
						<div className="space-y-1">
							<Label htmlFor="alert-channel-name">Name</Label>
							<Input
								id="alert-channel-name"
								placeholder="#ai-platform-alerts"
								value={form.name}
								onChange={(e) => setForm({ ...form, name: e.target.value })}
							/>
						</div>
						<div className="space-y-1">
							<Label htmlFor="alert-channel-type">Type</Label>
							<Select
								value={form.type}
								onValueChange={(v) => setForm({ ...form, type: v as AlertChannelType })}
							>
								<SelectTrigger id="alert-channel-type">
									<SelectValue />
								</SelectTrigger>
								<SelectContent>
									<SelectItem value="slack">Slack</SelectItem>
									<SelectItem value="webhook">Generic webhook</SelectItem>
								</SelectContent>
							</Select>
						</div>
						<div className="space-y-1">
							<Label htmlFor="alert-channel-url">
								{form.type === "slack" ? "Slack webhook URL" : "Webhook URL"}
							</Label>
							<Input
								id="alert-channel-url"
								placeholder={
									form.type === "slack"
										? "https://hooks.slack.com/services/..."
										: "https://example.com/hooks/bifrost"
								}
								value={form.url}
								onChange={(e) => setForm({ ...form, url: e.target.value })}
							/>
						</div>
					</div>
					<DialogFooter>
						<Button variant="outline" onClick={() => setDialogOpen(false)}>
							Cancel
						</Button>
						<Button onClick={handleCreate} disabled={isCreating}>
							{isCreating ? "Creating…" : "Create"}
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
		</div>
	);
}
