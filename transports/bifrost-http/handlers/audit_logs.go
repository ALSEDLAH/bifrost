// Audit log query handler — /api/audit-logs endpoint (US4, T026).

package handlers

import (
	"encoding/csv"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/logstore"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
	"gorm.io/gorm"
)

// AuditLogsHandler serves /api/audit-logs.
type AuditLogsHandler struct {
	db     *gorm.DB
	logger schemas.Logger
}

// NewAuditLogsHandler constructs the handler.
func NewAuditLogsHandler(db *gorm.DB, logger schemas.Logger) *AuditLogsHandler {
	return &AuditLogsHandler{db: db, logger: logger}
}

// RegisterRoutes wires audit log endpoints.
func (h *AuditLogsHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.GET("/api/audit-logs", lib.ChainMiddlewares(h.handleList, middlewares...))
	r.GET("/api/audit-logs/export", lib.ChainMiddlewares(h.handleExport, middlewares...))
}

func (h *AuditLogsHandler) handleList(ctx *fasthttp.RequestCtx) {
	query := h.db.Model(&logstore.TableAuditEntry{})
	query = h.applyFilters(ctx, query)

	limit, _ := strconv.Atoi(string(ctx.QueryArgs().Peek("limit")))
	offset, _ := strconv.Atoi(string(ctx.QueryArgs().Peek("offset")))
	if limit <= 0 || limit > 500 {
		limit = 50
	}

	var total int64
	query.Count(&total)

	var entries []logstore.TableAuditEntry
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&entries).Error; err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, "failed to query audit logs")
		return
	}

	result := map[string]any{
		"entries": entries,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	}

	ctx.SetContentType("application/json")
	buf, _ := json.Marshal(result)
	ctx.SetBody(buf)
}

func (h *AuditLogsHandler) handleExport(ctx *fasthttp.RequestCtx) {
	format := string(ctx.QueryArgs().Peek("format"))
	if format == "" {
		format = "json"
	}

	query := h.db.Model(&logstore.TableAuditEntry{})
	query = h.applyFilters(ctx, query)

	var entries []logstore.TableAuditEntry
	if err := query.Order("created_at DESC").Find(&entries).Error; err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, "failed to export audit logs")
		return
	}

	switch strings.ToLower(format) {
	case "csv":
		ctx.SetContentType("text/csv")
		ctx.Response.Header.Set("Content-Disposition", "attachment; filename=audit-logs.csv")
		w := csv.NewWriter(ctx)
		_ = w.Write([]string{"id", "created_at", "actor_type", "actor_id", "actor_display", "actor_ip", "action", "resource_type", "resource_id", "outcome", "reason"})
		for _, e := range entries {
			_ = w.Write([]string{e.ID, e.CreatedAt.Format(time.RFC3339), e.ActorType, e.ActorID, e.ActorDisplay, e.ActorIP, e.Action, e.ResourceType, e.ResourceID, e.Outcome, e.Reason})
		}
		w.Flush()
	default:
		ctx.SetContentType("application/json")
		ctx.Response.Header.Set("Content-Disposition", "attachment; filename=audit-logs.json")
		buf, _ := json.Marshal(entries)
		ctx.SetBody(buf)
	}
}

func (h *AuditLogsHandler) applyFilters(ctx *fasthttp.RequestCtx, query *gorm.DB) *gorm.DB {
	if v := string(ctx.QueryArgs().Peek("actor_id")); v != "" {
		query = query.Where("actor_id = ?", v)
	}
	if v := string(ctx.QueryArgs().Peek("action")); v != "" {
		query = query.Where("action = ?", v)
	}
	if v := string(ctx.QueryArgs().Peek("resource_type")); v != "" {
		query = query.Where("resource_type = ?", v)
	}
	if v := string(ctx.QueryArgs().Peek("outcome")); v != "" {
		query = query.Where("outcome = ?", v)
	}
	if v := string(ctx.QueryArgs().Peek("from")); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if v := string(ctx.QueryArgs().Peek("to")); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			query = query.Where("created_at <= ?", t)
		}
	}
	if v := string(ctx.QueryArgs().Peek("organization_id")); v != "" {
		query = query.Where("organization_id = ?", v)
	}
	return query
}
