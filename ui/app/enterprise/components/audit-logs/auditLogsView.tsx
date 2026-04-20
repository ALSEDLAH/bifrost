// Enterprise audit logs view (US4, T036).
//
// Wraps /api/audit-logs with filters (actor, action, resource type,
// outcome, date range) + CSV/JSON export. Entries are read-only and
// immutable by design (FR-010..FR-012).

import FullPageLoader from "@/components/fullPageLoader";
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
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { buildAuditExportUrl, getErrorMessage, useGetAuditLogsQuery } from "@/lib/store";
import type { AuditEntry, AuditLogFilters, AuditOutcome } from "@/lib/types/enterprise";
import { getApiBaseUrl } from "@/lib/utils/port";
import { RbacOperation, RbacResource, useRbac } from "@enterprise/lib";
import { Download, Eye, RefreshCw, Search } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";

const PAGE_SIZE = 50;
const OUTCOMES: AuditOutcome[] = ["allowed", "denied", "error"];
const OUTCOME_ANY = "__any__";

function outcomeBadge(outcome: string) {
	if (outcome === "denied") return <Badge variant="destructive">Denied</Badge>;
	if (outcome === "error") return <Badge variant="destructive">Error</Badge>;
	return <Badge>Allowed</Badge>;
}

function formatTime(iso: string): string {
	try {
		return new Date(iso).toLocaleString();
	} catch {
		return iso;
	}
}

function toRFC3339(local: string): string | undefined {
	if (!local) return undefined;
	const d = new Date(local);
	if (isNaN(d.getTime())) return undefined;
	return d.toISOString();
}

interface DetailsDialogProps {
	entry: AuditEntry | null;
	onClose: () => void;
}

function DetailsDialog({ entry, onClose }: DetailsDialogProps) {
	return (
		<Dialog open={!!entry} onOpenChange={(o) => !o && onClose()}>
			<DialogContent className="max-h-[90vh] max-w-3xl overflow-y-auto" data-testid="audit-details-dialog">
				<DialogHeader>
					<DialogTitle>Audit entry</DialogTitle>
					<DialogDescription>{entry?.id}</DialogDescription>
				</DialogHeader>
				{entry && (
					<div className="space-y-3 text-sm">
						<div className="grid grid-cols-2 gap-2">
							<div><span className="text-muted-foreground">Time:</span> {formatTime(entry.created_at)}</div>
							<div><span className="text-muted-foreground">Outcome:</span> {entry.outcome}</div>
							<div><span className="text-muted-foreground">Action:</span> {entry.action}</div>
							<div><span className="text-muted-foreground">Resource:</span> {entry.resource_type}{entry.resource_id ? `/${entry.resource_id}` : ""}</div>
							<div><span className="text-muted-foreground">Actor:</span> {entry.actor_display || entry.actor_id || "—"}</div>
							<div><span className="text-muted-foreground">Actor type:</span> {entry.actor_type}</div>
							<div><span className="text-muted-foreground">Actor IP:</span> {entry.actor_ip || "—"}</div>
							<div><span className="text-muted-foreground">Request ID:</span> {entry.request_id || "—"}</div>
						</div>
						{entry.reason && (
							<div>
								<div className="text-muted-foreground mb-1">Reason</div>
								<div className="rounded border p-2">{entry.reason}</div>
							</div>
						)}
						{entry.before_json && (
							<div>
								<div className="text-muted-foreground mb-1">Before</div>
								<pre className="bg-muted max-h-60 overflow-auto rounded p-2 text-xs">{entry.before_json}</pre>
							</div>
						)}
						{entry.after_json && (
							<div>
								<div className="text-muted-foreground mb-1">After</div>
								<pre className="bg-muted max-h-60 overflow-auto rounded p-2 text-xs">{entry.after_json}</pre>
							</div>
						)}
					</div>
				)}
				<DialogFooter>
					<Button variant="outline" onClick={onClose}>Close</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}

export default function AuditLogsView() {
	const hasView = useRbac(RbacResource.AuditLogs, RbacOperation.View);
	const hasDownload = useRbac(RbacResource.AuditLogs, RbacOperation.Download);

	const [actor, setActor] = useState("");
	const [action, setAction] = useState("");
	const [resourceType, setResourceType] = useState("");
	const [outcome, setOutcome] = useState<string>(OUTCOME_ANY);
	const [from, setFrom] = useState("");
	const [to, setTo] = useState("");
	const [offset, setOffset] = useState(0);

	// Filters actually applied to the query — updated when the user hits Apply.
	const [applied, setApplied] = useState<AuditLogFilters>({ limit: PAGE_SIZE, offset: 0 });

	const { data, isLoading, isFetching, error, refetch } = useGetAuditLogsQuery(applied, { skip: !hasView });
	const [detailsEntry, setDetailsEntry] = useState<AuditEntry | null>(null);

	const entries = useMemo(() => data?.entries ?? [], [data]);
	const total = data?.total ?? 0;

	useEffect(() => {
		if (error) toast.error(`Failed to load audit logs: ${getErrorMessage(error)}`);
	}, [error]);

	useEffect(() => {
		setApplied((prev) => ({ ...prev, offset }));
	}, [offset]);

	if (!hasView) return <NoPermissionView entity="audit logs" />;

	const handleApply = () => {
		setOffset(0);
		setApplied({
			limit: PAGE_SIZE,
			offset: 0,
			actor_id: actor.trim() || undefined,
			action: action.trim() || undefined,
			resource_type: resourceType.trim() || undefined,
			outcome: outcome !== OUTCOME_ANY ? (outcome as AuditOutcome) : undefined,
			from: toRFC3339(from),
			to: toRFC3339(to),
		});
	};

	const handleReset = () => {
		setActor("");
		setAction("");
		setResourceType("");
		setOutcome(OUTCOME_ANY);
		setFrom("");
		setTo("");
		setOffset(0);
		setApplied({ limit: PAGE_SIZE, offset: 0 });
	};

	const handleExport = (format: "csv" | "json") => {
		const url = getApiBaseUrl() + buildAuditExportUrl(format, applied).replace(/^\/api/, "");
		window.open(url, "_blank");
	};

	return (
		<div className="space-y-6" data-testid="audit-logs-view">
			<div className="flex items-center justify-between">
				<div>
					<h2 className="text-2xl font-semibold">Audit logs</h2>
					<p className="text-muted-foreground text-sm">
						{total.toLocaleString()} entries. Immutable record of administrative actions.
					</p>
				</div>
				<div className="flex gap-2">
					<Button variant="outline" onClick={() => refetch()} disabled={isFetching} data-testid="audit-refresh">
						<RefreshCw className="mr-2 h-4 w-4" /> Refresh
					</Button>
					{hasDownload && (
						<>
							<Button variant="outline" onClick={() => handleExport("csv")} data-testid="audit-export-csv">
								<Download className="mr-2 h-4 w-4" /> CSV
							</Button>
							<Button variant="outline" onClick={() => handleExport("json")} data-testid="audit-export-json">
								<Download className="mr-2 h-4 w-4" /> JSON
							</Button>
						</>
					)}
				</div>
			</div>

			<div className="grid grid-cols-1 gap-3 rounded-md border p-4 md:grid-cols-3">
				<div className="space-y-1">
					<Label htmlFor="audit-filter-actor">Actor ID</Label>
					<Input
						id="audit-filter-actor"
						data-testid="audit-filter-actor"
						value={actor}
						onChange={(e) => setActor(e.target.value)}
						placeholder="user UUID"
					/>
				</div>
				<div className="space-y-1">
					<Label htmlFor="audit-filter-action">Action</Label>
					<Input
						id="audit-filter-action"
						data-testid="audit-filter-action"
						value={action}
						onChange={(e) => setAction(e.target.value)}
						placeholder="e.g. role.create"
					/>
				</div>
				<div className="space-y-1">
					<Label htmlFor="audit-filter-resource">Resource type</Label>
					<Input
						id="audit-filter-resource"
						data-testid="audit-filter-resource"
						value={resourceType}
						onChange={(e) => setResourceType(e.target.value)}
						placeholder="e.g. role"
					/>
				</div>
				<div className="space-y-1">
					<Label htmlFor="audit-filter-outcome">Outcome</Label>
					<Select value={outcome} onValueChange={setOutcome}>
						<SelectTrigger id="audit-filter-outcome" data-testid="audit-filter-outcome">
							<SelectValue placeholder="Any outcome" />
						</SelectTrigger>
						<SelectContent>
							<SelectItem value={OUTCOME_ANY}>Any</SelectItem>
							{OUTCOMES.map((o) => (
								<SelectItem key={o} value={o}>{o}</SelectItem>
							))}
						</SelectContent>
					</Select>
				</div>
				<div className="space-y-1">
					<Label htmlFor="audit-filter-from">From</Label>
					<Input
						id="audit-filter-from"
						type="datetime-local"
						data-testid="audit-filter-from"
						value={from}
						onChange={(e) => setFrom(e.target.value)}
					/>
				</div>
				<div className="space-y-1">
					<Label htmlFor="audit-filter-to">To</Label>
					<Input
						id="audit-filter-to"
						type="datetime-local"
						data-testid="audit-filter-to"
						value={to}
						onChange={(e) => setTo(e.target.value)}
					/>
				</div>
				<div className="flex items-end gap-2 md:col-span-3">
					<Button onClick={handleApply} data-testid="audit-apply-filters">
						<Search className="mr-2 h-4 w-4" /> Apply filters
					</Button>
					<Button variant="outline" onClick={handleReset} data-testid="audit-reset-filters">
						Reset
					</Button>
				</div>
			</div>

			{isLoading ? (
				<FullPageLoader />
			) : (
				<div className="rounded-md border">
					<Table>
						<TableHeader>
							<TableRow>
								<TableHead className="w-[180px]">Time</TableHead>
								<TableHead>Actor</TableHead>
								<TableHead>Action</TableHead>
								<TableHead>Resource</TableHead>
								<TableHead>Outcome</TableHead>
								<TableHead className="text-right">Details</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{entries.length === 0 ? (
								<TableRow>
									<TableCell colSpan={6} className="text-muted-foreground py-10 text-center" data-testid="audit-empty">
										No audit entries match the current filters.
									</TableCell>
								</TableRow>
							) : (
								entries.map((e) => (
									<TableRow key={e.id} data-testid={`audit-row-${e.id}`}>
										<TableCell className="font-mono text-xs">{formatTime(e.created_at)}</TableCell>
										<TableCell>
											<div className="font-medium">{e.actor_display || e.actor_id || "—"}</div>
											<div className="text-muted-foreground text-xs">{e.actor_type}</div>
										</TableCell>
										<TableCell>{e.action}</TableCell>
										<TableCell className="text-xs">
											<div>{e.resource_type}</div>
											{e.resource_id && <div className="text-muted-foreground font-mono">{e.resource_id}</div>}
										</TableCell>
										<TableCell>{outcomeBadge(e.outcome)}</TableCell>
										<TableCell className="text-right">
											<Button variant="ghost" size="sm" onClick={() => setDetailsEntry(e)} data-testid={`audit-details-${e.id}`}>
												<Eye className="h-4 w-4" />
											</Button>
										</TableCell>
									</TableRow>
								))
							)}
						</TableBody>
					</Table>
				</div>
			)}

			{total > PAGE_SIZE && (
				<div className="flex items-center justify-between text-sm">
					<div>
						Showing {offset + 1}–{Math.min(offset + PAGE_SIZE, total)} of {total.toLocaleString()}
					</div>
					<div className="flex gap-2">
						<Button
							variant="outline"
							size="sm"
							onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
							disabled={offset === 0 || isFetching}
						>
							Prev
						</Button>
						<Button
							variant="outline"
							size="sm"
							onClick={() => setOffset(offset + PAGE_SIZE)}
							disabled={offset + PAGE_SIZE >= total || isFetching}
						>
							Next
						</Button>
					</div>
				</div>
			)}

			<DetailsDialog entry={detailsEntry} onClose={() => setDetailsEntry(null)} />
		</div>
	);
}
