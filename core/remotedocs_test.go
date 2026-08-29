package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─── Test infrastructure ─────────────────────────────────────────

// docsTestTransport is a scripted http.RoundTripper: it records every request
// URL and answers from script (nil script = every attempt fails). Remote-doc
// tests run sequentially, so no locking is needed.
type docsTestTransport struct {
	calls  []string
	script func(url string) (*http.Response, error)
}

func (tr *docsTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tr.calls = append(tr.calls, req.URL.String())
	if tr.script == nil {
		return nil, fmt.Errorf("docsTestTransport: no scripted response for %s", req.URL)
	}
	return tr.script(req.URL.String())
}

// docsTestResp builds a minimal canned response for the scripted transport.
func docsTestResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

// setupDocsTest isolates one getRemoteDoc test: the cache dir is redirected
// into a temp dir, the clock is frozen at a fixed instant (advanced through
// the returned pointer) and the HTTP client is swapped for the scripted
// transport. Every override is restored after the test.
func setupDocsTest(t *testing.T, script func(url string) (*http.Response, error)) (*docsTestTransport, *time.Time) {
	t.Helper()
	origDir, origNow, origClient := docsCacheDir, docsNow, docsHTTPClient
	docsCacheDir = filepath.Join(t.TempDir(), "docscache")
	now := time.Date(2025, 8, 1, 12, 0, 0, 0, time.UTC)
	docsNow = func() time.Time { return now }
	tr := &docsTestTransport{script: script}
	docsHTTPClient = &http.Client{Transport: tr}
	t.Cleanup(func() {
		docsCacheDir, docsNow, docsHTTPClient = origDir, origNow, origClient
	})
	return tr, &now
}

// docsTestURLs returns the raw and jsDelivr URLs getRemoteDoc builds for a
// lang/section pair (derived from the production constants, no drift).
func docsTestURLs(lang, sectionID string) (rawURL, jsdURL string) {
	path := fmt.Sprintf(docsRepoPathFmt, lang, sectionID)
	return docsRawBaseURL + path, docsJSDBaseURL + path
}

// seedDocsCache writes a content file plus a meta entry directly, simulating
// a previous successful fetch at the given time.
func seedDocsCache(t *testing.T, key, content string, fetchedAt time.Time) {
	t.Helper()
	if err := os.MkdirAll(docsCacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsCacheDir, key+".md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	meta := readDocsMeta()
	meta.Entries[key] = docsCacheEntry{FetchedAt: fetchedAt.Format(time.RFC3339)}
	writeDocsMeta(meta)
}

// readDocsMetaFromDisk loads meta.json raw for persistence assertions.
func readDocsMetaFromDisk(t *testing.T) docsCacheMeta {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(docsCacheDir, docsMetaName))
	if err != nil {
		t.Fatalf("meta.json should exist after the call: %v", err)
	}
	var meta docsCacheMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("meta.json should parse: %v", err)
	}
	return meta
}

// ─── Validation ──────────────────────────────────────────────────

// TestRemoteDocInvalidArgs verifies errors are returned ONLY for invalid
// arguments (unknown lang, bad section id — including traversal shapes) and
// that no network attempt is made for them.
func TestRemoteDocInvalidArgs(t *testing.T) {
	tr, _ := setupDocsTest(t, nil)

	cases := []struct{ lang, id string }{
		{"fr", "faq"}, // unknown lang
		{"ZH", "faq"}, // lang is case-sensitive
		{"zh", "FAQ"}, // section id is case-sensitive
		{"zh", "ab"},  // below the 3-char minimum
		{"zh", "a very long section id over the cap"},
		{"zh", "../evil"}, // traversal shape
		{"zh", "faq.md"},  // extension smuggle
		{"zh", ""},        // empty
	}
	for _, c := range cases {
		if _, err := getRemoteDoc(c.lang, c.id, false); err == nil {
			t.Errorf("getRemoteDoc(%q, %q) should reject invalid arguments", c.lang, c.id)
		}
	}
	if len(tr.calls) != 0 {
		t.Errorf("invalid args must not touch the network, got %d calls", len(tr.calls))
	}
}

// ─── Cache short-circuits ────────────────────────────────────────

// TestRemoteDocFreshCacheShortCircuit verifies a cached fetch inside the TTL
// is served with zero transport calls.
func TestRemoteDocFreshCacheShortCircuit(t *testing.T) {
	tr, now := setupDocsTest(t, nil) // any call would fail loudly via the script
	seedDocsCache(t, "zh-faq", "# cached body", *now)

	res, err := getRemoteDoc("zh", "faq", false)
	if err != nil {
		t.Fatalf("fresh cache should be a nil-error hit: %v", err)
	}
	if res.Source != docsSourceCache || res.Text != "# cached body" {
		t.Errorf("fresh cache hit = %+v, want source cache + cached text", res)
	}
	if res.FetchedAt != now.Format(time.RFC3339) {
		t.Errorf("fetchedAt = %q, want the seed time %q", res.FetchedAt, now.Format(time.RFC3339))
	}
	if len(tr.calls) != 0 {
		t.Errorf("fresh cache must short-circuit with zero transport calls, got %d", len(tr.calls))
	}
}

// TestRemoteDocForceBypassesFreshCache verifies force=true re-fetches even
// when a cache entry is still inside the TTL.
func TestRemoteDocForceBypassesFreshCache(t *testing.T) {
	rawURL, _ := docsTestURLs("zh", "faq")
	tr, now := setupDocsTest(t, func(url string) (*http.Response, error) {
		if url != rawURL {
			return nil, fmt.Errorf("unexpected url %s", url)
		}
		return docsTestResp(http.StatusOK, "# fresh from network"), nil
	})
	seedDocsCache(t, "zh-faq", "# cached body", *now)

	res, err := getRemoteDoc("zh", "faq", true)
	if err != nil {
		t.Fatalf("force fetch failed: %v", err)
	}
	if res.Source != docsSourceOnline || res.Text != "# fresh from network" {
		t.Errorf("force should fetch online, got %+v", res)
	}
	if len(tr.calls) != 1 {
		t.Errorf("force should issue exactly one call, got %d", len(tr.calls))
	}
}

// ─── Network success paths ───────────────────────────────────────

// TestRemoteDocTTLExpiryFetchesOnline verifies a stale cache triggers a raw
// fetch whose result is persisted to the content file and meta.
func TestRemoteDocTTLExpiryFetchesOnline(t *testing.T) {
	rawURL, _ := docsTestURLs("en", "quickstart")
	tr, now := setupDocsTest(t, func(url string) (*http.Response, error) {
		if url != rawURL {
			return nil, fmt.Errorf("unexpected url %s", url)
		}
		return docsTestResp(http.StatusOK, "# updated en docs"), nil
	})
	stale := now.Add(-25 * time.Hour) // beyond docsRemoteTTL
	seedDocsCache(t, "en-quickstart", "# old en docs", stale)

	res, err := getRemoteDoc("en", "quickstart", false)
	if err != nil {
		t.Fatalf("TTL-expired fetch failed: %v", err)
	}
	if res.Source != docsSourceOnline || res.Text != "# updated en docs" {
		t.Errorf("expired cache should refetch online, got %+v", res)
	}
	if res.FetchedAt != now.Format(time.RFC3339) {
		t.Errorf("fetchedAt = %q, want %q", res.FetchedAt, now.Format(time.RFC3339))
	}
	if len(tr.calls) != 1 {
		t.Errorf("raw success should stop after one call, got %d", len(tr.calls))
	}

	// Content file and meta must be persisted for the next cache hit.
	data, err := os.ReadFile(filepath.Join(docsCacheDir, "en-quickstart.md"))
	if err != nil || string(data) != "# updated en docs" {
		t.Errorf("content file not persisted: %q, %v", data, err)
	}
	meta := readDocsMetaFromDisk(t)
	if got := meta.Entries["en-quickstart"].FetchedAt; got != now.Format(time.RFC3339) {
		t.Errorf("meta entry fetchedAt = %q, want %q", got, now.Format(time.RFC3339))
	}
	if meta.LastNetworkFailAt != "" {
		t.Errorf("success must clear lastNetworkFailAt, got %q", meta.LastNetworkFailAt)
	}
}

// TestRemoteDocRawFallbackToJsDelivr verifies a raw failure (404) falls through
// to the jsDelivr mirror and the first success wins.
func TestRemoteDocRawFallbackToJsDelivr(t *testing.T) {
	rawURL, jsdURL := docsTestURLs("zh", "api")
	tr, now := setupDocsTest(t, func(url string) (*http.Response, error) {
		switch url {
		case rawURL:
			return docsTestResp(http.StatusNotFound, "404"), nil
		case jsdURL:
			return docsTestResp(http.StatusOK, "# cdn content"), nil
		}
		return nil, fmt.Errorf("unexpected url %s", url)
	})

	res, err := getRemoteDoc("zh", "api", false)
	if err != nil {
		t.Fatalf("fallback fetch failed: %v", err)
	}
	if res.Source != docsSourceOnline || res.Text != "# cdn content" {
		t.Errorf("jsDelivr fallback should win, got %+v", res)
	}
	if len(tr.calls) != 2 || tr.calls[0] != rawURL || tr.calls[1] != jsdURL {
		t.Errorf("calls = %v, want [raw, jsDelivr]", tr.calls)
	}
	if res.FetchedAt != now.Format(time.RFC3339) {
		t.Errorf("fetchedAt = %q, want %q", res.FetchedAt, now.Format(time.RFC3339))
	}
}

// ─── Failure paths ───────────────────────────────────────────────

// TestRemoteDocAllSourcesFailWithCache verifies a total network failure with a
// (stale) cache present degrades to the cache and records lastNetworkFailAt.
func TestRemoteDocAllSourcesFailWithCache(t *testing.T) {
	tr, now := setupDocsTest(t, nil) // nil script = every attempt fails
	stale := now.Add(-2 * docsRemoteTTL)
	seedDocsCache(t, "zh-chat", "# cached chat docs", stale)

	res, err := getRemoteDoc("zh", "chat", false)
	if err != nil {
		t.Fatalf("network failure must not error: %v", err)
	}
	if res.Source != docsSourceCache || res.Text != "# cached chat docs" {
		t.Errorf("total failure with cache should serve the cache, got %+v", res)
	}
	if res.FetchedAt != stale.Format(time.RFC3339) {
		t.Errorf("cache fallback keeps the original fetchedAt: got %q, want %q", res.FetchedAt, stale.Format(time.RFC3339))
	}
	if len(tr.calls) != 2 {
		t.Errorf("both mirrors should be attempted, got %d calls", len(tr.calls))
	}
	meta := readDocsMetaFromDisk(t)
	if meta.LastNetworkFailAt != now.Format(time.RFC3339) {
		t.Errorf("lastNetworkFailAt = %q, want %q", meta.LastNetworkFailAt, now.Format(time.RFC3339))
	}
}

// TestRemoteDocBackoffSkipsNetwork verifies that within docsFailBackoff after
// a total failure the cache is served with zero transport calls.
func TestRemoteDocBackoffSkipsNetwork(t *testing.T) {
	tr, now := setupDocsTest(t, nil)
	stale := now.Add(-2 * docsRemoteTTL)
	seedDocsCache(t, "zh-chat", "# cached chat docs", stale)

	// First call fails and records the failure time.
	if _, err := getRemoteDoc("zh", "chat", false); err != nil {
		t.Fatalf("first call must not error: %v", err)
	}
	if len(tr.calls) != 2 {
		t.Fatalf("first call should attempt both mirrors, got %d", len(tr.calls))
	}

	// Advance 5 minutes (< docsFailBackoff) and call again: no network.
	*now = now.Add(5 * time.Minute)
	tr.calls = nil
	res, err := getRemoteDoc("zh", "chat", false)
	if err != nil {
		t.Fatalf("backoff call must not error: %v", err)
	}
	if res.Source != docsSourceCache || res.Text != "# cached chat docs" {
		t.Errorf("backoff should serve the cache, got %+v", res)
	}
	if len(tr.calls) != 0 {
		t.Errorf("within the backoff no transport call may happen, got %d", len(tr.calls))
	}
}

// TestRemoteDocBackoffSkipsNetworkNoCache verifies the backoff mutes the
// network even when no cache exists: the second call within the window makes
// zero transport calls and reports "none" immediately (nil error).
func TestRemoteDocBackoffSkipsNetworkNoCache(t *testing.T) {
	tr, now := setupDocsTest(t, nil) // nil script = every attempt fails

	// First call attempts both mirrors and records the failure time.
	res, err := getRemoteDoc("zh", "faq", false)
	if err != nil {
		t.Fatalf("first call must not error: %v", err)
	}
	if res.Source != docsSourceNone || res.Text != "" {
		t.Errorf("no-cache failure = %+v, want source none with empty text", res)
	}
	if len(tr.calls) != 2 {
		t.Fatalf("first call should attempt both mirrors, got %d", len(tr.calls))
	}

	// Advance 5 minutes (< docsFailBackoff): the second call skips the network
	// entirely even though there is no cache to serve.
	*now = now.Add(5 * time.Minute)
	tr.calls = nil
	res, err = getRemoteDoc("zh", "faq", false)
	if err != nil {
		t.Fatalf("backoff call must not error: %v", err)
	}
	if res.Source != docsSourceNone || res.Text != "" || res.FetchedAt != "" {
		t.Errorf("backoff without cache = %+v, want none source / empty text / empty fetchedAt", res)
	}
	if len(tr.calls) != 0 {
		t.Errorf("within the backoff no transport call may happen without a cache either, got %d", len(tr.calls))
	}
}

// TestRemoteDocAllFailNoCache verifies a total failure with no cache yields
// source "none" with an empty body and a nil error (expected offline state).
func TestRemoteDocAllFailNoCache(t *testing.T) {
	tr, _ := setupDocsTest(t, nil)

	res, err := getRemoteDoc("en", "faq", false)
	if err != nil {
		t.Fatalf("network unavailability is an expected state, not an error: %v", err)
	}
	if res.Source != docsSourceNone || res.Text != "" || res.FetchedAt != "" {
		t.Errorf("no-cache failure = %+v, want empty text / none source / empty fetchedAt", res)
	}
	if len(tr.calls) != 2 {
		t.Errorf("both mirrors should be attempted, got %d calls", len(tr.calls))
	}
	if meta := readDocsMetaFromDisk(t); meta.LastNetworkFailAt == "" {
		t.Error("failure time should be persisted even without a cache")
	}
}

// TestRemoteDocOversizedBodyRejected verifies a body reaching docsMaxBytes is
// treated as an attempt failure (the next mirror is tried instead).
func TestRemoteDocOversizedBodyRejected(t *testing.T) {
	rawURL, jsdURL := docsTestURLs("zh", "settings")
	origMax := docsMaxBytes
	docsMaxBytes = 64
	t.Cleanup(func() { docsMaxBytes = origMax })
	tr, _ := setupDocsTest(t, func(url string) (*http.Response, error) {
		switch url {
		case rawURL:
			return docsTestResp(http.StatusOK, strings.Repeat("x", 100)), nil // exceeds the cap
		case jsdURL:
			return docsTestResp(http.StatusOK, "# cdn content"), nil
		}
		return nil, fmt.Errorf("unexpected url %s", url)
	})

	res, err := getRemoteDoc("zh", "settings", false)
	if err != nil {
		t.Fatalf("oversized raw body must degrade, not error: %v", err)
	}
	if res.Source != docsSourceOnline || res.Text != "# cdn content" {
		t.Errorf("oversized attempt should fall through to jsDelivr, got %+v", res)
	}
	if len(tr.calls) != 2 {
		t.Errorf("oversized raw body should trigger the fallback call, got %d calls", len(tr.calls))
	}
	// The persisted content must hold the jsDelivr body, never a truncated one.
	data, readErr := os.ReadFile(filepath.Join(docsCacheDir, "zh-settings.md"))
	if readErr != nil || string(data) != "# cdn content" {
		t.Errorf("persisted content = %q (%v), want the jsDelivr body", data, readErr)
	}
}

// ─── Cache resilience ────────────────────────────────────────────

// TestRemoteDocCorruptMetaTolerated verifies a corrupt meta.json degrades to
// an empty cache ([WARN]) without failing the call: the content file is still
// served through the failure fallback.
func TestRemoteDocCorruptMetaTolerated(t *testing.T) {
	tr, _ := setupDocsTest(t, nil) // all network attempts fail
	if err := os.MkdirAll(docsCacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsCacheDir, "zh-faq.md"), []byte("# cached body"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsCacheDir, docsMetaName), []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := getRemoteDoc("zh", "faq", false)
	if err != nil {
		t.Fatalf("corrupt meta must never fail the call: %v", err)
	}
	if res.Source != docsSourceCache || res.Text != "# cached body" {
		t.Errorf("content file should still be served, got %+v", res)
	}
	if len(tr.calls) != 2 {
		t.Errorf("corrupt meta loses freshness info, so both mirrors should be attempted, got %d", len(tr.calls))
	}
}
