package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// ─── Router API（llama-server 路由器模式）─────────────────────────────
//
// 以下类型与函数封装 llama-server 路由器模式的 HTTP API（GET /models、
// POST /models/unload），供前端 TaskDock 展示内存模型列表与卸载。

// LoadedModel 表示路由器中当前加载/加载中/休眠的模型条目。
type LoadedModel struct {
	ID     string `json:"id"`     // 模型标识（与 API 响应的 id 一致）
	Type   string `json:"type"`   // 模型类型：chat | audio | image | video
	Status string `json:"status"` // loaded | loading | sleeping
}

// routerModelsResponse 对齐 llama-server GET /models 的 JSON 响应结构。
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

// routerUnloadRequest 是 POST /models/unload 的请求体。
type routerUnloadRequest struct {
	Model string `json:"model"`
}

// routerUnloadResponse 是 POST /models/unload 的响应体。
type routerUnloadResponse struct {
	Success bool `json:"success"`
}

// ─── URL 注入点 ─────────────────────────────────────────────────────
//
// routerBaseURL 声明为包级 var，便于测试通过替换注入本地 httptest 服务器
// （与 githubReleasesAPI / updateRepoAPI 同风格）。
var routerBaseURL = func(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

// ─── 模型类型分类 ────────────────────────────────────────────────────

// classifyModelType 按 output_modalities 判定模型类型：含 audio→audio；
// 含 image→image；含 video→video；否则 chat。视觉输入的聊天模型（如 LLaVA）
// output 为 text，自然归 chat。
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

// ─── 查询 / 卸载 ────────────────────────────────────────────────────

// fetchRouterModels 查询 llama-server 路由器模型列表，过滤出 loaded /
// loading / sleeping 条目并映射为 LoadedModel 切片。
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

// unloadRouterModel 向 llama-server 发送模型卸载请求。
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

// getServerPort 返回当前记录的服务器端口（0 表示未运行），并发安全。
func getServerPort() int {
	serverMu.Lock()
	port := serverPort
	serverMu.Unlock()
	return port
}

// setServerPort 设置当前服务器端口（启动成功时调用；0 表示未运行）。
func setServerPort(port int) {
	serverMu.Lock()
	serverPort = port
	serverMu.Unlock()
}
