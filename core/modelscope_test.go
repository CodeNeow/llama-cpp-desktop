package core

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// withModelScopeBases 在测试期间把 ModelScope 两个包级 Base 替换为本地
// httptest 服务器地址，测试结束后恢复原值。ModelScope 客户端通过包级 var
// 注入 Base（与 HF 的 *At 参数注入同风格，见 modelscope.go 注释）。
func withModelScopeBases(t *testing.T, openAPI, legacy string) {
	t.Helper()
	origOpenAPI := modelscopeOpenAPIBase
	origLegacy := modelscopeLegacyBase
	modelscopeOpenAPIBase = openAPI
	modelscopeLegacyBase = legacy
	t.Cleanup(func() {
		modelscopeOpenAPIBase = origOpenAPI
		modelscopeLegacyBase = origLegacy
	})
}

// TestSearchModelScopeAt 验证 ModelScope OpenAPI 搜索：
//   - author 取 Path 第一段，无 "/" 的 Path 整串作为 author；
//   - downloads/likes 数字与数字字符串两种形态都被宽松解析；
//   - Path 为空的条目被跳过。
func TestSearchModelScopeAt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"success": true,
			"data": {
				"models": [
					{"Path": "author/model-one", "downloads": 100, "likes": "10", "tags": ["llm"]},
					{"Path": "author/model-two", "downloads": "200", "likes": 20, "tags": []},
					{"Path": "", "downloads": 1, "likes": 1},
					{"Path": "no-slash-model", "downloads": 1, "likes": 1}
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
	if len(results) != 3 {
		t.Fatalf("结果数 = %d, want 3（Path 为空条目应跳过）", len(results))
	}
	m1 := results[0]
	if m1.ModelID != "author/model-one" || m1.ID != "author/model-one" {
		t.Errorf("model-one ModelID/ID = %q/%q", m1.ModelID, m1.ID)
	}
	if m1.Author != "author" {
		t.Errorf("model-one author = %q, want author（Path 第一段）", m1.Author)
	}
	if m1.Downloads != 100 || m1.Likes != 10 {
		t.Errorf("model-one downloads/likes = %d/%d, want 100/10（数字与字符串形态应都解析）", m1.Downloads, m1.Likes)
	}
	if len(m1.Tags) != 1 || m1.Tags[0] != "llm" {
		t.Errorf("model-one tags = %v, want [llm]", m1.Tags)
	}
	m2 := results[1]
	if m2.Downloads != 200 || m2.Likes != 20 {
		t.Errorf("model-two downloads/likes = %d/%d, want 200/20（字符串 downloads 与数字 likes）", m2.Downloads, m2.Likes)
	}
	// 无 "/" 的 Path：author 整串
	if results[2].Author != "no-slash-model" {
		t.Errorf("no-slash-model author = %q, want 整串 no-slash-model", results[2].Author)
	}
}

// TestSearchModelScopeAtSuccessFalse 验证响应 success=false 时返回错误
// （ModelScope 业务失败信号）。
func TestSearchModelScopeAtSuccessFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success": false, "data": {"models": []}}`))
	}))
	defer srv.Close()
	withModelScopeBases(t, srv.URL, srv.URL)

	if _, err := searchModelScope("q"); err == nil || !strings.Contains(err.Error(), "success=false") {
		t.Errorf("success=false 应返回包含 success=false 的错误, got %v", err)
	}
}

// TestSearchModelScopeAtHTTPError 验证非 200 响应返回错误。
func TestSearchModelScopeAtHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	withModelScopeBases(t, srv.URL, srv.URL)

	if _, err := searchModelScope("q"); err == nil {
		t.Error("503 响应应返回错误")
	}
}

// TestListModelScopeFilesAt 验证 ModelScope Legacy 文件列表：
//   - 仅保留 Type=="blob" 且小写 .gguf 结尾的条目；
//   - Size 数字与字符串两种形态都转 int64；
//   - 目录（tree）、非 gguf blob、README 被过滤。
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
		t.Fatalf("文件数 = %d, want 3（tree/非 gguf blob/README 应过滤）: %+v", len(files), files)
	}
	if files[0].Filename != "model-q4_k_m.gguf" || files[0].Size != 100 {
		t.Errorf("files[0] = %+v, want model-q4_k_m.gguf/100", files[0])
	}
	if files[1].Filename != "model-f16.gguf" || files[1].Size != 200 {
		t.Errorf("files[1] = %+v, want model-f16.gguf/200（字符串 Size 应解析）", files[1])
	}
	if files[2].Filename != "MODEL.UPPER.GGUF" || files[2].Size != 500 {
		t.Errorf("files[2] = %+v, want MODEL.UPPER.GGUF/500（大小写不敏感匹配 .gguf）", files[2])
	}
}

// TestListModelScopeFilesAtCodeError 验证响应 Code!=200 时返回错误。
func TestListModelScopeFilesAtCodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"Code": 500, "Data": {"Files": []}}`))
	}))
	defer srv.Close()
	withModelScopeBases(t, srv.URL, srv.URL)

	if _, err := listModelScopeFiles("author/model"); err == nil || !strings.Contains(err.Error(), "Code=500") {
		t.Errorf("Code=500 应返回包含 Code=500 的错误, got %v", err)
	}
}

// TestListModelScopeFilesAtHTTPError 验证非 200 响应返回错误。
func TestListModelScopeFilesAtHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	withModelScopeBases(t, srv.URL, srv.URL)

	if _, err := listModelScopeFiles("author/missing"); err == nil {
		t.Error("404 响应应返回错误")
	}
}

// TestBuildModelScopeDownloadURL 验证下载 URL 拼接与 PathEscape 转义：
// modelID 与 fileName 都经 url.PathEscape 转义（"/"、"空格、中文字符均被
// 转义），保证含空格/中文文件名的模型可以正确下载。
func TestBuildModelScopeDownloadURL(t *testing.T) {
	base := "https://modelscope.cn/api/v1/models"

	got := buildModelScopeDownloadURL(base, "author/model", "model q4.gguf")
	want := base + "/" + url.PathEscape("author/model") + "/repo?Revision=master&FilePath=" + url.PathEscape("model q4.gguf")
	if got != want {
		t.Errorf("URL = %q, want %q（modelID/fileName 应 PathEscape 转义）", got, want)
	}
	// 明确断言转义结果，防止 PathEscape 行为变化后测试失真
	if !strings.Contains(got, "/author%2Fmodel/repo?") {
		t.Errorf("modelID 中的 / 应转义为 %%2F: %q", got)
	}
	if !strings.Contains(got, "FilePath=model%20q4.gguf") {
		t.Errorf("文件名中的空格应转义为 %%20: %q", got)
	}

	// 中文文件名：UTF-8 逐字节百分号转义
	gotCN := buildModelScopeDownloadURL(base, "author/model", "模型文件.gguf")
	wantCN := base + "/author%2Fmodel/repo?Revision=master&FilePath=%E6%A8%A1%E5%9E%8B%E6%96%87%E4%BB%B6.gguf"
	if gotCN != wantCN {
		t.Errorf("中文文件名 URL = %q, want %q", gotCN, wantCN)
	}
}

// newModelScopeDescServer 起一个模拟 ModelScope README 的本地服务：区分
// 正常描述、超长段落、无描述段落与不存在的路径。ModelScope 描述走 repo 端点
// （{legacyBase}/{PathEscape(modelID)}/repo?FilePath=README.md），modelID 嵌在
// URL 路径中，以路径为路由判据。
func newModelScopeDescServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/author/model/repo":
			w.Write([]byte("---\nlicense: apache-2.0\n---\n\n# 标题\n\n第一段自然语言描述，用于验证 ModelScope front-matter 跳过与段落提取。\n\n## 子标题\n\n不应被返回的第二段。\n"))
		case "/author/longmodel/repo":
			// 构造一段超过 200 个 rune 的段落（70×3=210 rune），验证截断与省略号
			para := strings.Repeat("长描述", 70)
			w.Write([]byte("---\ntags: test\n---\n\n# 标题\n\n" + para + "\n"))
		case "/author/nodesc/repo":
			// README 存在但没有可用的描述段落（全为标题）
			w.Write([]byte("---\nlicense: mit\n---\n\n# 只有标题\n\n## 另一个标题\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestGetModelScopeDescriptionAt 验证 ModelScope README 描述提取：跳过 YAML
// front-matter 与标题行，返回首个自然语言段落（与 HF 共用 extractDescription）。
func TestGetModelScopeDescriptionAt(t *testing.T) {
	srv := newModelScopeDescServer(t)
	defer srv.Close()
	withModelScopeBases(t, srv.URL, srv.URL)

	desc, err := getModelScopeDescription("author/model")
	if err != nil {
		t.Fatal(err)
	}
	if desc != "第一段自然语言描述，用于验证 ModelScope front-matter 跳过与段落提取。" {
		t.Errorf("描述 = %q，不应包含 front-matter 或标题行", desc)
	}
	if strings.Contains(desc, "---") || strings.Contains(desc, "license") || strings.Contains(desc, "#") {
		t.Errorf("描述不应包含 front-matter 或标题内容: %q", desc)
	}
}

// TestGetModelScopeDescriptionAtTruncate 验证超长段落按 200 个 rune 截断并追加省略号。
func TestGetModelScopeDescriptionAtTruncate(t *testing.T) {
	srv := newModelScopeDescServer(t)
	defer srv.Close()
	withModelScopeBases(t, srv.URL, srv.URL)

	desc, err := getModelScopeDescriptionAt(srv.URL, "author/longmodel")
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

// TestGetModelScopeDescriptionAtNoDescription 验证 README 存在但无描述段落时
// 返回空串与 nil 错误（静默处理，与 HF 一致）。
func TestGetModelScopeDescriptionAtNoDescription(t *testing.T) {
	srv := newModelScopeDescServer(t)
	defer srv.Close()
	withModelScopeBases(t, srv.URL, srv.URL)

	desc, err := getModelScopeDescriptionAt(srv.URL, "author/nodesc")
	if err != nil {
		t.Fatal(err)
	}
	if desc != "" {
		t.Errorf("无描述段落时应返回空串, got %q", desc)
	}
}

// TestGetModelScopeDescriptionAtNotFound 验证 README 返回 404 时返回错误。
func TestGetModelScopeDescriptionAtNotFound(t *testing.T) {
	srv := newModelScopeDescServer(t)
	defer srv.Close()
	withModelScopeBases(t, srv.URL, srv.URL)

	if _, err := getModelScopeDescriptionAt(srv.URL, "author/missing"); err == nil {
		t.Error("404 响应应返回错误")
	} else if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("错误信息应包含 HTTP 状态码: %v", err)
	}
}
