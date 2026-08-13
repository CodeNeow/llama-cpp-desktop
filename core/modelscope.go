package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ─── ModelScope API 客户端 ─────────────────────────────────────────
//
// ModelScope 有两套端点：
//   - OpenAPI（modelscope.ai/openapi/v1）：模型搜索，返回 {success, data:{models}}；
//   - Legacy API（modelscope.cn/api/v1/models）：文件列表与文件下载（repo 端点）。
// 两处 Base 均声明为包级 var 供测试通过替换注入本地 httptest 服务器（与
// hfMirrorBase 经 *At 参数注入同风格，但 ModelScope 走 var 注入以贴合
// buildModelDownloadURL 等不传 Base 的调用面）。

var modelscopeOpenAPIBase = "https://modelscope.ai/openapi/v1"
var modelscopeLegacyBase = "https://modelscope.cn/api/v1/models"

// modelscopeSearchResponse 是 ModelScope OpenAPI 搜索的顶层响应结构。
type modelscopeSearchResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Models []modelscopeModel `json:"models"`
	} `json:"data"`
}

// modelscopeModel 是 OpenAPI 搜索返回的单个模型项。downloads / likes 在
// ModelScope 实际响应里可能是数字也可能是数字字符串，用 json.RawMessage 走
// parseLenientInt 宽松解析，避免类型不匹配导致整条结果丢弃。
type modelscopeModel struct {
	Path      string          `json:"Path"`
	Downloads json.RawMessage `json:"downloads"`
	Likes     json.RawMessage `json:"likes"`
	Tags      []string        `json:"tags"`
}

// parseLenientInt 宽松解析整数：数字直接取值，字符串按 ParseInt 解析，缺失或
// 非法返回 0。ModelScope 的 downloads/likes 字段类型不稳定，不能依赖 JSON 数值。
func parseLenientInt(raw json.RawMessage) int {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			return n
		}
		return 0
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n
	}
	return 0
}

// searchModelScope 使用默认 OpenAPI Base 搜索 ModelScope 模型。
func searchModelScope(q string) ([]HFSearchResult, error) {
	return searchModelScopeAt(modelscopeOpenAPIBase, q)
}

// searchModelScopeAt 向 ModelScope OpenAPI 搜索接口拉取模型列表
// （page_number=1&page_size=50）。响应 {success, data:{models:[...]}}：
// success!=true 返回错误；每个模型映射 modelId=id=Path、author=Path 第一段、
// downloads/likes 宽松解析、tags 原样映射。**不做 hasGGUF 过滤**：ModelScope
// 搜索响应不带文件列表，按 hasGGUF 过滤会把结果清空；GGUF 过滤在文件列表
// 阶段（listModelScopeFilesAt）完成。
func searchModelScopeAt(openAPIBase, q string) ([]HFSearchResult, error) {
	apiURL := fmt.Sprintf("%s/models?search=%s&page_number=1&page_size=50",
		openAPIBase, url.QueryEscape(q))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "llama-gui")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ModelScope API returned status %d", resp.StatusCode)
	}

	var raw modelscopeSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	if !raw.Success {
		return nil, fmt.Errorf("ModelScope 搜索失败: success=false")
	}

	results := make([]HFSearchResult, 0, len(raw.Data.Models))
	for _, m := range raw.Data.Models {
		path := m.Path
		if path == "" {
			continue
		}
		author := path
		if parts := strings.SplitN(path, "/", 2); len(parts) == 2 {
			author = parts[0]
		}
		results = append(results, HFSearchResult{
			ID:        path,
			ModelID:   path,
			Author:    author,
			Downloads: parseLenientInt(m.Downloads),
			Likes:     parseLenientInt(m.Likes),
			Tags:      m.Tags,
		})
	}
	return results, nil
}

// listModelScopeFiles 使用默认 Legacy Base 列出模型仓库文件（仅 GGUF blob）。
func listModelScopeFiles(modelID string) ([]HFFileOut, error) {
	return listModelScopeFilesAt(modelscopeLegacyBase, modelID)
}

// listModelScopeFilesAt 调用 ModelScope Legacy 文件列表接口
// `{legacyBase}/{modelID}/repo/files?Recursive=True`。响应
// {Code: int, Data: {Files: [{Path, Size, Type}]}}；Code!=200 返回错误；
// 仅保留 Type=="blob" 且小写 .gguf 结尾的条目（GGUF 过滤在文件列表阶段做）；
// Size 可能是数字或字符串，宽松转 int64。
func listModelScopeFilesAt(legacyBase, modelID string) ([]HFFileOut, error) {
	apiURL := fmt.Sprintf("%s/%s/repo/files?Recursive=True",
		legacyBase, url.PathEscape(modelID))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "llama-gui")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ModelScope API returned status %d", resp.StatusCode)
	}

	var raw struct {
		Code int `json:"Code"`
		Data struct {
			Files []struct {
				Path string          `json:"Path"`
				Size json.RawMessage `json:"Size"`
				Type string          `json:"Type"`
			} `json:"Files"`
		} `json:"Data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	if raw.Code != 200 {
		return nil, fmt.Errorf("ModelScope 文件列表失败: Code=%d", raw.Code)
	}

	files := make([]HFFileOut, 0, len(raw.Data.Files))
	for _, f := range raw.Data.Files {
		if f.Type != "blob" || !strings.HasSuffix(strings.ToLower(f.Path), ".gguf") {
			continue
		}
		files = append(files, HFFileOut{
			Filename: f.Path,
			Size:     parseLenientInt64(f.Size),
		})
	}
	return files, nil
}

// parseLenientInt64 宽松解析 int64：数字直接取值，字符串按 ParseInt 解析，
// 缺失或非法返回 0（与 parseLenientInt 同一策略，覆盖 Size 字段类型不稳定）。
func parseLenientInt64(raw json.RawMessage) int64 {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
			return n
		}
		return 0
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n
	}
	return 0
}

// buildModelScopeDownloadURL 拼接 ModelScope 文件下载 URL：
// `{legacyBase}/{PathEscape(modelID)}/repo?Revision=master&FilePath={PathEscape(fileName)}`。
func buildModelScopeDownloadURL(legacyBase, modelID, fileName string) string {
	return fmt.Sprintf("%s/%s/repo?Revision=master&FilePath=%s",
		legacyBase, url.PathEscape(modelID), url.PathEscape(fileName))
}

// getModelScopeDescription 使用默认 Legacy Base 获取模型 README 描述。
func getModelScopeDescription(modelID string) (string, error) {
	return getModelScopeDescriptionAt(modelscopeLegacyBase, modelID)
}

// getModelScopeDescriptionAt 通过 repo 端点（FilePath=README.md）获取模型
// README 正文，交给共享的 extractDescription 提取自然语言描述（跳过 YAML
// front-matter + 取首个非空非 # 段落 + 200 rune 截断，与 HF 一致）。
// 非 200 返回错误；README 存在但没有描述段落时返回空串与 nil（静默）。
func getModelScopeDescriptionAt(legacyBase, modelID string) (string, error) {
	apiURL := buildModelScopeDownloadURL(legacyBase, modelID, "README.md")

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "llama-gui")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("README 获取失败: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return extractDescription(string(body)), nil
}
