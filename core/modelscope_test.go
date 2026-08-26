package core

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// withModelScopeBases replaces both ModelScope package-level Base variables with local
// httptest server URLs during the test, restoring original values afterward.
// ModelScope clients inject Base via package-level vars (same style as HF *At parameter
// injection, see modelscope.go comments).
func withModelScopeBases(t *testing.T, openAPI, legacy string) {
	t.Helper()
	origOpenAPI := modelscopeOpenAPIBase
	origFallback := modelscopeOpenAPIFallback
	origLegacy := modelscopeLegacyBase
	modelscopeOpenAPIBase = openAPI
	modelscopeOpenAPIFallback = openAPI
	modelscopeLegacyBase = legacy
	t.Cleanup(func() {
		modelscopeOpenAPIBase = origOpenAPI
		modelscopeOpenAPIFallback = origFallback
		modelscopeLegacyBase = origLegacy
	})
}

// TestSearchModelScopeAt verifies ModelScope OpenAPI search:
//   - the OpenAPI response uses "id" (not "Path"); "Path" is still accepted as a fallback;
//   - author is taken from the first segment of id; id without "/" uses the whole string as author;
//   - downloads/likes in both numeric and numeric-string forms are leniently parsed;
//   - the first task populates PipelineTag;
//   - entries with empty id (and Path) are skipped;
//   - non-GGUF models (no "gguf" in id or tags) are filtered out so only
//     downloadable GGUF repositories are returned.
func TestSearchModelScopeAt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"success": true,
			"data": {
				"models": [
					{"id": "author/gguf-model-one", "downloads": 100, "likes": "10", "tags": ["llm"], "tasks": ["text-generation"]},
					{"id": "author/model-two", "downloads": "200", "likes": 20, "tags": ["library:gguf"]},
					{"id": "", "downloads": 1, "likes": 1},
					{"id": "gguf-noslash-model", "downloads": 1, "likes": 1},
					{"id": "author/plain-model", "downloads": 1, "likes": 1, "tags": ["llm"]},
					{"Path": "legacy/owner-gguf-model", "downloads": 7, "likes": 3}
				]
			}
		}`))
	}))
	defer srv.Close()
	withModelScopeBases(t, srv.URL, srv.URL)

	results, err := searchModelScope("q")
	if err != nil {
		t.Fatal(err)
	}
	// kept: gguf-model-one, model-two (tag library:gguf), gguf-noslash-model,
	// legacy/owner-gguf-model. dropped: empty id, author/plain-model (non-GGUF).
	if len(results) != 4 {
		t.Fatalf("result count = %d, want 4 (empty id and non-GGUF entries should be skipped)", len(results))
	}
	m1 := results[0]
	if m1.ModelID != "author/gguf-model-one" || m1.ID != "author/gguf-model-one" {
		t.Errorf("model-one ModelID/ID = %q/%q", m1.ModelID, m1.ID)
	}
	if m1.Author != "author" {
		t.Errorf("model-one author = %q, want author (first id segment)", m1.Author)
	}
	if m1.Downloads != 100 || m1.Likes != 10 {
		t.Errorf("model-one downloads/likes = %d/%d, want 100/10 (numeric and string forms must both parse)", m1.Downloads, m1.Likes)
	}
	if len(m1.Tags) != 1 || m1.Tags[0] != "llm" {
		t.Errorf("model-one tags = %v, want [llm]", m1.Tags)
	}
	if m1.PipelineTag != "text-generation" {
		t.Errorf("model-one PipelineTag = %q, want text-generation (from tasks[0])", m1.PipelineTag)
	}
	m2 := results[1]
	if m2.Downloads != 200 || m2.Likes != 20 {
		t.Errorf("model-two downloads/likes = %d/%d, want 200/20 (string downloads and numeric likes)", m2.Downloads, m2.Likes)
	}
	// id without "/": author is the whole string
	if results[2].Author != "gguf-noslash-model" {
		t.Errorf("gguf-noslash-model author = %q, want whole string gguf-noslash-model", results[2].Author)
	}
	// legacy "Path" fallback is still parsed
	if results[3].ModelID != "legacy/owner-gguf-model" || results[3].Author != "legacy" {
		t.Errorf("legacy Path fallback = %q/%q, want legacy/owner-gguf-model/legacy", results[3].ModelID, results[3].Author)
	}
}

// TestSearchModelScopeAtSuccessFalse verifies that when the response has success=false,
// an error is returned (ModelScope business-failure signal).
func TestSearchModelScopeAtSuccessFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success": false, "data": {"models": []}}`))
	}))
	defer srv.Close()
	withModelScopeBases(t, srv.URL, srv.URL)

	if _, err := searchModelScope("q"); err == nil || !strings.Contains(err.Error(), "success=false") {
		t.Errorf("success=false should return an error containing success=false, got %v", err)
	}
}

// TestSearchModelScopeAtHTTPError verifies a non-200 response returns an error.
func TestSearchModelScopeAtHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	withModelScopeBases(t, srv.URL, srv.URL)

	if _, err := searchModelScope("q"); err == nil {
		t.Error("503 response should return error")
	}
}

// TestListModelScopeFilesAt verifies ModelScope Legacy file list:
//   - only entries with Type=="blob" and lowercase .gguf suffix are retained;
//   - Size in both numeric and string forms is converted to int64;
//   - directories (tree), non-gguf blobs, and README are filtered out.
func TestListModelScopeFilesAt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"Code": 200,
			"Data": {
				"Files": [
					{"Path": "model-q4_k_m.gguf", "Size": 100, "Type": "blob"},
					{"Path": "model-f16.gguf", "Size": "200", "Type": "blob"},
					{"Path": "MODEL.UPPER.GGUF", "Size": 500, "Type": "blob"},
					{"Path": "model.bin", "Size": 300, "Type": "blob"},
					{"Path": "subdir/model.gguf", "Size": 400, "Type": "tree"},
					{"Path": "README.md", "Size": 50, "Type": "blob"}
				]
			}
		}`))
	}))
	defer srv.Close()
	withModelScopeBases(t, srv.URL, srv.URL)

	files, err := listModelScopeFiles("author/model")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("file count = %d, want 3 (tree / non-gguf blob / README should be filtered): %+v", len(files), files)
	}
	if files[0].Filename != "model-q4_k_m.gguf" || files[0].Size != 100 {
		t.Errorf("files[0] = %+v, want model-q4_k_m.gguf/100", files[0])
	}
	if files[1].Filename != "model-f16.gguf" || files[1].Size != 200 {
		t.Errorf("files[1] = %+v, want model-f16.gguf/200 (string Size should parse)", files[1])
	}
	if files[2].Filename != "MODEL.UPPER.GGUF" || files[2].Size != 500 {
		t.Errorf("files[2] = %+v, want MODEL.UPPER.GGUF/500 (case-insensitive .gguf match)", files[2])
	}
}

// TestListModelScopeFilesAtCodeError verifies that when response Code!=200, an error is returned.
func TestListModelScopeFilesAtCodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"Code": 500, "Data": {"Files": []}}`))
	}))
	defer srv.Close()
	withModelScopeBases(t, srv.URL, srv.URL)

	if _, err := listModelScopeFiles("author/model"); err == nil || !strings.Contains(err.Error(), "Code=500") {
		t.Errorf("Code=500 should return an error containing Code=500, got %v", err)
	}
}

// TestListModelScopeFilesAtHTTPError verifies a non-200 response returns an error.
func TestListModelScopeFilesAtHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	withModelScopeBases(t, srv.URL, srv.URL)

	if _, err := listModelScopeFiles("author/missing"); err == nil {
		t.Error("404 response should return error")
	}
}

// TestBuildModelScopeDownloadURL verifies download URL construction and PathEscape escaping:
// both modelID and fileName go through url.PathEscape ("/", spaces, and Chinese characters
// are all escaped), ensuring models with spaces/Chinese filenames can be downloaded correctly.
func TestBuildModelScopeDownloadURL(t *testing.T) {
	base := "https://modelscope.cn/api/v1/models"

	got := buildModelScopeDownloadURL(base, "author/model", "model q4.gguf")
	want := base + "/" + url.PathEscape("author/model") + "/repo?Revision=master&FilePath=" + url.PathEscape("model q4.gguf")
	if got != want {
		t.Errorf("URL = %q, want %q (modelID/fileName should be PathEscape-escaped)", got, want)
	}
	// explicitly assert escape results, preventing test distortion after PathEscape behavior changes
	if !strings.Contains(got, "/author%2Fmodel/repo?") {
		t.Errorf("/ in modelID should be escaped to %%2F: %q", got)
	}
	if !strings.Contains(got, "FilePath=model%20q4.gguf") {
		t.Errorf("space in filename should be escaped to %%20: %q", got)
	}

	// Chinese filename: UTF-8 byte-by-byte percent escaping
	gotCN := buildModelScopeDownloadURL(base, "author/model", "模型文件.gguf")
	wantCN := base + "/author%2Fmodel/repo?Revision=master&FilePath=%E6%A8%A1%E5%9E%8B%E6%96%87%E4%BB%B6.gguf"
	if gotCN != wantCN {
		t.Errorf("Chinese filename URL = %q, want %q", gotCN, wantCN)
	}
}

// newModelScopeDescServer starts a local server simulating ModelScope README: distinguishes
// normal description, overlong paragraph, no-description paragraph, and non-existent path.
// ModelScope description uses the repo endpoint
// ({legacyBase}/{PathEscape(modelID)}/repo?FilePath=README.md), modelID is embedded
// in the URL path and used as the routing criterion.
func newModelScopeDescServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/author/model/repo":
			w.Write([]byte("---\nlicense: apache-2.0\n---\n\n# 标题\n\n第一段自然语言描述，用于验证 ModelScope front-matter 跳过与段落提取。\n\n## 子标题\n\n不应被返回的第二段。\n"))
		case "/author/longmodel/repo":
			// construct a paragraph exceeding 200 runes (70×3=210 runes), verify truncation and ellipsis
			para := strings.Repeat("长描述", 70)
			w.Write([]byte("---\ntags: test\n---\n\n# 标题\n\n" + para + "\n"))
		case "/author/nodesc/repo":
			// README exists but has no usable description paragraph (all headings)
			w.Write([]byte("---\nlicense: mit\n---\n\n# 只有标题\n\n## 另一个标题\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestGetModelScopeDescriptionAt verifies ModelScope README description extraction: skips
// YAML front-matter and heading lines, returns the first natural-language paragraph
// (shares extractDescription with HF).
func TestGetModelScopeDescriptionAt(t *testing.T) {
	srv := newModelScopeDescServer(t)
	defer srv.Close()
	withModelScopeBases(t, srv.URL, srv.URL)

	desc, err := getModelScopeDescription("author/model")
	if err != nil {
		t.Fatal(err)
	}
	if desc != "第一段自然语言描述，用于验证 ModelScope front-matter 跳过与段落提取。" {
		t.Errorf("desc = %q, must not contain front-matter or heading lines", desc)
	}
	if strings.Contains(desc, "---") || strings.Contains(desc, "license") || strings.Contains(desc, "#") {
		t.Errorf("desc must not contain front-matter or heading content: %q", desc)
	}
}

// TestGetModelScopeDescriptionAtTruncate verifies overlong paragraphs are truncated at 200
// runes with an ellipsis appended.
func TestGetModelScopeDescriptionAtTruncate(t *testing.T) {
	srv := newModelScopeDescServer(t)
	defer srv.Close()
	withModelScopeBases(t, srv.URL, srv.URL)

	desc, err := getModelScopeDescriptionAt(srv.URL, "author/longmodel")
	if err != nil {
		t.Fatal(err)
	}
	runes := []rune(desc)
	if len(runes) != 203 { // 200 runes + "..."
		t.Fatalf("truncated length = %d runes, want 203", len(runes))
	}
	if !strings.HasSuffix(desc, "...") {
		t.Errorf("truncated description should end with ...: %q", desc)
	}
}

// TestGetModelScopeDescriptionAtNoDescription verifies that when a README exists but has no
// description paragraph, an empty string and nil error are returned (silent handling,
// consistent with HF).
func TestGetModelScopeDescriptionAtNoDescription(t *testing.T) {
	srv := newModelScopeDescServer(t)
	defer srv.Close()
	withModelScopeBases(t, srv.URL, srv.URL)

	desc, err := getModelScopeDescriptionAt(srv.URL, "author/nodesc")
	if err != nil {
		t.Fatal(err)
	}
	if desc != "" {
		t.Errorf("no description paragraph should return empty string, got %q", desc)
	}
}

// TestGetModelScopeDescriptionAtNotFound verifies a 404 README response returns an error.
func TestGetModelScopeDescriptionAtNotFound(t *testing.T) {
	srv := newModelScopeDescServer(t)
	defer srv.Close()
	withModelScopeBases(t, srv.URL, srv.URL)

	if _, err := getModelScopeDescriptionAt(srv.URL, "author/missing"); err == nil {
		t.Error("404 response should return error")
	} else if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("error message should contain HTTP status code: %v", err)
	}
}
