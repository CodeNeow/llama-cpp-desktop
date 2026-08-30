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

// TestClassifyModelType verifies model-type classification: audio/image/video/chat
// are all classified correctly; empty input falls back to chat.
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

// routerModelsPayload simulates a /models response containing loaded/loading/sleeping/unloaded/downloading/failed entries.
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

// newRouterServer starts a local server simulating the router API.
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

// TestFetchRouterModels verifies fetchRouterModels filtering and mapping: only
// loaded/loading/sleeping are retained, types are classified by output_modalities.
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
		t.Fatalf("got %d models, want 4 (loaded/loading/sleeping/video-loaded)", len(models))
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
		t.Error("unloaded-model should be filtered out")
	}
	if _, ok := byID["dl-model"]; ok {
		t.Error("downloading should be filtered out")
	}
	if _, ok := byID["failed-model"]; ok {
		t.Error("failed should be filtered out")
	}
}

// TestFetchRouterModelsConnectionError verifies a connection failure returns an error.
func TestFetchRouterModelsConnectionError(t *testing.T) {
	orig := routerBaseURL
	routerBaseURL = func(port int) string { return "http://127.0.0.1:1" }
	defer func() { routerBaseURL = orig }()

	_, err := fetchRouterModels(8080)
	if err == nil {
		t.Error("connection failure should return error")
	}
}

// ─── unloadRouterModel ──────────────────────────────────────────────

// newUnloadServer starts a local server simulating the unload endpoint.
func newUnloadServer(t *testing.T, success bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unload request method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/models/unload" {
			t.Errorf("unload path = %s, want /models/unload", r.URL.Path)
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

// TestUnloadRouterModel verifies the unload request has the correct method, path, and
// body; returns nil on success.
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

// TestUnloadRouterModelError verifies a non-2xx response returns an error.
func TestUnloadRouterModelError(t *testing.T) {
	srv := newUnloadServer(t, false)
	defer srv.Close()

	orig := routerBaseURL
	routerBaseURL = func(port int) string { return srv.URL }
	defer func() { routerBaseURL = orig }()

	err := unloadRouterModel(8080, "model-to-unload")
	if err == nil {
		t.Error("non-2xx should return error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error message should contain the server-returned error: %v", err)
	}
}

// TestUnloadRouterModelEmptyID verifies an empty id returns an error immediately
// without sending a request.
func TestUnloadRouterModelEmptyID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("empty id should not send any request")
	}))
	defer srv.Close()

	orig := routerBaseURL
	routerBaseURL = func(port int) string { return srv.URL }
	defer func() { routerBaseURL = orig }()

	err := unloadRouterModel(8080, "")
	if err == nil {
		t.Error("empty id should return error")
	}
}

// ─── serverPort read/write ─────────────────────────────────────────

// TestServerPortReadWrite verifies setServerPort / getServerPort round-trip consistency.
func TestServerPortReadWrite(t *testing.T) {
	saveServerState(t)

	setServerPort(0)
	if p := getServerPort(); p != 0 {
		t.Errorf("initial port = %d, want 0", p)
	}
	setServerPort(8080)
	if p := getServerPort(); p != 8080 {
		t.Errorf("after writing 8080, port = %d, want 8080", p)
	}
	setServerPort(0)
	if p := getServerPort(); p != 0 {
		t.Errorf("after resetting, port = %d, want 0", p)
	}
}

// TestConcurrentServerPort verifies concurrent reads/writes of serverPort are safe.
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

// ─── Direct-mode fallbacks (Android) ────────────────────────────────

// TestFetchRouterModelsDirectFallback404 verifies the direct-mode fallback: a
// server without the router /models route answers 404 and fetchRouterModels
// degrades to the OpenAI-compatible /v1/models listing, mapping every
// data[].id to a loaded chat model.
func TestFetchRouterModelsDirectFallback404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/models":
			// Direct-mode llama-server has no router route at all.
			http.NotFound(w, r)
		case "/v1/models":
			w.Write([]byte(`{"data":[{"id":"model-a"},{"id":"model-b"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	orig := routerBaseURL
	routerBaseURL = func(port int) string { return srv.URL }
	defer func() { routerBaseURL = orig }()

	models, err := fetchRouterModels(8080)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("got %d models, want 2", len(models))
	}
	want := []LoadedModel{
		{ID: "model-a", Type: "chat", Status: "loaded"},
		{ID: "model-b", Type: "chat", Status: "loaded"},
	}
	for i, w := range want {
		if models[i] != w {
			t.Errorf("models[%d] = %+v, want %+v", i, models[i], w)
		}
	}
}

// TestUnloadRouterModel404 verifies the direct-mode unload behavior: a 404
// from /models/unload surfaces the guided "stop the service instead" error
// (asserted in English by pinning the UI language — tr() follows the machine
// locale otherwise).
func TestUnloadRouterModel404(t *testing.T) {
	withLanguage(t, "en")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	orig := routerBaseURL
	routerBaseURL = func(port int) string { return srv.URL }
	defer func() { routerBaseURL = orig }()

	err := unloadRouterModel(8080, "resident-model")
	if err == nil {
		t.Fatal("404 should return an error")
	}
	if !strings.Contains(err.Error(), "direct mode: unload not supported") {
		t.Errorf("error = %v, want the direct-mode guidance", err)
	}
}
