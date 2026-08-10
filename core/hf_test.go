package core

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// hfModelsPayload 是 searchHFMirrorAt 的模拟响应：两个模型，一个有 GGUF
// sibling 且为 sentence-similarity（embedding 过滤命中），另一个无 GGUF。
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

// newHFServer 起一个模拟 HF API 的本地服务，baseURL 可注入到 *At 函数。
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

// TestSearchHFMirrorAt 验证 HF 搜索：过滤掉无 GGUF 的模型（hasGGUF 过滤），
// filter 参数已不再按 pipeline_tag 类型过滤。
func TestSearchHFMirrorAt(t *testing.T) {
	srv := newHFServer(t)
	defer srv.Close()

	results, err := searchHFMirrorAt(srv.URL, "bge", "embedding")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("搜索到 %d 条结果, want 1", len(results))
	}
	if results[0].ID != "Xorbits/bge-small-zh-v1.5" {
		t.Errorf("ID = %q", results[0].ID)
	}
	if len(results[0].Siblings) != 2 {
		t.Errorf("Siblings = %d, want 2", len(results[0].Siblings))
	}
}

// TestSearchHFMirrorAtAllFilter 验证 filter=all 不应用类型过滤。
func TestSearchHFMirrorAtAllFilter(t *testing.T) {
	srv := newHFServer(t)
	defer srv.Close()

	results, err := searchHFMirrorAt(srv.URL, "bge", "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("filter=all 结果数 = %d, want 1", len(results))
	}
}

// TestSearchHFMirrorAtHTTPError 验证非 200 响应返回错误。
func TestSearchHFMirrorAtHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	if _, err := searchHFMirrorAt(srv.URL, "q", "all"); err == nil {
		t.Error("503 响应应返回错误")
	}
}

// TestSearchHFMirrorAtFilterIgnored 验证 filter="llm" 时不再按 pipeline_tag
// 类型过滤：sentence-similarity（有 GGUF）与 text-generation（无 GGUF）都保留
// 在过滤范围内，但无 GGUF 的模型仍被 hasGGUF 过滤排除，故仍只返回一个有 GGUF
// 的结果。这验证了类型过滤分支已移除、hasGGUF 过滤保留。
func TestSearchHFMirrorAtFilterIgnored(t *testing.T) {
	srv := newHFServer(t)
	defer srv.Close()

	results, err := searchHFMirrorAt(srv.URL, "bge", "llm")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("filter=llm 结果数 = %d, want 1（类型过滤应被忽略，仅 hasGGUF 过滤生效）", len(results))
	}
	if results[0].ID != "Xorbits/bge-small-zh-v1.5" {
		t.Errorf("ID = %q, want %q（无 GGUF 的 text-generation 模型应被过滤）",
			results[0].ID, "Xorbits/bge-small-zh-v1.5")
	}
}

// newReadmeServer 起一个模拟 README 的本地服务，供 getModelDescriptionAt 测试
// 注入 baseURL。返回值按 path 区分：正常 README、超长段落 README、无描述段落
// README、以及不存在的路径（404）。
func newReadmeServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/author/model/raw/main/README.md":
			w.Write([]byte("---\nlicense: apache-2.0\n---\n\n# 标题\n\n第一段自然语言描述，用于验证 front-matter 跳过与段落提取。\n\n## 子标题\n\n第二段属于子标题之下，不应被返回。\n"))
		case "/author/longmodel/raw/main/README.md":
			// 构造一段超过 200 个 rune 的段落（70×3=210 rune），验证截断与省略号
			para := strings.Repeat("长描述", 70) // 210 个 rune
			w.Write([]byte("---\ntags: test\n---\n\n# 标题\n\n" + para + "\n"))
		case "/author/nodesc/raw/main/README.md":
			// README 存在但没有可用的描述段落（全为标题）
			w.Write([]byte("---\nlicense: mit\n---\n\n# 只有标题\n\n## 另一个标题\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestGetModelDescriptionAt 验证 README 描述提取：跳过 YAML front-matter 与
// 标题行，返回首个自然语言段落。
func TestGetModelDescriptionAt(t *testing.T) {
	srv := newReadmeServer(t)
	defer srv.Close()

	desc, err := getModelDescriptionAt(srv.URL, "author/model")
	if err != nil {
		t.Fatal(err)
	}
	if desc != "第一段自然语言描述，用于验证 front-matter 跳过与段落提取。" {
		t.Errorf("描述 = %q，不应包含 front-matter 或标题行", desc)
	}
	if strings.Contains(desc, "---") || strings.Contains(desc, "license") {
		t.Errorf("描述不应包含 front-matter 内容: %q", desc)
	}
	if strings.Contains(desc, "#") {
		t.Errorf("描述不应包含标题行: %q", desc)
	}
}

// TestGetModelDescriptionAtTruncate 验证超长段落按 200 个 rune 截断并追加省略号。
func TestGetModelDescriptionAtTruncate(t *testing.T) {
	srv := newReadmeServer(t)
	defer srv.Close()

	desc, err := getModelDescriptionAt(srv.URL, "author/longmodel")
	if err != nil {
		t.Fatal(err)
	}
	runes := []rune(desc)
	if len(runes) != 203 { // 200 rune + "..."
		t.Fatalf("截断后长度 = %d rune, want 203", len(runes))
	}
	if !strings.HasSuffix(desc, "...") {
		t.Errorf("截断后的描述应以 ... 结尾: %q", desc)
	}
}

// TestGetModelDescriptionAtNotFound 验证 README 返回 404 时返回错误。
func TestGetModelDescriptionAtNotFound(t *testing.T) {
	srv := newReadmeServer(t)
	defer srv.Close()

	if _, err := getModelDescriptionAt(srv.URL, "author/missing"); err == nil {
		t.Error("404 响应应返回错误")
	} else if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("错误信息应包含 HTTP 状态码: %v", err)
	}
}

// TestGetModelDescriptionAtNoDescription 验证 README 存在但无描述段落时
// 返回空串与 nil 错误（静默处理，不视为失败）。
func TestGetModelDescriptionAtNoDescription(t *testing.T) {
	srv := newReadmeServer(t)
	defer srv.Close()

	desc, err := getModelDescriptionAt(srv.URL, "author/nodesc")
	if err != nil {
		t.Fatal(err)
	}
	if desc != "" {
		t.Errorf("无描述段落时应返回空串, got %q", desc)
	}
}

// TestGetHFModelFilesAt 验证模型文件列表：只返回顶层 .gguf 文件，
// 去掉多余前导斜杠，忽略目录与隐藏文件。
func TestGetHFModelFilesAt(t *testing.T) {
	srv := newHFServer(t)
	defer srv.Close()

	files, err := getHFModelFilesAt(srv.URL, "Xorbits/bge-small-zh-v1.5")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("文件数 = %d, want 2（应排除隐藏文件与 README）", len(files))
	}
	for _, f := range files {
		if f.Filename == ".hidden.gguf" || f.Filename == "README.md" {
			t.Errorf("不应包含 %q", f.Filename)
		}
	}
	// "/model-f16.gguf" 应被去斜杠为 "model-f16.gguf"
	hasTrimmed := false
	for _, f := range files {
		if f.Filename == "model-f16.gguf" {
			hasTrimmed = true
		}
	}
	if !hasTrimmed {
		t.Errorf("未找到去斜杠后的 model-f16.gguf: %+v", files)
	}
}
