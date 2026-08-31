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

// TestFetchRouterModelsNativeShape verifies the dual-shape parse: a newer
// llama.cpp direct-mode server answers GET /models with HTTP 200 and the
// native OpenAI listing (data[].id + object:"model", no per-entry status).
// The body must map to a resident chat model without a /v1/models fallback.
func TestFetchRouterModelsNativeShape(t *testing.T) {
	var v1Hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/models":
			w.Write([]byte(`{"object":"list","data":[{"id":"stories260K","object":"model","created":1750000000,"owned_by":"llama.cpp"}]}`))
		case "/v1/models":
			v1Hits++
			w.Write([]byte(`{"data":[{"id":"stories260K"}]}`))
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
	if v1Hits != 0 {
		t.Errorf("native /models hit must not fall back to /v1/models (%d hits)", v1Hits)
	}
	want := []LoadedModel{{ID: "stories260K", Type: "chat", Status: "loaded"}}
	if len(models) != 1 || models[0] != want[0] {
		t.Errorf("native listing = %+v, want %+v", models, want)
	}
}

// TestFetchRouterModelsRouterShapeNothingLoaded verifies the router-shape
// empty semantics survive the dual-shape parse: entries present but none in
// loaded/loading/sleeping must still yield an empty list — without a
// /v1/models fallback that would resurrect them as loaded.
func TestFetchRouterModelsRouterShapeNothingLoaded(t *testing.T) {
	var v1Hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/models":
			w.Write([]byte(`{"data":[{"id":"m1","status":{"value":"unloaded"},"architecture":{}},{"id":"m2","status":{"value":"failed"},"architecture":{}}]}`))
		case "/v1/models":
			v1Hits++
			w.Write([]byte(`{"data":[{"id":"m1"},{"id":"m2"}]}`))
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
	if len(models) != 0 {
		t.Errorf("router shape with nothing loaded = %+v, want empty", models)
	}
	if v1Hits != 0 {
		t.Errorf("router shape must not fall back to /v1/models (%d hits)", v1Hits)
	}
}

// TestFetchRouterModelsEmptyData verifies a 200 with an empty or absent data
// array (both shapes empty) yields an empty list without an error.
func TestFetchRouterModelsEmptyData(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"empty array", `{"data":[]}`},
		{"missing data", `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			orig := routerBaseURL
			routerBaseURL = func(port int) string { return srv.URL }
			defer func() { routerBaseURL = orig }()

			models, err := fetchRouterModels(8080)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(models) != 0 {
				t.Errorf("models = %+v, want empty", models)
			}
		})
	}
}

// TestFetchRouterModelsLenientStatusShapes verifies the status leniency
// contract: entries with a missing / null / empty status, and bodies whose
// status fails to decode as an object (plain string — upstream shape drift),
// still map to resident loaded chat models instead of erroring out.
func TestFetchRouterModelsLenientStatusShapes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"id":"a","status":"loaded"},{"id":"b","status":null},{"id":"c"},{"id":"d","status":{}}]}`))
	}))
	defer srv.Close()

	orig := routerBaseURL
	routerBaseURL = func(port int) string { return srv.URL }
	defer func() { routerBaseURL = orig }()

	models, err := fetchRouterModels(8080)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []LoadedModel{
		{ID: "a", Type: "chat", Status: "loaded"},
		{ID: "b", Type: "chat", Status: "loaded"},
		{ID: "c", Type: "chat", Status: "loaded"},
		{ID: "d", Type: "chat", Status: "loaded"},
	}
	if len(models) != len(want) {
		t.Fatalf("models = %+v, want %d entries", models, len(want))
	}
	for i, w := range want {
		if models[i] != w {
			t.Errorf("models[%d] = %+v, want %+v", i, models[i], w)
		}
	}
}

// TestFetchRouterModelsGarbageBody verifies a non-JSON body still errors.
func TestFetchRouterModelsGarbageBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json at all"))
	}))
	defer srv.Close()

	orig := routerBaseURL
	routerBaseURL = func(port int) string { return srv.URL }
	defer func() { routerBaseURL = orig }()

	if _, err := fetchRouterModels(8080); err == nil {
		t.Error("garbage body should return an error")
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
