package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// ─── classifyModelType ──────────────────────────────────────────────

// TestClassifyModelType 验证模型类型分类：audio/image/video/chat 四类
// 均能正确判定，空输入归 chat。
func TestClassifyModelType(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"nil", nil, "chat"},
		{"empty", []string{}, "chat"},
		{"text only", []string{"text"}, "chat"},
		{"audio", []string{"audio"}, "audio"},
		{"image", []string{"image"}, "image"},
		{"video", []string{"video"}, "video"},
		{"audio and text", []string{"audio", "text"}, "audio"},
		{"image and text", []string{"image", "text"}, "image"},
		{"video and text", []string{"video", "text"}, "video"},
		{"text then audio", []string{"text", "audio"}, "audio"},
	}
	for _, tt := range tests {
		got := classifyModelType(tt.in)
		if got != tt.want {
			t.Errorf("%s: classifyModelType(%v) = %q, want %q", tt.name, tt.in, got, tt.want)
		}
	}
}

// ─── fetchRouterModels ──────────────────────────────────────────────

// routerModelsPayload 模拟 /models 响应：含 loaded/loading/sleeping/unloaded/downloading/failed 条目。
const routerModelsPayload = `{
  "data": [
    {"id":"chat-model","path":"/m/chat.gguf","status":{"value":"loaded","args":[]},"architecture":{"input_modalities":["text"],"output_modalities":["text"]}},
    {"id":"audio-model","path":"/m/audio.gguf","status":{"value":"loading","args":[]},"architecture":{"input_modalities":["text"],"output_modalities":["audio"]}},
    {"id":"image-model","path":"/m/img.gguf","status":{"value":"sleeping","args":[]},"architecture":{"input_modalities":["text","image"],"output_modalities":["image"]}},
    {"id":"video-model","path":"/m/vid.gguf","status":{"value":"loaded","args":[]},"architecture":{"input_modalities":["text"],"output_modalities":["video"]}},
    {"id":"unloaded-model","path":"/m/u.gguf","status":{"value":"unloaded","args":[]},"architecture":{"input_modalities":["text"],"output_modalities":["text"]}},
    {"id":"dl-model","path":"/m/dl.gguf","status":{"value":"downloading","args":[]},"architecture":{"input_modalities":["text"],"output_modalities":["text"]}},
    {"id":"failed-model","path":"/m/f.gguf","status":{"value":"failed","args":[]},"architecture":{"input_modalities":["text"],"output_modalities":["text"]}}
  ]
}`

// newRouterServer 起模拟路由器 API 的本地服务。
func newRouterServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/models":
			w.Write([]byte(routerModelsPayload))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestFetchRouterModels 验证 fetchRouterModels 过滤与映射：仅保留
// loaded/loading/sleeping，类型按 output_modalities 分类。
func TestFetchRouterModels(t *testing.T) {
	srv := newRouterServer(t)
	defer srv.Close()

	orig := routerBaseURL
	routerBaseURL = func(port int) string { return srv.URL }
	defer func() { routerBaseURL = orig }()

	models, err := fetchRouterModels(8080)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 4 {
		t.Fatalf("got %d models, want 4 (loaded/loading/sleeping/video loaded)", len(models))
	}
	byID := make(map[string]LoadedModel)
	for _, m := range models {
		byID[m.ID] = m
	}
	if m, ok := byID["chat-model"]; !ok || m.Type != "chat" || m.Status != "loaded" {
		t.Errorf("chat-model = %+v, want type=chat status=loaded", m)
	}
	if m, ok := byID["audio-model"]; !ok || m.Type != "audio" || m.Status != "loading" {
		t.Errorf("audio-model = %+v, want type=audio status=loading", m)
	}
	if m, ok := byID["image-model"]; !ok || m.Type != "image" || m.Status != "sleeping" {
		t.Errorf("image-model = %+v, want type=image status=sleeping", m)
	}
	if m, ok := byID["video-model"]; !ok || m.Type != "video" || m.Status != "loaded" {
		t.Errorf("video-model = %+v, want type=video status=loaded", m)
	}
	if _, ok := byID["unloaded-model"]; ok {
		t.Error("unloaded-model 应被过滤")
	}
	if _, ok := byID["dl-model"]; ok {
		t.Error("downloading 应被过滤")
	}
	if _, ok := byID["failed-model"]; ok {
		t.Error("failed 应被过滤")
	}
}

// TestFetchRouterModelsConnectionError 验证连接失败时返回错误。
func TestFetchRouterModelsConnectionError(t *testing.T) {
	orig := routerBaseURL
	routerBaseURL = func(port int) string { return "http://127.0.0.1:1" }
	defer func() { routerBaseURL = orig }()

	_, err := fetchRouterModels(8080)
	if err == nil {
		t.Error("连接失败应返回错误")
	}
}

// ─── unloadRouterModel ──────────────────────────────────────────────

// newUnloadServer 起模拟卸载端点的本地服务。
func newUnloadServer(t *testing.T, success bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unload 请求方法 = %s, want POST", r.Method)
		}
		if r.URL.Path != "/models/unload" {
			t.Errorf("unload 路径 = %s, want /models/unload", r.URL.Path)
		}
		var req routerUnloadRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "model-to-unload" {
			t.Errorf("unload body model = %q, want model-to-unload", req.Model)
		}
		if success {
			w.Write([]byte(`{"success":true}`))
		} else {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"not found"}`))
		}
	}))
}

// TestUnloadRouterModel 验证卸载请求的方法、路径与 body 正确，成功时返回 nil。
func TestUnloadRouterModel(t *testing.T) {
	srv := newUnloadServer(t, true)
	defer srv.Close()

	orig := routerBaseURL
	routerBaseURL = func(port int) string { return srv.URL }
	defer func() { routerBaseURL = orig }()

	err := unloadRouterModel(8080, "model-to-unload")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestUnloadRouterModelError 验证非 2xx 响应返回错误。
func TestUnloadRouterModelError(t *testing.T) {
	srv := newUnloadServer(t, false)
	defer srv.Close()

	orig := routerBaseURL
	routerBaseURL = func(port int) string { return srv.URL }
	defer func() { routerBaseURL = orig }()

	err := unloadRouterModel(8080, "model-to-unload")
	if err == nil {
		t.Error("非 2xx 应返回错误")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("错误信息应包含服务器返回的错误: %v", err)
	}
}

// TestUnloadRouterModelEmptyID 验证空 id 立即返回错误，不发送请求。
func TestUnloadRouterModelEmptyID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("空 id 不应发送请求")
	}))
	defer srv.Close()

	orig := routerBaseURL
	routerBaseURL = func(port int) string { return srv.URL }
	defer func() { routerBaseURL = orig }()

	err := unloadRouterModel(8080, "")
	if err == nil {
		t.Error("空 id 应返回错误")
	}
}

// ─── serverPort 读写 ────────────────────────────────────────────────

// TestServerPortReadWrite 验证 setServerPort / getServerPort 读写一致。
func TestServerPortReadWrite(t *testing.T) {
	saveServerState(t)

	setServerPort(0)
	if p := getServerPort(); p != 0 {
		t.Errorf("初始 port = %d, want 0", p)
	}
	setServerPort(8080)
	if p := getServerPort(); p != 8080 {
		t.Errorf("写入 8080 后 port = %d, want 8080", p)
	}
	setServerPort(0)
	if p := getServerPort(); p != 0 {
		t.Errorf("清零后 port = %d, want 0", p)
	}
}

// TestConcurrentServerPort 验证 serverPort 并发读写安全。
func TestConcurrentServerPort(t *testing.T) {
	saveServerState(t)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			setServerPort(i)
			_ = getServerPort()
		}(i)
	}
	wg.Wait()
}
