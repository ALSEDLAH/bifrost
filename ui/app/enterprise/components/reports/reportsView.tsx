// Compliance reports page (spec 019).
//
// Two cards: last-N-days admin activity (grouped by action/outcome)
// and last-N-days access-control summary. Window selector toggles
// 7/30/90 days. Both endpoints read from ent_audit_entries on every
// load — no caching (NFR-002).

import { NoPermissionView } from "@/components/noPermissionView";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import { useGetAccessControlQuery, useGetAdminActivityQuery } from "@/lib/store/apis";
import { RbacOperation, RbacResource, useRbac } from "@enterprise/lib";
import { Info } from "lucide-react";
import { useState } from "react";

const WINDOWS = [7, 30, 90] as const;

function StatRow({ label, value, loading }: { label: string; value: number; loading: boolean }) {
	return (
		<div className="flex items-baseline justify-between border-b py-2 last:border-b-0">
			<span className="text-muted-foreground text-xs uppercase tracking-wide">{label}</span>
			<span className="font-mono text-sm">{loading ? "…" : value.toLocaleString()}</span>
		</div>
	);
}

export default function ReportsView() {
	const hasView = useRbac(RbacResource.AuditLogs, RbacOperation.Read);
	const [days, setDays] = useState<number>(30);

	const { data: admin, isLoading: isAdminLoading } = useGetAdminActivityQuery(
		{ days },
		{ skip: !hasView },
	);
	const { data: ac, isLoading: isACLoading } = useGetAccessControlQuery(
		{ days },
		{ skip: !hasView },
	);

	if (!hasView) return <NoPermissionView entity="compliance reports" />;

	return (
		<div className="mx-auto w-full max-w-5xl space-y-4" data-testid="reports-view">
			<div className="flex items-start justify-between">
				<div>
					<h2 className="text-lg font-semibold tracking-tight">Compliance Reports</h2>
					<p className="text-muted-foreground text-sm">
						Aggregate views over audit entries for compliance evidence. Numbers reflect live DB state (no caching).
					</p>
				</div>
				<Select value={String(days)} onValueChange={(v) => setDays(Number(v))}>
					<SelectTrigger className="w-[160px]" data-testid="reports-window-selector">
						<SelectValue />
					</SelectTrigger>
					<SelectContent>
						{WINDOWS.map((w) => (
							<SelectItem key={w} value={String(w)}>
								Last {w} days
							</SelectItem>
						))}
					</SelectContent>
				</Select>
			</div>

			<Alert variant="default" className="border-blue-20">
				<Info className="h-4 w-4 text-blue-600" />
				<AlertDescription className="text-xs">
					Phase 1 ships admin-activity + access-control widgets. SOC 2 / GDPR / HIPAA / ISO 27001 framework mappings + export formats are tracked as future work.
				</AlertDescription>
			</Alert>

			<div className="grid grid-cols-1 gap-4 md:grid-cols-2">
				<Card data-testid="reports-access-control-card">
					<CardHeader>
						<CardTitle className="text-base">Access control (last {days}d)</CardTitle>
					</CardHeader>
					<CardContent>
						<StatRow label="Role changes" value={ac?.role_changes ?? 0} loading={isACLoading} />
						<StatRow label="Role assignments" value={ac?.role_assignments ?? 0} loading={isACLoading} />
						<StatRow label="User creates" value={ac?.user_creates ?? 0} loading={isACLoading} />
						<StatRow label="User deletes" value={ac?.user_deletes ?? 0} loading={isACLoading} />
						<StatRow label="Key rotations" value={ac?.key_rotations ?? 0} loading={isACLoading} />
					</CardContent>
				</Card>

				<Card data-testid="reports-admin-activity-card">
					<CardHeader>
						<CardTitle className="text-base">Admin activity (last {days}d)</CardTitle>
					</CardHeader>
					<CardContent className="p-0">
						<Table>
							<TableHeader>
								<TableRow>
									<TableHead>Action</TableHead>
									<TableHead>Outcome</TableHead>
									<TableHead className="text-right">Count</TableHead>
								</TableRow>
							</TableHeader>
							<TableBody>
								{isAdminLoading ? (
									<TableRow>
										<TableCell colSpan={3} className="text-muted-foreground py-6 text-center text-sm">
											Loading…
										</TableCell>
									</TableRow>
								) : (admin?.buckets.length ?? 0) === 0 ? (
									<TableRow>
										<TableCell colSpan={3} className="text-muted-foreground py-6 text-center text-sm">
											No admin activity in the selected window.
										</TableCell>
									</TableRow>
								) : (
									admin?.buckets.map((b, i) => (
										<TableRow key={`${b.action}-${b.outcome}-${i}`}>
											<TableCell className="font-mono text-xs">{b.action}</TableCell>
											<TableCell>
												<Badge variant={b.outcome === "denied" ? "destructive" : "outline"}>
													{b.outcome}
												</Badge>
											</TableCell>
											<TableCell className="text-right font-mono text-sm">
												{b.count.toLocaleString()}
											</TableCell>
										</TableRow>
									))
								)}
							</TableBody>
						</Table>
					</CardContent>
				</Card>
			</div>
		</div>
	);
}
