// Package golden implements the SC-001 byte-identical replay harness.
//
// Run with: go test -tags=golden ./tests/golden/...
//
// Constitution Principle II + SC-001: an existing OSS deployment upgrading
// to the enterprise-capable build with zero config changes MUST experience
// no behavior change. This harness replays a captured corpus and asserts
// per-response equivalence.
//
//go:build golden

package golden

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CapturedRequest is one entry in the corpus.
type CapturedRequest struct {
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body,omitempty"`
}

// CapturedResponse is the paired expected outcome.
type CapturedResponse struct {
	ID         string            `json:"id"`
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       json.RawMessage   `json:"body,omitempty"`
}

// Diff describes a single divergence between a replayed response and the
// expected one. Used by the test runner to produce an actionable failure.
type Diff struct {
	RequestID string
	Field     string // "status_code", "header:<name>", or "body"
	Expected  string
	Actual    string
}

// nonDeterministicHeaders are stripped from both sides before comparison —
// these are expected to vary across replays and do not constitute behavior
// regressions.
var nonDeterministicHeaders = map[string]struct{}{
	"date":             {},
	"x-request-id":     {},
	"x-bifrost-request-id": {},
	"x-trace-id":       {},
	"x-bifrost-latency-ms": {},
	"server":           {},
}

// LoadCorpus reads the captured request/response pairs from corpus/ and
// expected/ directories under root.
func LoadCorpus(root string) ([]CapturedRequest, map[string]CapturedResponse, error) {
	reqDir := filepath.Join(root, "corpus")
	respDir := filepath.Join(root, "expected")

	requests, err := readJSONLDir[CapturedRequest](reqDir)
	if err != nil {
		return nil, nil, fmt.Errorf("read corpus: %w", err)
	}
	responseList, err := readJSONLDir[CapturedResponse](respDir)
	if err != nil {
		return nil, nil, fmt.Errorf("read expected: %w", err)
	}

	respByID := make(map[string]CapturedResponse, len(responseList))
	for _, r := range responseList {
		respByID[r.ID] = r
	}
	return requests, respByID, nil
}

// Replay sends each captured request to the target gateway and collects
// diffs against the expected responses. Returns nil on full-pass.
func Replay(ctx context.Context, target string, reqs []CapturedRequest, expected map[string]CapturedResponse) ([]Diff, error) {
	if !strings.HasPrefix(target, "http") {
		return nil, errors.New("target must be a full URL")
	}
	client := &http.Client{Timeout: 30 * time.Second}

	var diffs []Diff
	for _, req := range reqs {
		exp, ok := expected[req.ID]
		if !ok {
			diffs = append(diffs, Diff{
				RequestID: req.ID,
				Field:     "expected_response",
				Expected:  "present",
				Actual:    "missing",
			})
			continue
		}

		got, err := sendOne(ctx, client, target, req)
		if err != nil {
			diffs = append(diffs, Diff{
				RequestID: req.ID,
				Field:     "transport_error",
				Expected:  "ok",
				Actual:    err.Error(),
			})
			continue
		}
		diffs = append(diffs, compareResponses(req.ID, exp, got)...)
	}
	return diffs, nil
}

func sendOne(ctx context.Context, client *http.Client, target string, req CapturedRequest) (CapturedResponse, error) {
	url := strings.TrimRight(target, "/") + req.Path
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, url, bytes.NewReader(req.Body))
	if err != nil {
		return CapturedResponse{}, err
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return CapturedResponse{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	headers := make(map[string]string, len(resp.Header))
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[strings.ToLower(k)] = v[0]
		}
	}
	return CapturedResponse{
		ID:         req.ID,
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       body,
	}, nil
}

func compareResponses(id string, exp, got CapturedResponse) []Diff {
	var diffs []Diff
	if exp.StatusCode != got.StatusCode {
		diffs = append(diffs, Diff{
			RequestID: id,
			Field:     "status_code",
			Expected:  fmt.Sprintf("%d", exp.StatusCode),
			Actual:    fmt.Sprintf("%d", got.StatusCode),
		})
	}
	for name, expVal := range exp.Headers {
		lname := strings.ToLower(name)
		if _, skip := nonDeterministicHeaders[lname]; skip {
			continue
		}
		if gotVal, ok := got.Headers[lname]; !ok || gotVal != expVal {
			diffs = append(diffs, Diff{
				RequestID: id,
				Field:     "header:" + lname,
				Expected:  expVal,
				Actual:    gotVal,
			})
		}
	}
	if !bytes.Equal(exp.Body, got.Body) {
		diffs = append(diffs, Diff{
			RequestID: id,
			Field:     "body",
			Expected:  truncate(string(exp.Body), 200),
			Actual:    truncate(string(got.Body), 200),
		})
	}
	return diffs
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}

func readJSONLDir[T any](dir string) ([]T, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil // empty corpus is acceptable; harness reports zero entries
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []T
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".jsonl") {
			continue
		}
		f, err := os.Open(filepath.Join(dir, ent.Name()))
		if err != nil {
			return nil, err
		}
		dec := json.NewDecoder(f)
		for {
			var item T
			if err := dec.Decode(&item); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				_ = f.Close()
				return nil, err
			}
			out = append(out, item)
		}
		_ = f.Close()
	}
	return out, nil
}
