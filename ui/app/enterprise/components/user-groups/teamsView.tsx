// Enterprise teams view — wraps the governance team management.
// governance_teams = workspaces in enterprise mode.

import FullPageLoader from "@/components/fullPageLoader";
import { useDebouncedValue } from "@/hooks/useDebounce";
import { getErrorMessage, useGetCustomersQuery, useGetTeamsQuery, useGetVirtualKeysQuery } from "@/lib/store";
import { RbacOperation, RbacResource, useRbac } from "@enterprise/lib";
import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import TeamsTable from "@/app/workspace/governance/views/teamsTable";

const POLLING_INTERVAL = 5000;
const PAGE_SIZE = 25;

export function TeamsView() {
	const hasVirtualKeysAccess = useRbac(RbacResource.VirtualKeys, RbacOperation.View);
	const hasTeamsAccess = useRbac(RbacResource.Teams, RbacOperation.View);
	const hasCustomersAccess = useRbac(RbacResource.Customers, RbacOperation.View);
	const shownErrorsRef = useRef(new Set<string>());

	const [search, setSearch] = useState("");
	const [offset, setOffset] = useState(0);
	const debouncedSearch = useDebouncedValue(search, 300);

	useEffect(() => { setOffset(0); }, [debouncedSearch]);

	const { data: virtualKeysData, error: vkError, isLoading: vkLoading } = useGetVirtualKeysQuery(undefined, {
		skip: !hasVirtualKeysAccess, pollingInterval: POLLING_INTERVAL,
	});
	const { data: customersData, error: customersError, isLoading: customersLoading } = useGetCustomersQuery(
		undefined, { skip: !hasCustomersAccess, pollingInterval: POLLING_INTERVAL },
	);
	const { data: teamsData, error: teamsError, isLoading: teamsLoading } = useGetTeamsQuery(
		{ limit: PAGE_SIZE, offset, search: debouncedSearch || undefined },
		{ skip: !hasTeamsAccess, pollingInterval: POLLING_INTERVAL },
	);

	const teamsTotal = teamsData?.total_count ?? 0;
	useEffect(() => {
		if (!teamsData || offset < teamsTotal) return;
		setOffset(teamsTotal === 0 ? 0 : Math.floor((teamsTotal - 1) / PAGE_SIZE) * PAGE_SIZE);
	}, [teamsTotal, offset]);

	const isLoading = vkLoading || teamsLoading || customersLoading;

	useEffect(() => {
		if (!vkError && !teamsError && !customersError) { shownErrorsRef.current.clear(); return; }
		const errorKey = `${!!vkError}-${!!teamsError}-${!!customersError}`;
		if (shownErrorsRef.current.has(errorKey)) return;
		shownErrorsRef.current.add(errorKey);
		if (vkError) toast.error(`Failed to load virtual keys: ${getErrorMessage(vkError)}`);
		if (teamsError) toast.error(`Failed to load teams: ${getErrorMessage(teamsError)}`);
		if (customersError) toast.error(`Failed to load customers: ${getErrorMessage(customersError)}`);
	}, [vkError, teamsError, customersError]);

	if (isLoading) return <FullPageLoader />;

	return (
		<div className="mx-auto w-full max-w-7xl">
			<TeamsTable
				teams={teamsData?.teams || []}
				totalCount={teamsData?.total_count || 0}
				customers={customersData?.customers || []}
				virtualKeys={virtualKeysData?.virtual_keys || []}
				search={search}
				debouncedSearch={debouncedSearch}
				onSearchChange={setSearch}
				offset={offset}
				limit={PAGE_SIZE}
				onOffsetChange={setOffset}
			/>
		</div>
	);
}
