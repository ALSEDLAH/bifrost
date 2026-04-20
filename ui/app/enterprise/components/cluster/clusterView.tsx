// Cluster status view (spec 007).
//
// Single-node v1: renders self-reported metadata pulled from
// /api/cluster/status. Multi-node coordination is a future spec.

import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { NoPermissionView } from "@/components/noPermissionView";
import { useGetClusterStatusQuery } from "@/lib/store/apis";
import type { ClusterStatus } from "@/lib/types/cluster";
import { RbacOperation, RbacResource, useRbac } from "@enterprise/lib";

function formatBytes(n: number): string {
	if (n < 1024) return `${n} B`;
	if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
	if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
	return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

function formatUptime(seconds: number): string {
	const d = Math.floor(seconds / 86400);
	const h = Math.floor((seconds % 86400) / 3600);
	const m = Math.floor((seconds % 3600) / 60);
	const s = Math.floor(seconds % 60);
	if (d > 0) return `${d}d ${h}h ${m}m`;
	if (h > 0) return `${h}h ${m}m`;
	if (m > 0) return `${m}m ${s}s`;
	return `${s}s`;
}

function StatRow({ label, value }: { label: string; value: React.ReactNode }) {
	return (
		<div className="flex items-baseline justify-between border-b py-2 last:border-b-0">
			<span className="text-muted-foreground text-xs uppercase tracking-wide">{label}</span>
			<span className="font-mono text-sm">{value}</span>
		</div>
	);
}

export default function ClusterView() {
	const hasView = useRbac(RbacResource.Cluster, RbacOperation.View);
	const { data, isLoading, error } = useGetClusterStatusQuery(undefined, { skip: !hasView, pollingInterval: 10000 });

	if (!hasView) return <NoPermissionView entity="cluster status" />;

	const status: ClusterStatus | undefined = data;

	return (
		<div className="mx-auto w-full max-w-4xl space-y-4" data-testid="cluster-view">
			<div>
				<h2 className="text-lg font-semibold tracking-tight">Cluster Status</h2>
				<p className="text-muted-foreground text-sm">
					This deployment is a single node. Multi-node coordination (peer discovery, leader election) is a future feature.
				</p>
			</div>

			{error ? (
				<div className="text-destructive text-sm">Failed to load cluster status.</div>
			) : null}

			<div className="grid grid-cols-1 gap-4 md:grid-cols-2">
				<Card>
					<CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
						<CardTitle className="text-base">This node</CardTitle>
						<Badge variant="outline" className="text-xs">
							{status?.node_role ?? "…"}
						</Badge>
					</CardHeader>
					<CardContent>
						<StatRow label="Hostname" value={isLoading ? "…" : status?.hostname ?? "—"} />
						<StatRow label="Version" value={isLoading ? "…" : status?.version ?? "—"} />
						<StatRow
							label="Started at"
							value={isLoading ? "…" : status ? new Date(status.started_at).toLocaleString() : "—"}
						/>
						<StatRow label="Uptime" value={isLoading ? "…" : status ? formatUptime(status.uptime_seconds) : "—"} />
					</CardContent>
				</Card>

				<Card>
					<CardHeader>
						<CardTitle className="text-base">Runtime</CardTitle>
					</CardHeader>
					<CardContent>
						<StatRow label="PID" value={isLoading ? "…" : status?.pid ?? "—"} />
						<StatRow label="Goroutines" value={isLoading ? "…" : status?.goroutines ?? "—"} />
						<StatRow
							label="Heap allocated"
							value={isLoading ? "…" : status ? formatBytes(status.process_memory_bytes) : "—"}
						/>
					</CardContent>
				</Card>
			</div>
		</div>
	);
}
