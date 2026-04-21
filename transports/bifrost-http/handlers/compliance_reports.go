// Compliance reports — /api/reports/* (spec 019).
//
// Phase 1: two aggregate endpoints over ent_audit_entries. Read-only;
// gated by the existing AuditLogs.Read scope.

package handlers

import (
	"strconv"
	"time"

	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/logstore"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
	"gorm.io/gorm"
)

// ComplianceReportsHandler serves /api/reports/*.
type ComplianceReportsHandler struct {
	db     *gorm.DB
	logger schemas.Logger
}

func NewComplianceReportsHandler(db *gorm.DB, logger schemas.Logger) *ComplianceReportsHandler {
	return &ComplianceReportsHandler{db: db, logger: logger}
}

func (h *ComplianceReportsHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.GET("/api/reports/admin-activity", lib.ChainMiddlewares(h.adminActivity, middlewares...))
	r.GET("/api/reports/access-control", lib.ChainMiddlewares(h.accessControl, middlewares...))
}

// clampDays parses ?days=N and clamps to [1, 365]. Default 30.
func clampDays(ctx *fasthttp.RequestCtx) int {
	n, _ := strconv.Atoi(string(ctx.QueryArgs().Peek("days")))
	if n <= 0 {
		n = 30
	}
	if n > 365 {
		n = 365
	}
	return n
}

type activityBucket struct {
	Action  string `json:"action"`
	Outcome string `json:"outcome"`
	Count   int64  `json:"count"`
}

func (h *ComplianceReportsHandler) adminActivity(ctx *fasthttp.RequestCtx) {
	days := clampDays(ctx)
	since := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)

	var rows []activityBucket
	err := h.db.
		Model(&logstore.TableAuditEntry{}).
		Select("action, outcome, COUNT(*) AS count").
		Where("created_at >= ?", since).
		Group("action, outcome").
		Order("count DESC").
		Scan(&rows).Error
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if rows == nil {
		rows = []activityBucket{}
	}
	SendJSON(ctx, map[string]any{
		"window_days": days,
		"since":       since.Format(time.RFC3339),
		"buckets":     rows,
	})
}

type accessControlReport struct {
	WindowDays      int    `json:"window_days"`
	Since           string `json:"since"`
	RoleChanges     int64  `json:"role_changes"`
	RoleAssignments int64  `json:"role_assignments"`
	UserCreates     int64  `json:"user_creates"`
	UserDeletes     int64  `json:"user_deletes"`
	KeyRotations    int64  `json:"key_rotations"`
}

func (h *ComplianceReportsHandler) accessControl(ctx *fasthttp.RequestCtx) {
	days := clampDays(ctx)
	since := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)

	count := func(actionLike string) int64 {
		var n int64
		_ = h.db.
			Model(&logstore.TableAuditEntry{}).
			Where("created_at >= ? AND action LIKE ?", since, actionLike).
			Count(&n).Error
		return n
	}

	resp := accessControlReport{
		WindowDays:      days,
		Since:           since.Format(time.RFC3339),
		RoleChanges:     count("role.%"),     // role.create, role.update, role.delete
		RoleAssignments: count("assignment.%"), // assignment.create, assignment.delete
		UserCreates:     count("user.create"),
		UserDeletes:     count("user.delete"),
		KeyRotations:    count("apikey.rotate"),
	}
	SendJSON(ctx, resp)
}
