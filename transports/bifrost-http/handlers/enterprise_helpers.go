// Shared helpers for enterprise handlers (Train A-E).
//
// This is a sibling file to upstream's handler pattern files. All
// upstream handler functions (governance.go, providers.go, etc.)
// remain untouched per Constitution Principle XI rule 1; enterprise
// handlers share these helpers to avoid reinventing JSON write
// semantics and audit-context construction.

package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/tenancy"
	"github.com/valyala/fasthttp"
)

// writeJSON writes a JSON response with the given status code.
func writeJSON(ctx *fasthttp.RequestCtx, status int, body any) {
	ctx.SetStatusCode(status)
	ctx.SetContentType("application/json")
	buf, err := json.Marshal(body)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		ctx.SetBodyString(`{"error":{"type":"server_error","message":"failed to marshal response"}}`)
		return
	}
	ctx.SetBody(buf)
}

// writeJSONError writes a standard error envelope.
//
//	{"error":{"type":"<errType>","message":"<msg>"}}
func writeJSONError(ctx *fasthttp.RequestCtx, status int, errType, msg string) {
	writeJSON(ctx, status, map[string]any{
		"error": map[string]any{
			"type":    errType,
			"message": msg,
		},
	})
}

// formatMsg is a fmt.Sprintf shim that avoids importing fmt in every
// enterprise handler file.
func formatMsg(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}

// newAuditBctxFromFasthttp reconstructs a minimal BifrostContext
// carrying the caller's TenantContext, suitable for feeding to
// audit.Emit from an HTTP handler. The full BifrostContext (used by
// the plugin chain) is not needed for audit writes — audit only needs
// the tenancy keys.
func newAuditBctxFromFasthttp(ctx *fasthttp.RequestCtx, tc tenancy.TenantContext) *schemas.BifrostContext {
	bctx := schemas.NewBifrostContext(context.Background())
	bctx.SetValue(tenancy.BifrostContextKeyTenantContext, tc)
	bctx.SetValue(tenancy.BifrostContextKeyOrganizationID, tc.OrganizationID)
	bctx.SetValue(tenancy.BifrostContextKeyWorkspaceID, tc.WorkspaceID)
	bctx.SetValue(tenancy.BifrostContextKeyUserID, tc.UserID)
	bctx.SetValue(tenancy.BifrostContextKeyRoleScopes, tc.RoleScopes)
	bctx.SetValue(tenancy.BifrostContextKeyResolvedVia, string(tc.ResolvedVia))
	return bctx
}
