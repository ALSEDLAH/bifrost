// SCIM provisioning view (spec 009 phase 1).
//
// Generates a bearer token and shows the endpoint URL. The /scim/v2/*
// HTTP endpoints that consume the token are phase 2 — the UI says so.

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
import { Switch } from "@/components/ui/switch";
import {
	getErrorMessage,
	useGetSCIMConfigQuery,
	usePatchSCIMConfigMutation,
	useRotateSCIMTokenMutation,
} from "@/lib/store/apis";
import { RbacOperation, RbacResource, useRbac } from "@enterprise/lib";
import { Copy, Info, RefreshCw } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

export default function SCIMView() {
	const hasView = useRbac(RbacResource.UserProvisioning, RbacOperation.View);
	const hasUpdate = useRbac(RbacResource.UserProvisioning, RbacOperation.Update);

	const { data, isLoading } = useGetSCIMConfigQuery(undefined, { skip: !hasView });
	const [patch, { isLoading: isPatching }] = usePatchSCIMConfigMutation();
	const [rotate, { isLoading: isRotating }] = useRotateSCIMTokenMutation();
	const [revealed, setRevealed] = useState<string | null>(null);

	if (!hasView) return <NoPermissionView entity="SCIM provisioning" />;

	async function handleToggle(v: boolean) {
		try {
			await patch({ enabled: v }).unwrap();
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	}

	async function handleRotate() {
		if (!confirm("Rotate the SCIM token? This will invalidate the existing one.")) return;
		try {
			const result = await rotate().unwrap();
			setRevealed(result.token);
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	}

	function copyToClipboard(v: string) {
		navigator.clipboard.writeText(v).then(
			() => toast.success("Copied to clipboard"),
			() => toast.error("Copy failed"),
		);
	}

	return (
		<div className="mx-auto w-full max-w-3xl space-y-4" data-testid="scim-view">
			<div className="flex items-start justify-between">
				<div>
					<h2 className="text-lg font-semibold tracking-tight">SCIM Provisioning</h2>
					<p className="text-muted-foreground text-sm">
						Generate a bearer token so Okta / Azure AD / JumpCloud can provision
						users against Bifrost.
					</p>
				</div>
				<Switch
					checked={data?.enabled ?? false}
					disabled={!hasUpdate || isPatching || isLoading}
					onCheckedChange={handleToggle}
					aria-label="Enable SCIM"
				/>
			</div>

			<Alert variant="default" className="border-blue-20">
				<Info className="h-4 w-4 text-blue-600" />
				<AlertDescription>
					Phase 1 ships the admin surface only. The <code>/scim/v2/Users</code> endpoints
					that IdPs call are tracked as phase 2 (see spec 009). Tokens generated here
					will work once phase 2 ships.
				</AlertDescription>
			</Alert>

			<div className="space-y-4 rounded-lg border p-4">
				<div className="space-y-1">
					<Label htmlFor="scim-endpoint">Endpoint URL</Label>
					<div className="flex items-center gap-2">
						<Input id="scim-endpoint" readOnly value={data?.endpoint_url ?? ""} className="font-mono text-sm" />
						<Button
							variant="outline"
							size="icon"
							onClick={() => data?.endpoint_url && copyToClipboard(data.endpoint_url)}
							aria-label="Copy endpoint URL"
						>
							<Copy className="h-4 w-4" />
						</Button>
					</div>
				</div>

				<div className="space-y-1">
					<Label>Bearer token</Label>
					{data?.token_prefix ? (
						<div className="flex items-center gap-2">
							<Badge variant="outline" className="font-mono">
								{data.token_prefix}…
							</Badge>
							<span className="text-muted-foreground text-xs">
								Generated {data.token_created_at ? new Date(data.token_created_at).toLocaleString() : ""}
							</span>
						</div>
					) : (
						<p className="text-muted-foreground text-sm">No token generated yet.</p>
					)}
					<Button
						variant="outline"
						size="sm"
						className="mt-2"
						disabled={!hasUpdate || isRotating}
						onClick={handleRotate}
					>
						<RefreshCw className="mr-1 h-3 w-3" />
						{data?.token_prefix ? "Regenerate token" : "Generate token"}
					</Button>
				</div>
			</div>

			<Dialog open={revealed !== null} onOpenChange={(open) => !open && setRevealed(null)}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>New SCIM token</DialogTitle>
						<DialogDescription>
							Copy this token now — it will not be shown again. Paste it into
							your IdP's SCIM integration as the bearer credential.
						</DialogDescription>
					</DialogHeader>
					<div className="space-y-2">
						<Input readOnly value={revealed ?? ""} className="font-mono text-xs" />
						<Button
							variant="outline"
							size="sm"
							onClick={() => revealed && copyToClipboard(revealed)}
						>
							<Copy className="mr-1 h-4 w-4" /> Copy token
						</Button>
					</div>
					<DialogFooter>
						<Button onClick={() => setRevealed(null)}>Done</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
		</div>
	);
}
