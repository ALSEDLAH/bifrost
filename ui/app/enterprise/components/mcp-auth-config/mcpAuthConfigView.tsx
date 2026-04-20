// Enterprise MCP Auth Config view (US30, T075).
//
// Reuse-only per SR-01: surfaces upstream MCP + OAuth handlers already
// shipped. For every MCP client with auth_type ∈ {oauth, per_user_oauth}
// we show its current OAuth config status (from /api/oauth/config/:id
// /status) and offer a revoke action (DELETE /api/oauth/config/:id).
// No new backend is introduced.

import FullPageLoader from "@/components/fullPageLoader";
import { NoPermissionView } from "@/components/noPermissionView";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@/components/ui/alertDialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import {
	getErrorMessage,
	useGetMCPClientsQuery,
	useGetOAuthConfigStatusQuery,
	useRevokeOAuthConfigMutation,
} from "@/lib/store";
import type { MCPClient, OAuthStatusResponse } from "@/lib/types/mcp";
import { RbacOperation, RbacResource, useRbac } from "@enterprise/lib";
import { RefreshCw, ShieldUser, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";

function statusBadge(status: string) {
	switch (status) {
		case "authorized":
			return <Badge>Authorized</Badge>;
		case "pending":
			return <Badge variant="secondary">Pending</Badge>;
		case "failed":
			return <Badge variant="destructive">Failed</Badge>;
		case "expired":
			return <Badge variant="destructive">Expired</Badge>;
		case "revoked":
			return <Badge variant="destructive">Revoked</Badge>;
		default:
			return <Badge variant="outline">{status}</Badge>;
	}
}

function formatDate(iso: string | null | undefined): string {
	if (!iso) return "—";
	try {
		return new Date(iso).toLocaleString();
	} catch {
		return iso;
	}
}

interface OAuthRowProps {
	client: MCPClient;
	canDelete: boolean;
	onRequestRevoke: (client: MCPClient, status: OAuthStatusResponse | undefined) => void;
}

function OAuthRow({ client, canDelete, onRequestRevoke }: OAuthRowProps) {
	const cfg = client.config;
	const configId = cfg.oauth_config_id;
	const {
		data: status,
		isLoading,
		isFetching,
		refetch,
	} = useGetOAuthConfigStatusQuery(configId ?? "", { skip: !configId });

	if (!configId) return null;

	return (
		<TableRow data-testid={`mcp-auth-row-${cfg.name}`}>
			<TableCell className="font-medium">{cfg.name}</TableCell>
			<TableCell>
				<Badge variant="outline">{cfg.auth_type}</Badge>
			</TableCell>
			<TableCell>
				{isLoading ? (
					<span className="text-muted-foreground text-sm">Loading…</span>
				) : status ? (
					statusBadge(status.status)
				) : (
					<span className="text-muted-foreground text-sm">Unknown</span>
				)}
			</TableCell>
			<TableCell className="text-sm">{formatDate(status?.token_expires_at ?? status?.expires_at)}</TableCell>
			<TableCell className="font-mono text-xs">{status?.token_scopes || "—"}</TableCell>
			<TableCell className="text-right">
				<Button
					variant="ghost"
					size="sm"
					onClick={() => refetch()}
					disabled={isFetching}
					data-testid={`mcp-auth-refresh-${cfg.name}`}
				>
					<RefreshCw className={isFetching ? "h-4 w-4 animate-spin" : "h-4 w-4"} />
				</Button>
				{canDelete && status?.status === "authorized" && (
					<Button
						variant="ghost"
						size="sm"
						onClick={() => onRequestRevoke(client, status)}
						data-testid={`mcp-auth-revoke-${cfg.name}`}
					>
						<Trash2 className="text-destructive h-4 w-4" />
					</Button>
				)}
			</TableCell>
		</TableRow>
	);
}

export default function MCPAuthConfigView() {
	const hasView = useRbac(RbacResource.MCPGateway, RbacOperation.View);
	const hasDelete = useRbac(RbacResource.MCPGateway, RbacOperation.Delete);

	const { data: clients, isLoading, error } = useGetMCPClientsQuery(undefined, { skip: !hasView });
	const [revoke, { isLoading: isRevoking }] = useRevokeOAuthConfigMutation();

	const [toRevoke, setToRevoke] = useState<MCPClient | null>(null);

	const oauthClients = useMemo(
		() => (clients?.clients ?? []).filter((c) => c.config.auth_type === "oauth" || c.config.auth_type === "per_user_oauth"),
		[clients],
	);

	useEffect(() => {
		if (error) toast.error(`Failed to load MCP clients: ${getErrorMessage(error)}`);
	}, [error]);

	if (!hasView) return <NoPermissionView entity="MCP auth configuration" />;
	if (isLoading) return <FullPageLoader />;

	const handleConfirmRevoke = async () => {
		const configId = toRevoke?.config.oauth_config_id;
		if (!configId) return;
		try {
			await revoke(configId).unwrap();
			toast.success(`OAuth token revoked for "${toRevoke.config.name}"`);
			setToRevoke(null);
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	};

	return (
		<div className="space-y-6" data-testid="mcp-auth-config-view">
			<div>
				<div className="flex items-center gap-3">
					<ShieldUser className="h-6 w-6" />
					<h2 className="text-2xl font-semibold">MCP Auth Config</h2>
				</div>
				<p className="text-muted-foreground mt-1 text-sm">
					OAuth-authenticated MCP clients and their current token status. Revoking here invalidates the
					stored token; the client must re-authorize before it can be used again.
				</p>
			</div>

			<div className="rounded-md border">
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>MCP client</TableHead>
							<TableHead>Auth type</TableHead>
							<TableHead>Status</TableHead>
							<TableHead>Token expires</TableHead>
							<TableHead>Scopes</TableHead>
							<TableHead className="text-right">Actions</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{oauthClients.length === 0 ? (
							<TableRow>
								<TableCell colSpan={6} className="text-muted-foreground py-10 text-center" data-testid="mcp-auth-empty">
									No MCP clients are configured with OAuth yet. Set <code>auth_type: oauth</code> on a
									client to enable token-based access.
								</TableCell>
							</TableRow>
						) : (
							oauthClients.map((c) => (
								<OAuthRow
									key={c.config.client_id}
									client={c}
									canDelete={hasDelete}
									onRequestRevoke={(cl) => setToRevoke(cl)}
								/>
							))
						)}
					</TableBody>
				</Table>
			</div>

			<AlertDialog open={!!toRevoke} onOpenChange={(o) => !o && setToRevoke(null)}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Revoke OAuth token?</AlertDialogTitle>
						<AlertDialogDescription>
							Revoke the OAuth token for <strong>{toRevoke?.config.name}</strong>? The client will stop
							functioning until it is re-authorized.
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel disabled={isRevoking}>Cancel</AlertDialogCancel>
						<AlertDialogAction onClick={handleConfirmRevoke} disabled={isRevoking}>
							{isRevoking ? "Revoking…" : "Revoke"}
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	);
}
