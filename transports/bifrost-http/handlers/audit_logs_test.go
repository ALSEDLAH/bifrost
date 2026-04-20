// Tests for handlers/audit_logs.go (Constitution Principle VIII).
//
// Seeds ent_audit_entries directly via the store's GORM handle and
// exercises the list + export endpoints across the filter dimensions
// the UI actually sends (actor_id, action, resource_type, outcome,
// from, to).

package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/maximhq/bifrost/framework/logstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func setupAuditLogsHandler(t *testing.T) (*AuditLogsHandler, func(entry logstore.TableAuditEntry)) {
	t.Helper()
	SetLogger(&mockLogger{})

	dbPath := t.TempDir() + "/config.db"
	store, err := configstore.NewConfigStore(context.Background(), &configstore.Config{
		Enabled: true,
		Type:    configstore.ConfigStoreTypeSQLite,
		Config:  &configstore.SQLiteConfig{Path: dbPath},
	}, &mockLogger{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(dbPath) })

	// Enterprise logstore migration creates ent_audit_entries against the
	// configstore DB (mirrors what server.go does in production).
	require.NoError(t, logstore.RegisterEnterpriseMigrations(context.Background(), store.DB()))

	handler := NewAuditLogsHandler(store.DB(), &mockLogger{})
	seed := func(entry logstore.TableAuditEntry) {
		if entry.ID == "" {
			entry.ID = uuid.NewString()
		}
		if entry.CreatedAt.IsZero() {
			entry.CreatedAt = time.Now().UTC()
		}
		require.NoError(t, store.DB().Create(&entry).Error)
	}
	return handler, seed
}

func auditReq(query string) *fasthttp.RequestCtx {
	var req fasthttp.Request
	req.Header.SetMethod("GET")
	req.SetRequestURI("/api/audit-logs?" + query)
	ctx := &fasthttp.RequestCtx{}
	ctx.Init(&req, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}, nil)
	return ctx
}

// ---- List ---------------------------------------------------------

func TestAuditLogs_List_EmptyByDefault(t *testing.T) {
	h, _ := setupAuditLogsHandler(t)
	ctx := auditReq("")
	h.handleList(ctx)

	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	var got map[string]any
	require.NoError(t, json.Unmarshal(ctx.Response.Body(), &got))
	assert.Equal(t, float64(0), got["total"])
	assert.Equal(t, float64(50), got["limit"])
	assert.Equal(t, float64(0), got["offset"])
}

func TestAuditLogs_List_PaginationAndOrdering(t *testing.T) {
	h, seed := setupAuditLogsHandler(t)

	// Seed 5 entries with distinct timestamps.
	base := time.Now().UTC().Add(-1 * time.Hour)
	for i := 0; i < 5; i++ {
		seed(logstore.TableAuditEntry{
			Action:       "role.create",
			ResourceType: "role",
			ResourceID:   uuid.NewString(),
			Outcome:      "allowed",
			CreatedAt:    base.Add(time.Duration(i) * time.Minute),
		})
	}

	// limit=2, offset=0 -> the 2 most recent
	ctx := auditReq("limit=2&offset=0")
	h.handleList(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	var page map[string]any
	require.NoError(t, json.Unmarshal(ctx.Response.Body(), &page))
	assert.Equal(t, float64(5), page["total"])
	entries := page["entries"].([]any)
	require.Len(t, entries, 2)
	// DESC order: the newest (base+4min) first.
	first := entries[0].(map[string]any)
	second := entries[1].(map[string]any)
	assert.True(t, first["created_at"].(string) > second["created_at"].(string))
}

func TestAuditLogs_List_FilterByAction(t *testing.T) {
	h, seed := setupAuditLogsHandler(t)

	seed(logstore.TableAuditEntry{Action: "role.create", ResourceType: "role", Outcome: "allowed"})
	seed(logstore.TableAuditEntry{Action: "role.delete", ResourceType: "role", Outcome: "allowed"})
	seed(logstore.TableAuditEntry{Action: "user.create", ResourceType: "user", Outcome: "allowed"})

	ctx := auditReq("action=role.create")
	h.handleList(ctx)
	var page map[string]any
	_ = json.Unmarshal(ctx.Response.Body(), &page)
	assert.Equal(t, float64(1), page["total"])
}

func TestAuditLogs_List_FilterByResourceType(t *testing.T) {
	h, seed := setupAuditLogsHandler(t)

	seed(logstore.TableAuditEntry{Action: "role.create", ResourceType: "role", Outcome: "allowed"})
	seed(logstore.TableAuditEntry{Action: "user.create", ResourceType: "user", Outcome: "allowed"})

	ctx := auditReq("resource_type=user")
	h.handleList(ctx)
	var page map[string]any
	_ = json.Unmarshal(ctx.Response.Body(), &page)
	assert.Equal(t, float64(1), page["total"])
}

func TestAuditLogs_List_FilterByOutcome(t *testing.T) {
	h, seed := setupAuditLogsHandler(t)

	seed(logstore.TableAuditEntry{Action: "role.create", ResourceType: "role", Outcome: "allowed"})
	seed(logstore.TableAuditEntry{Action: "role.delete", ResourceType: "role", Outcome: "denied"})

	ctx := auditReq("outcome=denied")
	h.handleList(ctx)
	var page map[string]any
	_ = json.Unmarshal(ctx.Response.Body(), &page)
	assert.Equal(t, float64(1), page["total"])
}

func TestAuditLogs_List_FilterByDateRange(t *testing.T) {
	h, seed := setupAuditLogsHandler(t)

	old := time.Now().UTC().Add(-48 * time.Hour)
	recent := time.Now().UTC().Add(-30 * time.Minute)
	seed(logstore.TableAuditEntry{Action: "a", ResourceType: "r", Outcome: "allowed", CreatedAt: old})
	seed(logstore.TableAuditEntry{Action: "b", ResourceType: "r", Outcome: "allowed", CreatedAt: recent})

	from := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	ctx := auditReq("from=" + from)
	h.handleList(ctx)
	var page map[string]any
	_ = json.Unmarshal(ctx.Response.Body(), &page)
	assert.Equal(t, float64(1), page["total"])
}

func TestAuditLogs_List_BadDateIsIgnored(t *testing.T) {
	h, seed := setupAuditLogsHandler(t)
	seed(logstore.TableAuditEntry{Action: "a", ResourceType: "r", Outcome: "allowed"})

	// Unparseable "from" must be silently skipped, not produce a 500.
	ctx := auditReq("from=not-a-date")
	h.handleList(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	var page map[string]any
	_ = json.Unmarshal(ctx.Response.Body(), &page)
	assert.Equal(t, float64(1), page["total"])
}

// ---- Export -------------------------------------------------------

func TestAuditLogs_Export_JSONDefault(t *testing.T) {
	h, seed := setupAuditLogsHandler(t)
	seed(logstore.TableAuditEntry{Action: "role.create", ResourceType: "role", Outcome: "allowed"})

	ctx := auditReq("")
	h.handleExport(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	assert.Equal(t, "application/json", string(ctx.Response.Header.ContentType()))
	assert.Contains(t, string(ctx.Response.Header.Peek("Content-Disposition")), "attachment; filename=audit-logs.json")

	var rows []map[string]any
	require.NoError(t, json.Unmarshal(ctx.Response.Body(), &rows))
	assert.Len(t, rows, 1)
}

func TestAuditLogs_Export_CSVHeaderAndRows(t *testing.T) {
	h, seed := setupAuditLogsHandler(t)
	seed(logstore.TableAuditEntry{
		Action:       "role.create",
		ResourceType: "role",
		ResourceID:   "role-1",
		ActorType:    "user",
		ActorID:      "alice",
		Outcome:      "allowed",
	})

	ctx := auditReq("format=csv")
	h.handleExport(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	assert.Equal(t, "text/csv", string(ctx.Response.Header.ContentType()))
	assert.Contains(t, string(ctx.Response.Header.Peek("Content-Disposition")), "attachment; filename=audit-logs.csv")

	r := csv.NewReader(strings.NewReader(string(ctx.Response.Body())))
	records, err := r.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 2, "expected header + 1 row")
	// Header
	assert.Equal(t, []string{"id", "created_at", "actor_type", "actor_id", "actor_display", "actor_ip", "action", "resource_type", "resource_id", "outcome", "reason"}, records[0])
	// Row content
	row := records[1]
	assert.Equal(t, "user", row[2])
	assert.Equal(t, "alice", row[3])
	assert.Equal(t, "role.create", row[6])
	assert.Equal(t, "role", row[7])
	assert.Equal(t, "role-1", row[8])
	assert.Equal(t, "allowed", row[9])
}

func TestAuditLogs_Export_EmptyResultReturnsHeaderOnly(t *testing.T) {
	h, _ := setupAuditLogsHandler(t)

	ctx := auditReq("format=csv")
	h.handleExport(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())

	r := csv.NewReader(strings.NewReader(string(ctx.Response.Body())))
	records, err := r.ReadAll()
	require.NoError(t, err)
	assert.Len(t, records, 1, "CSV export of empty result should emit header row only")
}

func TestAuditLogs_Export_RespectsOutcomeFilter(t *testing.T) {
	h, seed := setupAuditLogsHandler(t)
	seed(logstore.TableAuditEntry{Action: "a1", ResourceType: "r", Outcome: "allowed"})
	seed(logstore.TableAuditEntry{Action: "d1", ResourceType: "r", Outcome: "denied"})

	ctx := auditReq("outcome=denied")
	h.handleExport(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(ctx.Response.Body(), &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, "denied", rows[0]["outcome"])
}
