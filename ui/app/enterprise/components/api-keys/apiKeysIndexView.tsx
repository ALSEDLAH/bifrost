// Enterprise stub: re-exports the OSS fallback.
//
// The fallback surfaces the upstream admin credential (basic auth via
// `auth_config`) — that IS the existing admin auth path. No parallel
// multi-key system is built here; an enterprise-only scoped admin-key
// system was explicitly descoped 2026-04-20.

export { default } from "../../../_fallbacks/enterprise/components/api-keys/apiKeysIndexView";
export * from "../../../_fallbacks/enterprise/components/api-keys/apiKeysIndexView";
