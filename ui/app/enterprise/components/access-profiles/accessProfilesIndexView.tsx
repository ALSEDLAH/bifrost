// Enterprise access profiles view (spec 002, US2 reuse win, T005).
//
// Audit verdict: expose (reuse win). Access profiles and roles share
// the same underlying data — named bundles of Resource.Operation
// scopes — so this view surfaces the existing governance roles under
// an access-profile lens rather than inventing a separate store.
//
// Read-only here. Edits flow through the RBAC page (one source of
// truth); this page has a button to navigate there.

import FullPageLoader from "@/components/fullPageLoader";
import { NoPermissionView } from "@/components/noPermissionView";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { getErrorMessage, useGetRolesQuery } from "@/lib/store";
import type { RoleScopeMap } from "@/lib/types/enterprise";
import { RbacOperation, RbacResource, useRbac } from "@enterprise/lib";
import { Link } from "@tanstack/react-router";
import { ExternalLink, InfoIcon } from "lucide-react";
import { useEffect, useMemo } from "react";
import { toast } from "sonner";

function scopeCount(scopes: RoleScopeMap | null | undefined): number {
	if (!scopes) return 0;
	return Object.values(scopes).reduce((sum, ops) => sum + ops.length, 0);
}

function topResources(scopes: RoleScopeMap | null | undefined, n = 4): string[] {
	if (!scopes) return [];
	return Object.entries(scopes)
		.sort(([, a], [, b]) => b.length - a.length)
		.slice(0, n)
		.map(([r]) => r);
}

export default function AccessProfilesIndexView() {
	const hasView = useRbac(RbacResource.AccessProfiles, RbacOperation.View);
	const { data, isLoading, error } = useGetRolesQuery(undefined, { skip: !hasView });

	const profiles = useMemo(() => data?.roles ?? [], [data]);

	useEffect(() => {
		if (error) toast.error(`Failed to load access profiles: ${getErrorMessage(error)}`);
	}, [error]);

	if (!hasView) return <NoPermissionView entity="access profiles" />;
	if (isLoading) return <FullPageLoader />;

	return (
		<div className="space-y-6" data-testid="access-profiles-view">
			<div>
				<h2 className="text-2xl font-semibold">Access profiles</h2>
				<p className="text-muted-foreground text-sm">
					Named scope bundles. Assign to users directly, or attach to admin API keys when scoped-key
					support lands. {profiles.length} profile(s).
				</p>
			</div>

			<Alert>
				<InfoIcon className="h-4 w-4" />
				<AlertTitle>Access profiles are roles</AlertTitle>
				<AlertDescription>
					Profiles share the RBAC role catalog — edits in either screen reflect everywhere. To add
					or change a profile&apos;s scopes, use the roles &amp; permissions page.
					<div className="mt-2">
						<Button asChild variant="outline" size="sm" data-testid="access-profiles-open-rbac">
							<Link to="/workspace/governance/rbac">
								<ExternalLink className="mr-2 h-3 w-3" /> Open roles &amp; permissions
							</Link>
						</Button>
					</div>
				</AlertDescription>
			</Alert>

			<div className="rounded-md border">
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>Profile</TableHead>
							<TableHead>Type</TableHead>
							<TableHead>Scopes</TableHead>
							<TableHead>Top resources</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{profiles.length === 0 ? (
							<TableRow>
								<TableCell colSpan={4} className="text-muted-foreground py-10 text-center">
									No access profiles yet.
								</TableCell>
							</TableRow>
						) : (
							profiles.map((p) => (
								<TableRow key={p.id} data-testid={`access-profile-row-${p.name}`}>
									<TableCell className="font-medium">{p.name}</TableCell>
									<TableCell>
										{p.is_builtin ? <Badge>Built-in</Badge> : <Badge variant="secondary">Custom</Badge>}
									</TableCell>
									<TableCell>{scopeCount(p.scopes)}</TableCell>
									<TableCell>
										<div className="flex flex-wrap gap-1">
											{topResources(p.scopes).map((r) => (
												<Badge key={r} variant="outline" className="text-xs">{r}</Badge>
											))}
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
