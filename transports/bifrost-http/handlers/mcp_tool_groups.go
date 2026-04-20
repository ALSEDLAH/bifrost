// MCP Tool Groups handlers — /api/mcp/tool-groups (spec 005).

package handlers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	tables_enterprise "github.com/maximhq/bifrost/framework/configstore/tables-enterprise"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

type MCPToolGroupsHandler struct {
	store  configstore.ConfigStore
	logger schemas.Logger
}

func NewMCPToolGroupsHandler(store configstore.ConfigStore, logger schemas.Logger) *MCPToolGroupsHandler {
	return &MCPToolGroupsHandler{store: store, logger: logger}
}

func (h *MCPToolGroupsHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.GET("/api/mcp/tool-groups", lib.ChainMiddlewares(h.list, middlewares...))
	r.POST("/api/mcp/tool-groups", lib.ChainMiddlewares(h.create, middlewares...))
	r.PATCH("/api/mcp/tool-groups/{id}", lib.ChainMiddlewares(h.update, middlewares...))
	r.DELETE("/api/mcp/tool-groups/{id}", lib.ChainMiddlewares(h.delete, middlewares...))
}

type mcpToolRef struct {
	MCPClientID string `json:"mcp_client_id"`
	ToolName    string `json:"tool_name"`
}

type mcpToolGroupRequest struct {
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Tools       []mcpToolRef `json:"tools,omitempty"`
}

func (h *MCPToolGroupsHandler) list(ctx *fasthttp.RequestCtx) {
	groups, err := h.store.ListMCPToolGroups(ctx)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if groups == nil {
		groups = []tables_enterprise.TableMCPToolGroup{}
	}
	SendJSON(ctx, map[string]any{"groups": groups})
}

func (h *MCPToolGroupsHandler) create(ctx *fasthttp.RequestCtx) {
	var req mcpToolGroupRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid body: %v", err))
		return
	}
	if req.Name == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "name is required")
		return
	}
	// Case-insensitive uniqueness (spec 005 FR-001).
	if existing, _ := h.store.GetMCPToolGroupByName(ctx, req.Name); existing != nil {
		SendError(ctx, fasthttp.StatusConflict, fmt.Sprintf("a tool group named %q already exists (case-insensitive)", req.Name))
		return
	}
	toolsJSON, err := marshalTools(req.Tools)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, err.Error())
		return
	}
	g := &tables_enterprise.TableMCPToolGroup{
		Name:        req.Name,
		Description: req.Description,
		Tools:       toolsJSON,
	}
	if err := h.store.CreateMCPToolGroup(ctx, g); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	SendJSONWithStatus(ctx, g, fasthttp.StatusCreated)
}

func (h *MCPToolGroupsHandler) update(ctx *fasthttp.RequestCtx) {
	id := ctx.UserValue("id").(string)
	existing, err := h.store.GetMCPToolGroupByID(ctx, id)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		SendError(ctx, fasthttp.StatusNotFound, "tool group not found")
		return
	}
	var req mcpToolGroupRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid body: %v", err))
		return
	}
	if req.Name != "" && !strings.EqualFold(req.Name, existing.Name) {
		if conflict, _ := h.store.GetMCPToolGroupByName(ctx, req.Name); conflict != nil && conflict.ID != existing.ID {
			SendError(ctx, fasthttp.StatusConflict, fmt.Sprintf("a tool group named %q already exists (case-insensitive)", req.Name))
			return
		}
		existing.Name = req.Name
	} else if req.Name != "" {
		// Same name, different case — accept as rename-to-normalize.
		existing.Name = req.Name
	}
	if req.Description != "" || ctxBodyHasKey(ctx.PostBody(), "description") {
		existing.Description = req.Description
	}
	if ctxBodyHasKey(ctx.PostBody(), "tools") {
		toolsJSON, err := marshalTools(req.Tools)
		if err != nil {
			SendError(ctx, fasthttp.StatusBadRequest, err.Error())
			return
		}
		existing.Tools = toolsJSON
	}
	if err := h.store.UpdateMCPToolGroup(ctx, existing); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	SendJSON(ctx, existing)
}

func (h *MCPToolGroupsHandler) delete(ctx *fasthttp.RequestCtx) {
	id := ctx.UserValue("id").(string)
	if err := h.store.DeleteMCPToolGroup(ctx, id); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func marshalTools(tools []mcpToolRef) (string, error) {
	if tools == nil {
		tools = []mcpToolRef{}
	}
	for i, t := range tools {
		if t.MCPClientID == "" || t.ToolName == "" {
			return "", fmt.Errorf("tools[%d]: mcp_client_id and tool_name are required", i)
		}
	}
	raw, err := json.Marshal(tools)
	if err != nil {
		return "", fmt.Errorf("marshal tools: %w", err)
	}
	return string(raw), nil
}

// ctxBodyHasKey checks whether the JSON body contains the given top-level
// key. Used to distinguish "field omitted" from "field set to empty string"
// on PATCH requests where the zero value has user-meaningful semantics.
func ctxBodyHasKey(body []byte, key string) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return false
	}
	_, ok := m[key]
	return ok
}
