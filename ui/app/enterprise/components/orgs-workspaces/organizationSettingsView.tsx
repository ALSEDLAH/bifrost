// Organization settings (US1, T042).
//
// Edits the single current-org fields exposed by
// /v1/admin/organizations/current: sso_required, break_glass_enabled,
// default_retention_days, plus the display name.

import { getErrorMessage } from "@/lib/store/apis/baseApi";
import {
	useGetCurrentOrganizationQuery,
	useUpdateCurrentOrganizationMutation,
} from "@/lib/store/apis/enterpriseApi";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { useEffect, useState } from "react";
import { toast } from "sonner";

export function OrganizationSettingsView() {
	const { data: org, isLoading, error } = useGetCurrentOrganizationQuery();
	const [updateOrg, { isLoading: saving }] = useUpdateCurrentOrganizationMutation();

	const [name, setName] = useState("");
	const [ssoRequired, setSsoRequired] = useState(false);
	const [breakGlass, setBreakGlass] = useState(false);
	const [defaultRetention, setDefaultRetention] = useState("90");

	useEffect(() => {
		if (org) {
			setName(org.name);
			setSsoRequired(org.sso_required);
			setBreakGlass(org.break_glass_enabled);
			setDefaultRetention(String(org.default_retention_days));
		}
	}, [org]);

	if (isLoading) return <div data-testid="org-settings-loading">Loading…</div>;
	if (error) {
		return (
			<div className="text-destructive" data-testid="org-settings-error">
				{getErrorMessage(error)}
			</div>
		);
	}
	if (!org) return null;

	const save = async () => {
		const body: Record<string, any> = {};
		if (name !== org.name) body.name = name;
		if (ssoRequired !== org.sso_required) body.sso_required = ssoRequired;
		if (breakGlass !== org.break_glass_enabled) body.break_glass_enabled = breakGlass;
		const parsedRetention = parseInt(defaultRetention, 10);
		if (!Number.isNaN(parsedRetention) && parsedRetention !== org.default_retention_days) {
			body.default_retention_days = parsedRetention;
		}
		if (Object.keys(body).length === 0) {
			toast.info("No changes to save.");
			return;
		}
		try {
			await updateOrg(body).unwrap();
			toast.success("Organization settings saved.");
		} catch (err) {
			toast.error(`Save failed: ${getErrorMessage(err)}`);
		}
	};

	return (
		<div className="flex max-w-xl flex-col gap-6" data-testid="org-settings-root">
			<div>
				<h1 className="text-2xl font-semibold">Organization settings</h1>
				<p className="text-muted-foreground text-sm">
					Org-level defaults. Workspaces may override individual values (retention, payload encryption).
				</p>
			</div>

			<div className="grid gap-2">
				<Label htmlFor="org-input-name">Organization name</Label>
				<Input
					id="org-input-name"
					data-testid="org-input-name"
					value={name}
					onChange={(e) => setName(e.target.value)}
				/>
			</div>

			<div className="flex items-center justify-between gap-4 rounded-md border p-4">
				<div>
					<p className="font-medium">Require SSO</p>
					<p className="text-muted-foreground text-sm">
						When enabled, all logins go through the configured SSO provider. Local logins are rejected unless
						Break-Glass is also on.
					</p>
				</div>
				<Switch
					data-testid="org-toggle-sso-required"
					checked={ssoRequired}
					onCheckedChange={setSsoRequired}
				/>
			</div>

			<div className="flex items-center justify-between gap-4 rounded-md border p-4">
				<div>
					<p className="font-medium">Break-glass local login</p>
					<p className="text-muted-foreground text-sm">
						Allow a whitelisted set of users to authenticate with local credentials even when SSO is required.
						Every break-glass login fires a high-severity audit entry.
					</p>
				</div>
				<Switch
					data-testid="org-toggle-break-glass"
					checked={breakGlass}
					onCheckedChange={setBreakGlass}
				/>
			</div>

			<div className="grid gap-2">
				<Label htmlFor="org-input-retention">Default log retention (days)</Label>
				<Input
					id="org-input-retention"
					data-testid="org-input-retention"
					type="number"
					min={1}
					value={defaultRetention}
					onChange={(e) => setDefaultRetention(e.target.value)}
				/>
				<p className="text-muted-foreground text-xs">
					Workspaces may override via their per-workspace retention setting.
				</p>
			</div>

			<div>
				<p className="text-muted-foreground text-xs">
					Data residency region: <span className="font-mono">{org.data_residency_region}</span> (set at
					deployment; read-only in v1 — FR-050b)
				</p>
			</div>

			<div className="flex justify-end">
				<Button onClick={save} disabled={saving} data-testid="org-button-save">
					{saving ? "Saving…" : "Save changes"}
				</Button>
			</div>
		</div>
	);
}
