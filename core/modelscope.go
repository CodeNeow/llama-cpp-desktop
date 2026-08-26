package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ─── ModelScope API Client ─────────────────────────────────────────
//
// ModelScope exposes two endpoint sets:
//   - OpenAPI (modelscope.cn/openapi/v1): model search, returns
//     {success, data:{models}}. modelscope.cn is the China-accessible host and
//     is used as the primary; modelscope.ai is kept as a fallback for
//     international reachability.
//   - Legacy API (modelscope.cn/api/v1/models): file listing and file download
//     (repo endpoint).
// Both bases are declared as package-level vars so tests can swap in a local
// httptest server (same style as hfMirrorBase via *At parameters, but
// ModelScope uses var injection because buildModelDownloadURL and friends do
// not take a base parameter).

var modelscopeOpenAPIBase = "https://modelscope.cn/openapi/v1"
var modelscopeOpenAPIFallback = "https://modelscope.ai/openapi/v1"
var modelscopeLegacyBase = "https://modelscope.cn/api/v1/models"

// modelscopeSearchResponse is the top-level response structure for ModelScope
// OpenAPI search.
type modelscopeSearchResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Models []modelscopeModel `json:"models"`
	} `json:"data"`
}

// modelscopeModel is a single model item returned by OpenAPI search.
// The OpenAPI response uses "id" (e.g. "Qwen/Qwen2.5-0.5B") for the repo path;
// the legacy endpoint used "Path", so both are parsed with "id" winning and
// "Path" as a fallback. downloads / likes may be numbers or numeric strings,
// so json.RawMessage is used with parseLenientInt for loose parsing.
type modelscopeModel struct {
	ID        string          `json:"id"`
	Path      string          `json:"Path"`
	Downloads json.RawMessage `json:"downloads"`
	Likes     json.RawMessage `json:"likes"`
	Tasks     []string        `json:"tasks"`
	Tags      []string        `json:"tags"`
}

// parseLenientInt loosely parses an integer: takes the number directly, parses
// strings via ParseInt, and returns 0 when missing or invalid. ModelScope's
// downloads/likes fields have unstable types, so JSON numerics cannot be
// relied upon.
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

// modelScopeIsGGUF reports whether a ModelScope model is a GGUF repository,
// using the only GGUF signals available at search time (the search response
// does not include file lists): the repo id or any tag contains "gguf"
// (case-insensitive). This covers library:gguf, custom_tag:gguf, and -GGUF
// repo names, keeping ModelScope search aligned with HF's GGUF-only results.
func modelScopeIsGGUF(modelID string, tags []string) bool {
	if strings.Contains(strings.ToLower(modelID), "gguf") {
		return true
	}
	for _, tag := range tags {
		if strings.Contains(strings.ToLower(tag), "gguf") {
			return true
		}
	}
	return false
}

// searchModelScope searches ModelScope models using the default OpenAPI base,
// falling back to the alternate host when the primary fails (covers
// region-specific reachability differences between modelscope.cn and
// modelscope.ai).
func searchModelScope(q string) ([]HFSearchResult, error) {
	res, err := searchModelScopeAt(modelscopeOpenAPIBase, q)
	if err == nil {
		return res, nil
	}
	return searchModelScopeAt(modelscopeOpenAPIFallback, q)
}

// searchModelScopeAt fetches the model list from the ModelScope OpenAPI search
// endpoint (page_number=1&page_size=50). Response
// {success, data:{models:[...]}}: success!=true returns error; each model maps
// modelId=id (Path fallback), author=first segment of the id, downloads/likes
// are parsed loosely, the first task becomes the pipeline tag, and tags pass
// through. The query is biased toward GGUF (see below) and results are further
// filtered to GGUF repositories (modelScopeIsGGUF) as a safety net, so the
// search stays aligned with HF's GGUF-only results and the detail page always
// has downloadable files.
func searchModelScopeAt(openAPIBase, q string) ([]HFSearchResult, error) {
	// Append " GGUF" to the query so ModelScope ranks GGUF repositories first.
	// The OpenAPI search has no server-side GGUF/tag filter (the tags= param is
	// ignored), so biasing the query is the lightweight way to surface GGUF
	// models; the name/tag post-filter below is a safety net for any non-GGUF
	// result that still slips in.
	apiURL := fmt.Sprintf("%s/models?search=%s&page_number=1&page_size=50",
		openAPIBase, url.QueryEscape(q+" GGUF"))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", appUserAgent())

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
		return nil, errors.New(tr("ModelScope 搜索失败: success=false", "ModelScope search failed: success=false"))
	}

	results := make([]HFSearchResult, 0, len(raw.Data.Models))
	for _, m := range raw.Data.Models {
		// OpenAPI uses "id"; fall back to legacy "Path" for robustness.
		path := m.ID
		if path == "" {
			path = m.Path
		}
		if path == "" {
			continue
		}
		// Keep only GGUF repositories: ModelScope search does not return file
		// lists, so GGUF presence is inferred from the repo id or tags. This
		// matches HF search (GGUF-only) and prevents detail pages that show
		// "no files" for Transformer-format models.
		if !modelScopeIsGGUF(path, m.Tags) {
			continue
		}
		author := path
		if parts := strings.SplitN(path, "/", 2); len(parts) == 2 {
			author = parts[0]
		}
		result := HFSearchResult{
			ID:        path,
			ModelID:   path,
			Author:    author,
			Downloads: parseLenientInt(m.Downloads),
			Likes:     parseLenientInt(m.Likes),
			Tags:      m.Tags,
		}
		if len(m.Tasks) > 0 {
			result.PipelineTag = m.Tasks[0]
		}
		results = append(results, result)
	}
	return results, nil
}

// listModelScopeFiles lists model repo files (GGUF blobs only) using the
// default Legacy base.
func listModelScopeFiles(modelID string) ([]HFFileOut, error) {
	return listModelScopeFilesAt(modelscopeLegacyBase, modelID)
}

// listModelScopeFilesAt calls the ModelScope Legacy file-list endpoint
// `{legacyBase}/{modelID}/repo/files?Recursive=True`. Response
// {Code: int, Data: {Files: [{Path, Size, Type}]}}: Code!=200 returns error;
// only Type=="blob" entries ending with lowercase .gguf are kept (GGUF
// filtering happens at the file-list stage); Size may be a number or string,
// loosely converted to int64.
func listModelScopeFilesAt(legacyBase, modelID string) ([]HFFileOut, error) {
	apiURL := fmt.Sprintf("%s/%s/repo/files?Recursive=True",
		legacyBase, url.PathEscape(modelID))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", appUserAgent())

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
		return nil, fmt.Errorf(tr("ModelScope 文件列表失败: Code=%d", "ModelScope file list failed: Code=%d"), raw.Code)
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

// parseLenientInt64 loosely parses an int64: takes the number directly, parses
// strings via ParseInt, and returns 0 when missing or invalid (same strategy
// as parseLenientInt, covering unstable Size field types).
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

// buildModelScopeDownloadURL builds a ModelScope file download URL:
// `{legacyBase}/{PathEscape(modelID)}/repo?Revision=master&FilePath={PathEscape(fileName)}`.
func buildModelScopeDownloadURL(legacyBase, modelID, fileName string) string {
	return fmt.Sprintf("%s/%s/repo?Revision=master&FilePath=%s",
		legacyBase, url.PathEscape(modelID), url.PathEscape(fileName))
}

// getModelScopeDescription fetches a model README description using the
// default Legacy base.
func getModelScopeDescription(modelID string) (string, error) {
	return getModelScopeDescriptionAt(modelscopeLegacyBase, modelID)
}

// getModelScopeDescriptionAt fetches the README of a model via the repo
// endpoint (FilePath=README.md), then passes it to the shared
// extractDescription to extract natural-language description (skips YAML
// front-matter + takes first non-empty non-# paragraph + 200 rune truncation,
// matching HF behavior). Non-200 returns error; README present but without a
// description paragraph returns empty string and nil (silent).
func getModelScopeDescriptionAt(legacyBase, modelID string) (string, error) {
	apiURL := buildModelScopeDownloadURL(legacyBase, modelID, "README.md")

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", appUserAgent())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf(tr("README 获取失败: HTTP %d", "failed to fetch README: HTTP %d"), resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return extractDescription(string(body)), nil
}
