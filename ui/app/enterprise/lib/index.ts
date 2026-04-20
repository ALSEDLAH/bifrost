// Enterprise lib barrel — re-exports enterprise contexts + store utilities
// plus passes through the OSS fallback exports (oauth token helpers, base
// query) that both builds share.

export {
	REFRESH_TOKEN_ENDPOINT,
	clearOAuthStorage,
	clearUserInfo,
	getAccessToken,
	getRefreshState,
	getRefreshToken,
	getTokenExpiry,
	getUserInfo,
	isTokenExpired,
	setOAuthTokens,
	setRefreshState,
	setUserInfo,
	type UserInfo,
} from "../../_fallbacks/enterprise/lib/store/utils/tokenManager";

export { createBaseQueryWithRefresh } from "../../_fallbacks/enterprise/lib/store/utils/baseQueryWithRefresh";

// Real enterprise RBAC context (supersedes the always-allow fallback).
export { RbacOperation, RbacProvider, RbacResource, useRbac, useRbacContext } from "./contexts/rbacContext";
