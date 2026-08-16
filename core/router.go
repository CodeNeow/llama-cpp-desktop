package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// ─── Router API (llama-server router mode) ─────────────────────────
//
// Types and functions wrapping the llama-server router HTTP API (GET /models,
// POST /models/unload) so the frontend TaskDock can list in-memory models and
// unload them.

// LoadedModel represents a model entry currently loaded / loading / sleeping
// in the router.
type LoadedModel struct {
	ID     string `json:"id"`     // Model ID (matches API response id)
	Type   string `json:"type"`   // Model type: chat | audio | image | video
	Status string `json:"status"` // loaded | loading | sleeping
}

// routerModelsResponse mirrors the llama-server GET /models JSON structure.
type routerModelsResponse struct {
	Data []routerModelItem `json:"data"`
}

type routerModelItem struct {
	ID           string            `json:"id"`
	Path         string            `json:"path"`
	Status       routerModelStatus `json:"status"`
	Architecture routerModelArch   `json:"architecture"`
}

type routerModelStatus struct {
	Value string   `json:"value"` // unloaded / loading / loaded / sleeping / downloading / failed
	Args  []string `json:"args"`
}

type routerModelArch struct {
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
}

// routerUnloadRequest is the request body for POST /models/unload.
type routerUnloadRequest struct {
	Model string `json:"model"`
}

// routerUnloadResponse is the response body for POST /models/unload.
type routerUnloadResponse struct {
	Success bool `json:"success"`
}

// ─── URL injection point ───────────────────────────────────────────
//
// routerBaseURL is declared as a package-level var so tests can swap in a
// local httptest server (same style as githubReleasesAPI / updateRepoAPI).
var routerBaseURL = func(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

// ─── Model type classification ─────────────────────────────────────

// classifyModelType determines the model type from output_modalities: audio
// if audio is present, image if image, video if video, otherwise chat. Chat
// models with vision input (e.g. LLaVA) output text, so they naturally fall
// into chat.
func classifyModelType(outputModalities []string) string {
	for _, m := range outputModalities {
		switch m {
		case "audio":
			return "audio"
		case "image":
			return "image"
		case "video":
			return "video"
		}
	}
	return "chat"
}

// ─── Query / Unload ────────────────────────────────────────────────

// fetchRouterModels queries the llama-server router for the model list,
// filters to loaded / loading / sleeping entries, and maps them to a
// LoadedModel slice.
func fetchRouterModels(port int) ([]LoadedModel, error) {
	base := routerBaseURL(port)
	url := base + "/models"

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch router models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch router models: HTTP %d", resp.StatusCode)
	}

	var raw routerModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("fetch router models: %w", err)
	}

	out := make([]LoadedModel, 0, len(raw.Data))
	for _, item := range raw.Data {
		switch item.Status.Value {
		case "loaded", "loading", "sleeping":
			out = append(out, LoadedModel{
				ID:     item.ID,
				Type:   classifyModelType(item.Architecture.OutputModalities),
				Status: item.Status.Value,
			})
		}
	}
	return out, nil
}

// unloadRouterModel sends a model unload request to llama-server.
func unloadRouterModel(port int, id string) error {
	if id == "" {
		return errors.New("unload router model: empty model id")
	}

	base := routerBaseURL(port)
	url := base + "/models/unload"

	body := routerUnloadRequest{Model: id}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("unload router model: %w", err)
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("unload router model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != "" {
			return fmt.Errorf("unload router model: %s (HTTP %d)", errResp.Error, resp.StatusCode)
		}
		return fmt.Errorf("unload router model: HTTP %d", resp.StatusCode)
	}

	var result routerUnloadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("unload router model: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("unload router model: server returned success=false")
	}
	return nil
}

// getServerPort returns the currently recorded server port (0 means not
// running); safe for concurrent use.
func getServerPort() int {
	serverMu.Lock()
	port := serverPort
	serverMu.Unlock()
	return port
}

// setServerPort sets the current server port (called on successful start; 0
// means not running).
func setServerPort(port int) {
	serverMu.Lock()
	serverPort = port
	serverMu.Unlock()
}
