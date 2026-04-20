// Enterprise adaptive routing view (spec 002, US2 reuse win, T006).
//
// Audit verdict: expose (reuse win). Upstream governance routing-rules
// already implement weighted-target traffic splitting (canary-style).
// This view surfaces the existing `/api/governance/routing-rules`
// endpoint with a canary-focused lens: it filters to rules that have
// *more than one* target (the interesting-for-canary subset). Writes
// stay on the canonical `/workspace/routing-rules` page — one editor,
// no UX divergence.

import FullPageLoader from "@/components/fullPageLoader";
import { NoPermissionView } from "@/components/noPermissionView";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { getErrorMessage } from "@/lib/store";
import { useGetRoutingRulesQuery } from "@/lib/store/apis/routingRulesApi";
import type { RoutingRule, RoutingTarget } from "@/lib/types/routingRules";
import { RbacOperation, RbacResource, useRbac } from "@enterprise/lib";
import { Link } from "@tanstack/react-router";
import { ExternalLink, InfoIcon, SplitSquareHorizontal } from "lucide-react";
import { useEffect, useMemo } from "react";
import { toast } from "sonner";

function targetLabel(t: RoutingTarget): string {
	const parts: string[] = [];
	if (t.provider) parts.push(t.provider);
	if (t.model) parts.push(t.model);
	if (!parts.length) parts.push("incoming");
	return parts.join(" / ");
}

function weightPct(t: RoutingTarget): string {
	const pct = Math.round(t.weight * 100);
	return `${pct}%`;
}

export default function AdaptiveRoutingView() {
	const hasView = useRbac(RbacResource.AdaptiveRouter, RbacOperation.View);
	const hasGovernanceView = useRbac(RbacResource.RoutingRules, RbacOperation.View);
	const canView = hasView || hasGovernanceView;

	const { data, isLoading, error } = useGetRoutingRulesQuery(undefined, { skip: !canView });

	// Adaptive routing focuses on rules with *more than one* target — the
	// interesting-for-canary subset. Single-target rules are pure pins
	// and live on the canonical routing-rules page.
	const canaryRules = useMemo<RoutingRule[]>(() => {
		return (data?.rules ?? []).filter((r) => (r.targets?.length ?? 0) > 1);
	}, [data]);

	useEffect(() => {
		if (error) toast.error(`Failed to load routing rules: ${getErrorMessage(error)}`);
	}, [error]);

	if (!canView) return <NoPermissionView entity="adaptive routing" />;
	if (isLoading) return <FullPageLoader />;

	return (
		<div className="space-y-6" data-testid="adaptive-routing-view">
			<div className="flex items-start justify-between gap-4">
				<div>
					<div className="flex items-center gap-3">
						<SplitSquareHorizontal className="h-6 w-6" />
						<h2 className="text-2xl font-semibold">Adaptive routing</h2>
					</div>
					<p className="text-muted-foreground mt-1 text-sm">
						Weighted traffic splitting across providers, models, and keys — the canary subset of
						governance routing rules. {canaryRules.length} rule(s) currently split traffic across
						multiple targets.
					</p>
				</div>
				<Button asChild variant="outline" data-testid="adaptive-routing-open-editor">
					<Link to="/workspace/routing-rules">
						<ExternalLink className="mr-2 h-4 w-4" /> Edit rules
					</Link>
				</Button>
			</div>

			<Alert>
				<InfoIcon className="h-4 w-4" />
				<AlertTitle>Adaptive routing reuses routing rules</AlertTitle>
				<AlertDescription>
					Every rule you see here is a governance routing rule with more than one target. Edits
					happen on the canonical routing-rules editor — click <strong>Edit rules</strong> above.
					Single-target rules are pure pins and don&apos;t appear here.
				</AlertDescription>
			</Alert>

			<div className="rounded-md border">
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>Rule</TableHead>
							<TableHead>Scope</TableHead>
							<TableHead>Status</TableHead>
							<TableHead>Split</TableHead>
							<TableHead className="w-[90px] text-right">Priority</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{canaryRules.length === 0 ? (
							<TableRow>
								<TableCell colSpan={5} className="text-muted-foreground py-10 text-center">
									No adaptive rules yet — create a routing rule with multiple targets on the{" "}
									<Link to="/workspace/routing-rules" className="underline">
										routing-rules page
									</Link>{" "}
									to start splitting traffic.
								</TableCell>
							</TableRow>
						) : (
							canaryRules.map((rule) => (
								<TableRow key={rule.id} data-testid={`adaptive-routing-row-${rule.name}`}>
									<TableCell className="font-medium">
										<div>{rule.name}</div>
										{rule.description && (
											<div className="text-muted-foreground text-xs">{rule.description}</div>
										)}
									</TableCell>
									<TableCell>
										<Badge variant="outline">{rule.scope}</Badge>
									</TableCell>
									<TableCell>
										{rule.enabled ? (
											<Badge>Enabled</Badge>
										) : (
											<Badge variant="secondary">Disabled</Badge>
										)}
									</TableCell>
									<TableCell className="space-y-1 text-xs">
										{rule.targets.map((t, i) => (
											<div key={i} className="flex items-center gap-2">
												<span className="font-mono w-12">{weightPct(t)}</span>
												<span>{targetLabel(t)}</span>
											</div>
										))}
									</TableCell>
									<TableCell className="text-right font-mono">{rule.priority}</TableCell>
								</TableRow>
							))
						)}
					</TableBody>
				</Table>
			</div>
		</div>
	);
}
