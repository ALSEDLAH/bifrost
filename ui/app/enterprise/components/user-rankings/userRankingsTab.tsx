// User Rankings dashboard tab (spec 003 T008 / US1 + US2).
//
// Displays per-user requests / tokens / cost over the time window the
// dashboard's URL state owns (start_time, end_time — seconds since
// epoch, populated by the dashboard's time picker). Clicking a row
// drills into /workspace/logs pre-filtered to that user_id.

import FullPageLoader from "@/components/fullPageLoader";
import { NoPermissionView } from "@/components/noPermissionView";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useGetUserRankingsQuery } from "@/lib/store";
import { dateUtils } from "@/lib/types/logs";
import type { UserRankingEntry } from "@/lib/types/logs";
import { RbacOperation, RbacResource, useRbac } from "@enterprise/lib";
import { useNavigate } from "@tanstack/react-router";
import { ArrowDown, ArrowUp, Minus } from "lucide-react";
import { parseAsInteger, useQueryStates } from "nuqs";
import { useMemo } from "react";

function formatNumber(n: number): string {
	return n.toLocaleString(undefined, { maximumFractionDigits: 0 });
}

function formatUsd(n: number): string {
	if (n >= 100) return `$${n.toFixed(0)}`;
	if (n >= 1) return `$${n.toFixed(2)}`;
	return `$${n.toFixed(4)}`;
}

function TrendArrow({ pct }: { pct: number | null }) {
	if (pct === null) return <Minus className="text-muted-foreground h-3 w-3" />;
	const abs = Math.abs(pct);
	const label = abs < 0.5 ? "≈" : `${pct >= 0 ? "+" : ""}${pct.toFixed(1)}%`;
	if (pct > 0.5) {
		return (
			<span className="inline-flex items-center gap-1 text-xs text-amber-600 dark:text-amber-400">
				<ArrowUp className="h-3 w-3" />
				{label}
			</span>
		);
	}
	if (pct < -0.5) {
		return (
			<span className="inline-flex items-center gap-1 text-xs text-emerald-600 dark:text-emerald-400">
				<ArrowDown className="h-3 w-3" />
				{label}
			</span>
		);
	}
	return <span className="text-muted-foreground inline-flex items-center gap-1 text-xs">≈</span>;
}

export default function UserRankingsTab() {
	const hasView = useRbac(RbacResource.Users, RbacOperation.View);
	const navigate = useNavigate();

	const [{ start_time, end_time }] = useQueryStates({
		start_time: parseAsInteger.withDefault(Math.floor(Date.now() / 1000) - 24 * 3600),
		end_time: parseAsInteger.withDefault(Math.floor(Date.now() / 1000)),
	});

	const filters = useMemo(
		() => ({
			start_time: dateUtils.toISOString(start_time),
			end_time: dateUtils.toISOString(end_time),
		}),
		[start_time, end_time],
	);

	const { data, isLoading, isFetching, error } = useGetUserRankingsQuery(
		{ filters },
		{ skip: !hasView },
	);

	const rankings = data?.rankings ?? [];

	if (!hasView) return <NoPermissionView entity="user rankings" />;
	if (isLoading) return <FullPageLoader />;

	const handleRowClick = (entry: UserRankingEntry) => {
		navigate({
			to: "/workspace/logs",
			search: {
				user_ids: entry.user_id,
				start_time: start_time,
				end_time: end_time,
			} as unknown as undefined,
		});
	};

	return (
		<div className="space-y-4" data-testid="user-rankings-view">
			<div className="flex items-center justify-between">
				<div>
					<h3 className="text-lg font-semibold">Top users</h3>
					<p className="text-muted-foreground text-sm">
						Ranked by total cost over the selected time range. Click a row to see that user&apos;s requests.
					</p>
				</div>
				{isFetching && !isLoading && (
					<Badge variant="outline" className="animate-pulse">Refreshing…</Badge>
				)}
			</div>

			{error ? (
				<div className="text-destructive text-sm">
					Failed to load user rankings. Check `/api/logs/user-rankings` is reachable.
				</div>
			) : null}

			<div className="rounded-md border">
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>User</TableHead>
							<TableHead className="text-right">Requests</TableHead>
							<TableHead className="text-right">Tokens</TableHead>
							<TableHead className="text-right">Cost</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{rankings.length === 0 ? (
							<TableRow>
								<TableCell colSpan={4} className="text-muted-foreground py-10 text-center" data-testid="user-rankings-empty">
									No user-attributed requests in this time range.
								</TableCell>
							</TableRow>
						) : (
							rankings.map((r) => (
								<TableRow
									key={r.user_id}
									data-testid={`user-rankings-row-${r.user_id}`}
									className="hover:bg-accent/50 cursor-pointer"
									onClick={() => handleRowClick(r)}
								>
									<TableCell className="font-medium">{r.user_id}</TableCell>
									<TableCell className="text-right">
										<div className="flex items-center justify-end gap-2">
											<span className="font-mono">{formatNumber(r.total_requests)}</span>
											<TrendArrow pct={r.trend.has_previous_period ? r.trend.requests_trend : null} />
										</div>
									</TableCell>
									<TableCell className="text-right">
										<div className="flex items-center justify-end gap-2">
											<span className="font-mono">{formatNumber(r.total_tokens)}</span>
											<TrendArrow pct={r.trend.has_previous_period ? r.trend.tokens_trend : null} />
										</div>
									</TableCell>
									<TableCell className="text-right">
										<div className="flex items-center justify-end gap-2">
											<span className="font-mono">{formatUsd(r.total_cost)}</span>
											<TrendArrow pct={r.trend.has_previous_period ? r.trend.cost_trend : null} />
										</div>
									</TableCell>
								</TableRow>
							))
						)}
					</TableBody>
				</Table>
			</div>
		</div>
	);
}
