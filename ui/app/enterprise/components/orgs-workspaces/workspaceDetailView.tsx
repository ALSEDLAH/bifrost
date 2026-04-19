// Enterprise workspace detail + edit view (US1).

import { getErrorMessage } from "@/lib/store/apis/baseApi";
import { useGetWorkspaceQuery, usePatchWorkspaceMutation } from "@/lib/store/apis/enterpriseApi";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { useParams } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { toast } from "sonner";

export function WorkspaceDetailView() {
	const { workspaceId } = useParams({ strict: false }) as { workspaceId: string };
	const { data: ws, isLoading, error } = useGetWorkspaceQuery(workspaceId, { skip: !workspaceId });
	const [patchWorkspace, { isLoading: saving }] = usePatchWorkspaceMutation();

	const [name, setName] = useState("");
	const [description, setDescription] = useState("");
	const [logRetention, setLogRetention] = useState<string>("");
	const [payloadEncryption, setPayloadEncryption] = useState(false);

	useEffect(() => {
		if (ws) {
			setName(ws.name);
			setDescription(ws.description ?? "");
			setLogRetention(ws.log_retention_days != null ? String(ws.log_retention_days) : "");
			setPayloadEncryption(ws.payload_encryption_enabled);
		}
	}, [ws]);

	if (isLoading) return <div data-testid="workspace-detail-loading">Loading…</div>;
	if (error) {
		return (
			<div className="text-destructive" data-testid="workspace-detail-error">
				{getErrorMessage(error)}
			</div>
		);
	}
	if (!ws) return null;

	const save = async () => {
		const body: Record<string, any> = {};
		if (name !== ws.name) body.name = name;
		if (description !== (ws.description ?? "")) body.description = description;
		if (payloadEncryption !== ws.payload_encryption_enabled) {
			body.payload_encryption_enabled = payloadEncryption;
		}
		if (logRetention !== String(ws.log_retention_days ?? "")) {
			const n = parseInt(logRetention, 10);
			if (!Number.isNaN(n)) body.log_retention_days = n;
		}
		if (Object.keys(body).length === 0) {
			toast.info("No changes to save.");
			return;
		}
		try {
			await patchWorkspace({ id: workspaceId, body }).unwrap();
			toast.success("Workspace updated.");
		} catch (err) {
			toast.error(`Save failed: ${getErrorMessage(err)}`);
		}
	};

	return (
		<div className="flex flex-col gap-6" data-testid="workspace-detail-root">
			<div>
				<h1 className="text-2xl font-semibold" data-testid="workspace-detail-name">
					{ws.name}
				</h1>
				<p className="text-muted-foreground font-mono text-sm">{ws.slug}</p>
			</div>

			<div className="grid max-w-xl gap-5">
				<div className="grid gap-2">
					<Label htmlFor="workspace-input-name-edit">Name</Label>
					<Input
						id="workspace-input-name-edit"
						data-testid="workspace-input-name-edit"
						value={name}
						onChange={(e) => setName(e.target.value)}
					/>
				</div>

				<div className="grid gap-2">
					<Label htmlFor="workspace-input-description-edit">Description</Label>
					<Textarea
						id="workspace-input-description-edit"
						data-testid="workspace-input-description-edit"
						value={description}
						onChange={(e) => setDescription(e.target.value)}
						rows={3}
					/>
				</div>

				<div className="grid gap-2">
					<Label htmlFor="workspace-input-log-retention">Log retention (days)</Label>
					<Input
						id="workspace-input-log-retention"
						data-testid="workspace-input-log-retention"
						type="number"
						min={1}
						value={logRetention}
						onChange={(e) => setLogRetention(e.target.value)}
						placeholder="Inherit from organization"
					/>
					<p className="text-muted-foreground text-xs">Leave empty to inherit the organization default.</p>
				</div>

				<div className="flex items-center justify-between gap-4 rounded-md border p-4">
					<div>
						<p className="font-medium">Payload encryption (BYOK)</p>
						<p className="text-muted-foreground text-sm">
							Encrypt this workspace's request/response payloads at rest with the active customer-managed KMS
							key. Requires an active KMS configuration.
						</p>
					</div>
					<Switch
						data-testid="workspace-toggle-payload-encryption"
						checked={payloadEncryption}
						onCheckedChange={setPayloadEncryption}
					/>
				</div>

				<div className="flex justify-end">
					<Button onClick={save} disabled={saving} data-testid="workspace-button-save">
						{saving ? "Saving…" : "Save changes"}
					</Button>
				</div>
			</div>
		</div>
	);
}
