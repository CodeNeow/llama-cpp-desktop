package core

import (
	"net/http"
	"net/http/httptest"
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

// TestSearchHFMirrorAt 验证 HF 搜索：过滤掉无 GGUF 的模型，保留 embedding 类型。
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
