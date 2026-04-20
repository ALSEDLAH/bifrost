package logstore

import (
	"context"
	"testing"
	"time"
)

// TestRDBLogStore_GetUserRankings seeds three users plus one empty-user_id
// row and asserts totals, ordering by total_requests, and trend computation
// against a hand-calculated expectation (spec 003 T006).
func TestRDBLogStore_GetUserRankings(t *testing.T) {
	store := newTestSQLiteStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	windowStart := now.Add(-1 * time.Hour)
	windowEnd := now
	prevOffset := 30 * time.Minute // inside [windowStart-1h, windowStart)

	alice := "user-alice"
	bob := "user-bob"
	carol := "user-carol"

	cost := func(v float64) *float64 { return &v }

	current := []*Log{
		{ID: "a1", Timestamp: windowStart.Add(5 * time.Minute), UserID: &alice, TotalTokens: 100, Cost: cost(0.01)},
		{ID: "a2", Timestamp: windowStart.Add(10 * time.Minute), UserID: &alice, TotalTokens: 200, Cost: cost(0.02)},
		{ID: "a3", Timestamp: windowStart.Add(15 * time.Minute), UserID: &alice, TotalTokens: 300, Cost: cost(0.03)},
		{ID: "b1", Timestamp: windowStart.Add(20 * time.Minute), UserID: &bob, TotalTokens: 50, Cost: cost(0.005)},
		{ID: "b2", Timestamp: windowStart.Add(25 * time.Minute), UserID: &bob, TotalTokens: 50, Cost: cost(0.005)},
		{ID: "c1", Timestamp: windowStart.Add(30 * time.Minute), UserID: &carol, TotalTokens: 1000, Cost: cost(0.10)},
		// Empty user_id → must be excluded from rankings.
		{ID: "nouser", Timestamp: windowStart.Add(35 * time.Minute), UserID: nil, TotalTokens: 9999, Cost: cost(9.99)},
	}

	previous := []*Log{
		// Alice had 2 requests / 400 tokens / $0.04 in prior period.
		{ID: "a-prev-1", Timestamp: windowStart.Add(-prevOffset), UserID: &alice, TotalTokens: 150, Cost: cost(0.015)},
		{ID: "a-prev-2", Timestamp: windowStart.Add(-prevOffset + 5*time.Minute), UserID: &alice, TotalTokens: 250, Cost: cost(0.025)},
		// Bob and Carol have no prior-period data.
	}

	for _, logs := range [][]*Log{current, previous} {
		for _, entry := range logs {
			entry.Object = "chat.completion"
			entry.Provider = "openai"
			entry.Model = "gpt-4o-mini"
			entry.Status = "success"
			if err := store.Create(ctx, entry); err != nil {
				t.Fatalf("seed Create(%s) error = %v", entry.ID, err)
			}
		}
	}

	result, err := store.GetUserRankings(ctx, SearchFilters{
		StartTime: &windowStart,
		EndTime:   &windowEnd,
	})
	if err != nil {
		t.Fatalf("GetUserRankings() error = %v", err)
	}

	if got, want := len(result.Rankings), 3; got != want {
		t.Fatalf("len(rankings) = %d, want %d (empty user_id row must be excluded)", got, want)
	}

	byID := make(map[string]UserRankingWithTrend, len(result.Rankings))
	for _, r := range result.Rankings {
		byID[r.UserID] = r
	}

	// Ordering: total_requests DESC → alice(3), bob(2), carol(1).
	wantOrder := []string{alice, bob, carol}
	for i, want := range wantOrder {
		if got := result.Rankings[i].UserID; got != want {
			t.Errorf("rankings[%d].UserID = %q, want %q", i, got, want)
		}
	}

	cases := []struct {
		userID               string
		wantRequests         int64
		wantTokens           int64
		wantCost             float64
		wantHasPrev          bool
		wantRequestsTrendPct float64
	}{
		{alice, 3, 600, 0.06, true, 50.0},
		{bob, 2, 100, 0.01, false, 0},
		{carol, 1, 1000, 0.10, false, 0},
	}

	for _, c := range cases {
		got, ok := byID[c.userID]
		if !ok {
			t.Fatalf("missing ranking for user %q", c.userID)
		}
		if got.TotalRequests != c.wantRequests {
			t.Errorf("%s: TotalRequests = %d, want %d", c.userID, got.TotalRequests, c.wantRequests)
		}
		if got.TotalTokens != c.wantTokens {
			t.Errorf("%s: TotalTokens = %d, want %d", c.userID, got.TotalTokens, c.wantTokens)
		}
		if !approxEqual(got.TotalCost, c.wantCost, 1e-9) {
			t.Errorf("%s: TotalCost = %v, want %v", c.userID, got.TotalCost, c.wantCost)
		}
		if got.Trend.HasPreviousPeriod != c.wantHasPrev {
			t.Errorf("%s: Trend.HasPreviousPeriod = %v, want %v", c.userID, got.Trend.HasPreviousPeriod, c.wantHasPrev)
		}
		if c.wantHasPrev && !approxEqual(got.Trend.RequestsTrend, c.wantRequestsTrendPct, 1e-9) {
			t.Errorf("%s: Trend.RequestsTrend = %v, want %v", c.userID, got.Trend.RequestsTrend, c.wantRequestsTrendPct)
		}
	}
}

func approxEqual(a, b, eps float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= eps
}
