package core

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// hfModelsPayload is a mock response for searchHFMirrorAt: two models, one with a GGUF
// sibling and pipeline_tag sentence-similarity (embedding filter hit), the other without GGUF.
const hfModelsPayload = `[
  {
    "id": "Xorbits/bge-small-zh-v1.5",
    "modelId": "Xorbits/bge-small-zh-v1.5",
    "author": "Xorbits",
    "downloads": 1000,
    "likes": 10,
    "pipeline_tag": "sentence-similarity",
    "tags": ["sentence-transformers", "text-embeddings"],
    "siblings": [
      {"rfilename": "bge-small-zh-v1.5-q8_0.gguf", "size": 1024},
      {"rfilename": "config.json", "size": 500}
    ]
  },
  {
    "id": "org/no-gguf",
    "modelId": "org/no-gguf",
    "author": "org",
    "downloads": 5,
    "likes": 0,
    "pipeline_tag": "text-generation",
    "tags": [],
    "siblings": [{"rfilename": "model.safetensors", "size": 999}]
  }
]`

// newHFServer starts a local server simulating the HF API; baseURL can be injected into
// *At functions.
func newHFServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/models":
			w.Write([]byte(hfModelsPayload))
		case "/api/models/Xorbits/bge-small-zh-v1.5":
			w.Write([]byte(`{"siblings":[
				{"rfilename":"bge-small-zh-v1.5-q8_0.gguf","size":1024},
				{"rfilename":"/model-f16.gguf","size":2048},
				{"rfilename":".hidden.gguf","size":1},
				{"rfilename":"README.md","size":10}
			]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestSearchHFMirrorAt verifies HF search: models without GGUF are filtered out
// (hasGGUF filter), and the filter parameter no longer filters by pipeline_tag type.
func TestSearchHFMirrorAt(t *testing.T) {
	srv := newHFServer(t)
	defer srv.Close()

	results, err := searchHFMirrorAt(srv.URL, "bge", "embedding")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("search returned %d results, want 1", len(results))
	}
	if results[0].ID != "Xorbits/bge-small-zh-v1.5" {
		t.Errorf("ID = %q", results[0].ID)
	}
	if len(results[0].Siblings) != 2 {
		t.Errorf("Siblings = %d, want 2", len(results[0].Siblings))
	}
}

// TestSearchHFMirrorAtAllFilter verifies filter=all does not apply type filtering.
func TestSearchHFMirrorAtAllFilter(t *testing.T) {
	srv := newHFServer(t)
	defer srv.Close()

	results, err := searchHFMirrorAt(srv.URL, "bge", "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("filter=all result count = %d, want 1", len(results))
	}
}

// TestSearchHFMirrorAtHTTPError verifies a non-200 response returns an error.
func TestSearchHFMirrorAtHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	if _, err := searchHFMirrorAt(srv.URL, "q", "all"); err == nil {
		t.Error("503 response should return error")
	}
}

// TestSearchHFMirrorAtFilterIgnored verifies that when filter="llm", pipeline_tag type
// filtering is no longer applied: both sentence-similarity (has GGUF) and
// text-generation (no GGUF) are retained within the filter range, but models without
// GGUF are still excluded by hasGGUF filtering, so only one model with GGUF is returned.
// This verifies the type-filter branch has been removed while hasGGUF filtering is retained.
func TestSearchHFMirrorAtFilterIgnored(t *testing.T) {
	srv := newHFServer(t)
	defer srv.Close()

	results, err := searchHFMirrorAt(srv.URL, "bge", "llm")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("filter=llm result count = %d, want 1 (type filtering should be ignored, only hasGGUF filtering active)", len(results))
	}
	if results[0].ID != "Xorbits/bge-small-zh-v1.5" {
		t.Errorf("ID = %q, want %q (text-generation model without GGUF should be filtered)",
			results[0].ID, "Xorbits/bge-small-zh-v1.5")
	}
}

// hfModelJSON constructs an HF /api/models list-item JSON; id is modelId; whether siblings
// contain gguf or safetensors files is determined by hasGGUF.
func hfModelJSON(id string, hasGGUF bool) string {
	file := `{"rfilename":"model.safetensors","size":999}`
	if hasGGUF {
		file = `{"rfilename":"model-q8_0.gguf","size":1024}`
	}
	return fmt.Sprintf(`{"id":%q,"modelId":%q,"author":"org","downloads":1,"likes":1,"pipeline_tag":"text-generation","tags":[],"siblings":[%s]}`,
		id, id, file)
}

// TestSearchHFMirrorAtMultiSortMerge verifies three-way sort (downloads / likes /
// lastModified) parallel fetch then merge/dedup in downloads → likes → lastModified order:
// different sort mocks return different sets; C without GGUF from downloads and E without
// GGUF from lastModified are filtered; B and D appearing in both likes and lastModified
// are kept only from the downloads path or first appearance. Assertion: merged = [A,B,D].
func TestSearchHFMirrorAtMultiSortMerge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("sort") {
		case "downloads":
			w.Write([]byte(fmt.Sprintf("[%s,%s,%s]",
				hfModelJSON("A", true), hfModelJSON("B", true), hfModelJSON("C", false))))
		case "likes":
			w.Write([]byte(fmt.Sprintf("[%s,%s]",
				hfModelJSON("B", true), hfModelJSON("D", true))))
		case "lastModified":
			w.Write([]byte(fmt.Sprintf("[%s,%s]",
				hfModelJSON("D", true), hfModelJSON("E", false))))
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	results, err := searchHFMirrorAt(srv.URL, "q", "all")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, r := range results {
		ids = append(ids, r.ModelID)
	}
	want := []string{"A", "B", "D"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Errorf("merged result = %v, want %v (downloads primary order, dedup by modelId, no-GGUF excluded)", ids, want)
	}
}

// TestSearchHFMirrorAtPartialSortFailure verifies that when a single sort path fails,
// the overall operation does not error: the likes path returning 500 is skipped, normal
// results from downloads / lastModified paths are retained and deduped by modelId,
// remaining [A].
func TestSearchHFMirrorAtPartialSortFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("sort") == "likes" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte(fmt.Sprintf("[%s]", hfModelJSON("A", true))))
	}))
	defer srv.Close()

	results, err := searchHFMirrorAt(srv.URL, "q", "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ModelID != "A" {
		t.Errorf("single-path failure should retain normal-path results, got %+v", results)
	}
}

// newReadmeServer starts a local server simulating README, used by getModelDescriptionAt
// tests to inject baseURL. Return values are distinguished by path: normal README,
// overlong paragraph README, no-description-paragraph README, and non-existent path (404).
func newReadmeServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/author/model/raw/main/README.md":
			w.Write([]byte("---\nlicense: apache-2.0\n---\n\n# 标题\n\n第一段自然语言描述，用于验证 front-matter 跳过与段落提取。\n\n## 子标题\n\n第二段属于子标题之下，不应被返回。\n"))
		case "/author/longmodel/raw/main/README.md":
			// construct a paragraph exceeding 200 runes (70×3=210 runes), verify truncation and ellipsis
			para := strings.Repeat("长描述", 70) // 210 runes
			w.Write([]byte("---\ntags: test\n---\n\n# 标题\n\n" + para + "\n"))
		case "/author/nodesc/raw/main/README.md":
			// README exists but has no usable description paragraph (all headings)
			w.Write([]byte("---\nlicense: mit\n---\n\n# 只有标题\n\n## 另一个标题\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestGetModelDescriptionAt verifies README description extraction: skips YAML
// front-matter and heading lines, returns the first natural-language paragraph.
func TestGetModelDescriptionAt(t *testing.T) {
	srv := newReadmeServer(t)
	defer srv.Close()

	desc, err := getModelDescriptionAt(srv.URL, "author/model")
	if err != nil {
		t.Fatal(err)
	}
	if desc != "第一段自然语言描述，用于验证 front-matter 跳过与段落提取。" {
		t.Errorf("desc = %q, must not contain front-matter or heading lines", desc)
	}
	if strings.Contains(desc, "---") || strings.Contains(desc, "license") {
		t.Errorf("desc must not contain front-matter content: %q", desc)
	}
	if strings.Contains(desc, "#") {
		t.Errorf("desc must not contain heading lines: %q", desc)
	}
}

// TestGetModelDescriptionAtTruncate verifies overlong paragraphs are truncated at 200
// runes with an ellipsis appended.
func TestGetModelDescriptionAtTruncate(t *testing.T) {
	srv := newReadmeServer(t)
	defer srv.Close()

	desc, err := getModelDescriptionAt(srv.URL, "author/longmodel")
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

// TestGetModelDescriptionAtNotFound verifies a 404 README response returns an error.
func TestGetModelDescriptionAtNotFound(t *testing.T) {
	srv := newReadmeServer(t)
	defer srv.Close()

	if _, err := getModelDescriptionAt(srv.URL, "author/missing"); err == nil {
		t.Error("404 response should return error")
	} else if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("error message should contain HTTP status code: %v", err)
	}
}

// TestGetModelDescriptionAtNoDescription verifies that when a README exists but has no
// description paragraph, an empty string and nil error are returned (silent handling,
// not treated as failure).
func TestGetModelDescriptionAtNoDescription(t *testing.T) {
	srv := newReadmeServer(t)
	defer srv.Close()

	desc, err := getModelDescriptionAt(srv.URL, "author/nodesc")
	if err != nil {
		t.Fatal(err)
	}
	if desc != "" {
		t.Errorf("no description paragraph should return empty string, got %q", desc)
	}
}

// TestGetHFModelFilesAt verifies the model file list: only top-level .gguf files are
// returned, excess leading slashes are stripped, and directories and hidden files are ignored.
func TestGetHFModelFilesAt(t *testing.T) {
	srv := newHFServer(t)
	defer srv.Close()

	files, err := getHFModelFilesAt(srv.URL, "Xorbits/bge-small-zh-v1.5")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("file count = %d, want 2 (should exclude hidden files and README)", len(files))
	}
	for _, f := range files {
		if f.Filename == ".hidden.gguf" || f.Filename == "README.md" {
			t.Errorf("must not contain %q", f.Filename)
		}
	}
	// "/model-f16.gguf" should be stripped to "model-f16.gguf"
	hasTrimmed := false
	for _, f := range files {
		if f.Filename == "model-f16.gguf" {
			hasTrimmed = true
		}
	}
	if !hasTrimmed {
		t.Errorf("stripped model-f16.gguf not found: %+v", files)
	}
}

// TestGetHFModelMaxGGUFSizeAt verifies model maximum GGUF size aggregation: search card
// size requires the details endpoint (blobs=true for real size), takes the largest .gguf
// file (excluding hidden files and non-gguf). mock siblings: bge q8_0(1024),
// /model-f16.gguf(2048), .hidden.gguf(1), README.md(10) → maximum should be 2048.
func TestGetHFModelMaxGGUFSizeAt(t *testing.T) {
	srv := newHFServer(t)
	defer srv.Close()

	size, err := getHFModelMaxGGUFSizeAt(srv.URL, "Xorbits/bge-small-zh-v1.5")
	if err != nil {
		t.Fatal(err)
	}
	if size != 2048 {
		t.Errorf("max GGUF size = %d, want 2048 (should exclude hidden files and README, take model-f16.gguf)", size)
	}
}

// TestGetHFModelMaxGGUFSizeAtNoGGUF verifies that when a model has no GGUF files,
// 0 and nil are returned (not treated as error; frontend silently does not display size).
func TestGetHFModelMaxGGUFSizeAtNoGGUF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"siblings":[
			{"rfilename":"model.safetensors","size":999},
			{"rfilename":"README.md","size":10}
		]}`))
	}))
	defer srv.Close()

	size, err := getHFModelMaxGGUFSizeAt(srv.URL, "org/no-gguf")
	if err != nil {
		t.Fatal(err)
	}
	if size != 0 {
		t.Errorf("no GGUF size = %d, want 0", size)
	}
}

// TestGetHFModelMaxGGUFSizeAtHTTPError verifies a non-200 details endpoint response
// returns an error.
func TestGetHFModelMaxGGUFSizeAtHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := getHFModelMaxGGUFSizeAt(srv.URL, "org/missing"); err == nil {
		t.Error("404 response should return error")
	}
}
