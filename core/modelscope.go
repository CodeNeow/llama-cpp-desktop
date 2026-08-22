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
//   - OpenAPI (modelscope.ai/openapi/v1): model search, returns
//     {success, data:{models}};
//   - Legacy API (modelscope.cn/api/v1/models): file listing and file download
//     (repo endpoint).
// Both bases are declared as package-level vars so tests can swap in a local
// httptest server (same style as hfMirrorBase via *At parameters, but
// ModelScope uses var injection because buildModelDownloadURL and friends do
// not take a base parameter).

var modelscopeOpenAPIBase = "https://modelscope.ai/openapi/v1"
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
// downloads / likes may be numbers or numeric strings in real ModelScope
// responses, so json.RawMessage is used with parseLenientInt for loose parsing;
// this avoids discarding entire results due to type mismatches.
type modelscopeModel struct {
	Path      string          `json:"Path"`
	Downloads json.RawMessage `json:"downloads"`
	Likes     json.RawMessage `json:"likes"`
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

// searchModelScope searches ModelScope models using the default OpenAPI base.
func searchModelScope(q string) ([]HFSearchResult, error) {
	return searchModelScopeAt(modelscopeOpenAPIBase, q)
}

// searchModelScopeAt fetches the model list from the ModelScope OpenAPI search
// endpoint (page_number=1&page_size=50). Response
// {success, data:{models:[...]}}: success!=true returns error; each model maps
// modelId=id=Path, author=first segment of Path, downloads/likes are parsed
// loosely, tags are passed through. **No hasGGUF filtering here**: ModelScope
// search responses do not include file lists, so hasGGUF filtering would empty
// the results; GGUF filtering happens at the file-list stage
// (listModelScopeFilesAt).
func searchModelScopeAt(openAPIBase, q string) ([]HFSearchResult, error) {
	apiURL := fmt.Sprintf("%s/models?search=%s&page_number=1&page_size=50",
		openAPIBase, url.QueryEscape(q))

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
