// Real enterprise RBAC context (US2, T023).
//
// Fetches the current user's resolved scopes from /api/rbac/me and
// exposes useRbac(resource, operation) + useRbacContext() to the app.
// The RbacResource / RbacOperation enums are re-exported from the
// fallback so that every enterprise-side import resolves through this
// file without duplicating the enum definitions.

import { useGetRbacMeQuery } from "@/lib/store/apis/enterpriseApi";
import { createContext, useContext, useMemo, type ReactNode } from "react";
import {
	RbacOperation,
	RbacResource,
} from "../../../_fallbacks/enterprise/lib/contexts/rbacContext";

export { RbacOperation, RbacResource };

interface RbacContextType {
	isAllowed: (resource: RbacResource, operation: RbacOperation) => boolean;
	permissions: Record<string, Record<string, boolean>>;
	isLoading: boolean;
	refetch: () => void;
}

const RbacContext = createContext<RbacContextType | null>(null);

function buildPermissions(perms: Record<string, string[]> | undefined): Record<string, Record<string, boolean>> {
	const out: Record<string, Record<string, boolean>> = {};
	if (!perms) return out;
	for (const [resource, ops] of Object.entries(perms)) {
		out[resource] = {};
		for (const op of ops) out[resource][op] = true;
	}
	return out;
}

export function RbacProvider({ children }: { children: ReactNode }) {
	const { data, isLoading, refetch } = useGetRbacMeQuery();

	const value = useMemo<RbacContextType>(() => {
		const permissions = buildPermissions(data?.permissions);
		const hasWildcard = data?.scopes?.includes("*") ?? false;
		const isAllowed = (resource: RbacResource, operation: RbacOperation) => {
			if (hasWildcard) return true;
			return permissions[resource]?.[operation] === true;
		};
		return { isAllowed, permissions, isLoading, refetch: () => { void refetch(); } };
	}, [data, isLoading, refetch]);

	return <RbacContext.Provider value={value}>{children}</RbacContext.Provider>;
}

export function useRbac(resource: RbacResource, operation: RbacOperation): boolean {
	const ctx = useContext(RbacContext);
	if (!ctx) return true;
	return ctx.isAllowed(resource, operation);
}

export function useRbacContext(): RbacContextType {
	const ctx = useContext(RbacContext);
	if (ctx) return ctx;
	return {
		isAllowed: () => true,
		permissions: {},
		isLoading: false,
		refetch: () => {},
	};
}
