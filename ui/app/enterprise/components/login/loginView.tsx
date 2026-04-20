// Enterprise stub: re-exports the OSS fallback.
//
// Surfaces the upstream basic-auth login form. Backend `/api/session/login`
// is build-agnostic — the fallback IS the working admin auth path for
// enterprise until an SSO handler ships under its own spec.

export { default } from "../../../_fallbacks/enterprise/components/login/loginView";
export * from "../../../_fallbacks/enterprise/components/login/loginView";
