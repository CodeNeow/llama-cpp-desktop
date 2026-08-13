package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// withTempCwd 切到临时目录并在测试结束后恢复原工作目录。
// loadConfig/saveConfig 读取相对路径 configFile，测试需隔离工作目录。
func withTempCwd(t *testing.T) string {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Errorf("恢复工作目录失败: %v", err)
		}
	})
	return tmp
}

// saveConfigState 记录配置相关全局状态，供测试结束后恢复，避免污染其他测试。
func saveConfigState(t *testing.T) (origModels map[string]ModelConfig, origServer ServerConfig, origTheme string, origDir string, origModelsDir string) {
	t.Helper()
	modelConfigsMu.Lock()
	origModels = make(map[string]ModelConfig, len(cachedModelConfigs))
	for k, v := range cachedModelConfigs {
		origModels[k] = v
	}
	modelConfigsMu.Unlock()
	serverConfigMu.Lock()
	origServer = cachedServerConfig
	serverConfigMu.Unlock()
	configMu.Lock()
	origTheme = currentTheme
	configMu.Unlock()
	customLlamaCppMu.Lock()
	origDir = customLlamaCppDir
	customLlamaCppMu.Unlock()
	modelsDirMu.Lock()
	origModelsDir = customModelsDir
	modelsDirMu.Unlock()
	t.Cleanup(func() {
		modelConfigsMu.Lock()
		cachedModelConfigs = origModels
		modelConfigsMu.Unlock()
		serverConfigMu.Lock()
		cachedServerConfig = origServer
		serverConfigMu.Unlock()
		configMu.Lock()
		currentTheme = origTheme
		configMu.Unlock()
		customLlamaCppMu.Lock()
		customLlamaCppDir = origDir
		customLlamaCppMu.Unlock()
		modelsDirMu.Lock()
		customModelsDir = origModelsDir
		modelsDirMu.Unlock()
	})
	return
}

// TestSaveLoadConfigRoundTrip 验证 saveConfig 写入、loadConfig 读回的一致性。
func TestSaveLoadConfigRoundTrip(t *testing.T) {
	tmp := withTempCwd(t)
	saveConfigState(t)

	// 自定义模型目录必须真实存在，loadConfig 才会接受（见 loadConfig 校验）。
	modelsDirPath := filepath.Join(tmp, "custom-models")
	if err := os.MkdirAll(modelsDirPath, 0755); err != nil {
		t.Fatal(err)
	}

	// 写入
	modelConfigsMu.Lock()
	cachedModelConfigs = map[string]ModelConfig{
		"qwen": {Threads: 8, GPULayers: "99", CtxSize: 8192, FlashAttn: true},
	}
	modelConfigsMu.Unlock()
	serverConfigMu.Lock()
	cachedServerConfig = ServerConfig{Host: "0.0.0.0", Port: 9000, MaxModels: 2, CacheRAM: 4096}
	serverConfigMu.Unlock()
	configMu.Lock()
	currentTheme = "light"
	configMu.Unlock()
	customLlamaCppMu.Lock()
	customLlamaCppDir = "D:\\llama-cpp"
	customLlamaCppMu.Unlock()
	modelsDirMu.Lock()
	customModelsDir = modelsDirPath
	modelsDirMu.Unlock()
	saveConfig()

	if _, err := os.Stat(configFile); err != nil {
		t.Fatalf("saveConfig 未生成配置文件: %v", err)
	}

	// 清空全局，模拟全新启动
	modelConfigsMu.Lock()
	cachedModelConfigs = map[string]ModelConfig{}
	modelConfigsMu.Unlock()
	serverConfigMu.Lock()
	cachedServerConfig = ServerConfig{}
	serverConfigMu.Unlock()
	configMu.Lock()
	currentTheme = ""
	configMu.Unlock()
	customLlamaCppMu.Lock()
	customLlamaCppDir = ""
	customLlamaCppMu.Unlock()
	modelsDirMu.Lock()
	customModelsDir = ""
	modelsDirMu.Unlock()

	loadConfig()

	modelConfigsMu.Lock()
	got := cachedModelConfigs["qwen"]
	modelConfigsMu.Unlock()
	if got.Threads != 8 || got.CtxSize != 8192 || !got.FlashAttn {
		t.Errorf("模型配置读回错误: %+v", got)
	}
	serverConfigMu.Lock()
	scfg := cachedServerConfig
	serverConfigMu.Unlock()
	if scfg.Port != 9000 || scfg.Host != "0.0.0.0" || scfg.MaxModels != 2 {
		t.Errorf("服务器配置读回错误: %+v", scfg)
	}
	configMu.Lock()
	if currentTheme != "light" {
		t.Errorf("主题读回错误: %q", currentTheme)
	}
	configMu.Unlock()
	customLlamaCppMu.Lock()
	if customLlamaCppDir != "D:\\llama-cpp" {
		t.Errorf("自定义 llama.cpp 目录读回错误: %q", customLlamaCppDir)
	}
	customLlamaCppMu.Unlock()
	modelsDirMu.Lock()
	if customModelsDir != modelsDirPath {
		t.Errorf("自定义模型目录读回错误: %q, want %q", customModelsDir, modelsDirPath)
	}
	modelsDirMu.Unlock()
}

// TestLoadConfigDefaults 验证缺失/部分配置时用默认值兜底，不因旧数据崩溃。
func TestLoadConfigDefaults(t *testing.T) {
	withTempCwd(t)
	saveConfigState(t)

	// 只有 host 的残缺配置
	if err := os.WriteFile(configFile, []byte(`{"serverConfig":{"host":"127.0.0.1"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	loadConfig()

	serverConfigMu.Lock()
	scfg := cachedServerConfig
	serverConfigMu.Unlock()
	if scfg.Port != 8080 || scfg.MaxModels != 1 || scfg.CacheRAM != 8192 {
		t.Errorf("残缺配置应回退默认端口/模型数/缓存: %+v", scfg)
	}
	configMu.Lock()
	if currentTheme != "light" {
		t.Errorf("无主题时应回退 light, 实际 %q", currentTheme)
	}
	configMu.Unlock()
	modelConfigsMu.Lock()
	if cachedModelConfigs == nil {
		t.Error("无模型配置时不应为 nil map")
	}
	modelConfigsMu.Unlock()
}

// TestLoadConfigMissingFile 验证配置文件不存在时静默返回（首次启动场景）。
func TestLoadConfigMissingFile(t *testing.T) {
	withTempCwd(t)
	saveConfigState(t)
	loadConfig() // 不应 panic
}

// TestLoadConfigMigratesMLockNoMMap 验证旧格式配置中的 mlock/noMmap 字段
// 在 loadConfig 时迁移为 load-mode（b10342 起 mlock/no-mmap DEPRECATED），
// 兼容字段随即清零，避免旧布尔值被写入新格式配置。
func TestLoadConfigMigratesMLockNoMMap(t *testing.T) {
	withTempCwd(t)
	saveConfigState(t)

	cases := []struct {
		name       string
		configJSON string
		want       string
	}{
		{"mlock only", `{"modelConfigs":{"m1":{"threads":4,"mlock":true,"noMmap":false}}}`, "mlock"},
		{"mlock and noMmap", `{"modelConfigs":{"m1":{"threads":4,"mlock":true,"noMmap":true}}}`, "mlock"}, // mlock 语义优先
		{"noMmap only", `{"modelConfigs":{"m1":{"threads":4,"mlock":false,"noMmap":true}}}`, "none"},
		{"neither", `{"modelConfigs":{"m1":{"threads":4,"mlock":false,"noMmap":false}}}`, ""}, // 无旧字段保持默认
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := os.WriteFile(configFile, []byte(c.configJSON), 0644); err != nil {
				t.Fatal(err)
			}
			loadConfig()
			modelConfigsMu.Lock()
			got := cachedModelConfigs["m1"]
			modelConfigsMu.Unlock()
			if got.LoadMode != c.want {
				t.Errorf("LoadMode = %q, want %q（config %s）", got.LoadMode, c.want, c.configJSON)
			}
			if got.MLock || got.NoMMap {
				t.Errorf("迁移后兼容字段应清零, 实际 MLock=%v NoMMap=%v", got.MLock, got.NoMMap)
			}
		})
	}

	// 新格式配置（已含 loadMode）不应被迁移覆盖
	writeConfig := `{"modelConfigs":{"m1":{"threads":4,"loadMode":"dio"}}}`
	if err := os.WriteFile(configFile, []byte(writeConfig), 0644); err != nil {
		t.Fatal(err)
	}
	loadConfig()
	modelConfigsMu.Lock()
	got := cachedModelConfigs["m1"]
	modelConfigsMu.Unlock()
	if got.LoadMode != "dio" {
		t.Errorf("新格式 loadMode=dio 应保持不变, 实际 %q", got.LoadMode)
	}
}

// TestSaveServerConfigRejectsNonLoopbackHost 验证 SaveServerConfig 拒绝
// 非环回 Host（#5）。若允许 0.0.0.0 等地址，llama-server 会把推理服务
// 暴露到局域网/公网，须在存配置前拒绝。
func TestSaveServerConfigRejectsNonLoopbackHost(t *testing.T) {
	withTempCwd(t)
	saveConfigState(t)

	app := &App{}
	if err := app.SaveServerConfig(ServerConfig{Host: "0.0.0.0", Port: 8080, MaxModels: 1, CacheRAM: 0}); err == nil {
		t.Error("Host=0.0.0.0 应返回错误")
	}
	if err := app.SaveServerConfig(ServerConfig{Host: "192.168.1.10", Port: 8080, MaxModels: 1, CacheRAM: 0}); err == nil {
		t.Error("局域网地址应返回错误")
	}
	// 拒绝分支不得改动已存配置
	serverConfigMu.Lock()
	got := cachedServerConfig
	serverConfigMu.Unlock()
	if got.Host != "127.0.0.1" {
		t.Errorf("非法 Host 不应改写配置, 当前 Host = %q", got.Host)
	}
}

// TestSaveServerConfigRejectsInvalidPort 验证端口必须落在 1024-65535，
// 避开特权端口与非法范围（#5）。
func TestSaveServerConfigRejectsInvalidPort(t *testing.T) {
	withTempCwd(t)
	saveConfigState(t)

	app := &App{}
	if err := app.SaveServerConfig(ServerConfig{Host: "127.0.0.1", Port: 80, MaxModels: 1, CacheRAM: 0}); err == nil {
		t.Error("Port=80（特权端口）应返回错误")
	}
	if err := app.SaveServerConfig(ServerConfig{Host: "127.0.0.1", Port: 99999, MaxModels: 1, CacheRAM: 0}); err == nil {
		t.Error("Port=99999 超出范围应返回错误")
	}
}

// TestSaveServerConfigRejectsInvalidNumbers 验证 MaxModels 至少为 1、
// CacheRAM 非负（#5）。
func TestSaveServerConfigRejectsInvalidNumbers(t *testing.T) {
	withTempCwd(t)
	saveConfigState(t)

	app := &App{}
	if err := app.SaveServerConfig(ServerConfig{Host: "127.0.0.1", Port: 8080, MaxModels: 0, CacheRAM: 0}); err == nil {
		t.Error("MaxModels=0 应返回错误")
	}
	if err := app.SaveServerConfig(ServerConfig{Host: "127.0.0.1", Port: 8080, MaxModels: 1, CacheRAM: -1}); err == nil {
		t.Error("CacheRAM 为负应返回错误")
	}
}

// TestSaveServerConfigAcceptsLoopback 验证合法环回配置被接受并写入
// 全局缓存与配置文件（#5 对照组：localhost / ::1 均合法）。
func TestSaveServerConfigAcceptsLoopback(t *testing.T) {
	withTempCwd(t)
	saveConfigState(t)

	app := &App{}
	for _, host := range []string{"127.0.0.1", "localhost", "::1"} {
		if err := app.SaveServerConfig(ServerConfig{Host: host, Port: 8080, MaxModels: 2, CacheRAM: 4096}); err != nil {
			t.Errorf("Host=%q 应被接受: %v", host, err)
		}
		serverConfigMu.Lock()
		got := cachedServerConfig
		serverConfigMu.Unlock()
		if got.Host != host || got.Port != 8080 || got.MaxModels != 2 || got.CacheRAM != 4096 {
			t.Errorf("Host=%q 配置未正确写入: %+v", host, got)
		}
	}
}

// TestSaveModelConfigRejectsInjection 验证 SaveModelConfig 拒绝含换行的
// 字符串字段（#9）。这类值若进入预设生成会被原样写入 INI，构成配置注入。
// 拒绝分支不得写入配置缓存；合法值则正常写入（对照组）。
func TestSaveModelConfigRejectsInjection(t *testing.T) {
	withTempCwd(t)
	saveConfigState(t)

	app := &App{}
	badGPU := "99\n[evil]\nmodel=/tmp/x"
	if err := app.SaveModelConfig("m1", ModelConfig{GPULayers: badGPU}); err == nil {
		t.Error("含换行的 GPULayers 应返回错误")
	}
	if err := app.SaveModelConfig("m1", ModelConfig{CacheTypeK: "q8_0\nfoo"}); err == nil {
		t.Error("含换行的 CacheTypeK 应返回错误")
	}
	// 拒绝分支不得写入配置缓存
	modelConfigsMu.Lock()
	_, ok := cachedModelConfigs["m1"]
	modelConfigsMu.Unlock()
	if ok {
		t.Error("被拒绝的配置不应写入缓存")
	}

	// 合法 CacheTypeK 应被接受并写入
	if err := app.SaveModelConfig("m1-ok", ModelConfig{CacheTypeK: "q4_0"}); err != nil {
		t.Errorf("合法 CacheTypeK 应被接受: %v", err)
	}
	modelConfigsMu.Lock()
	got, ok := cachedModelConfigs["m1-ok"]
	modelConfigsMu.Unlock()
	if !ok || got.CacheTypeK != "q4_0" {
		t.Error("合法配置未写入缓存")
	}
}

// TestSaveModelConfigRejectsInvalidWhitelist 验证 GPULayers / CacheType
// 白名单之外的值被拒绝（#9 第一层）。
func TestSaveModelConfigRejectsInvalidWhitelist(t *testing.T) {
	withTempCwd(t)
	saveConfigState(t)

	app := &App{}
	if err := app.SaveModelConfig("m2", ModelConfig{GPULayers: "-1"}); err == nil {
		t.Error("GPULayers=-1 应返回错误")
	}
	if err := app.SaveModelConfig("m2", ModelConfig{GPULayers: "1.5"}); err == nil {
		t.Error("GPULayers=1.5 应返回错误")
	}
	if err := app.SaveModelConfig("m2", ModelConfig{CacheTypeV: "q4_2"}); err == nil {
		t.Error("CacheTypeV=q4_2 不在白名单应返回错误")
	}
}

// TestEffectiveModelsDir 验证生效模型目录：默认返回 LLM-Models；配置了
// 自定义目录后返回自定义目录。
func TestEffectiveModelsDir(t *testing.T) {
	saveConfigState(t)

	modelsDirMu.Lock()
	customModelsDir = ""
	modelsDirMu.Unlock()
	if got := effectiveModelsDir(); got != modelsDir {
		t.Errorf("默认生效目录 = %q, want %q", got, modelsDir)
	}

	custom := t.TempDir()
	modelsDirMu.Lock()
	customModelsDir = custom
	modelsDirMu.Unlock()
	if got := effectiveModelsDir(); got != custom {
		t.Errorf("设置自定义目录后生效目录 = %q, want %q", got, custom)
	}
}

// TestLoadConfigIgnoresInvalidModelDir 验证配置文件中 modelDir 指向不存在的
// 目录或普通文件时，loadConfig 忽略该值（打 WARN 并回退默认），customModelsDir
// 保持为空。
func TestLoadConfigIgnoresInvalidModelDir(t *testing.T) {
	withTempCwd(t)
	saveConfigState(t)
	modelsDirMu.Lock()
	customModelsDir = ""
	modelsDirMu.Unlock()

	// writeConfig 用 json.Marshal 编码路径，避免 Windows 反斜杠破坏 JSON 转义。
	writeConfig := func(modelDir string) {
		t.Helper()
		cfg := appConfig{ModelDir: modelDir}
		data, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(configFile, data, 0644); err != nil {
			t.Fatal(err)
		}
	}

	// modelDir 指向不存在的目录
	writeConfig(filepath.Join(t.TempDir(), "does-not-exist"))
	loadConfig()
	modelsDirMu.Lock()
	if customModelsDir != "" {
		t.Errorf("不存在的 modelDir 应被忽略, 实际 customModelsDir = %q", customModelsDir)
	}
	modelsDirMu.Unlock()

	// modelDir 指向普通文件（非目录）
	filePath := filepath.Join(t.TempDir(), "plain-file")
	if err := os.WriteFile(filePath, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	writeConfig(filePath)
	loadConfig()
	modelsDirMu.Lock()
	if customModelsDir != "" {
		t.Errorf("普通文件的 modelDir 应被忽略, 实际 customModelsDir = %q", customModelsDir)
	}
	modelsDirMu.Unlock()
}

// TestSetModelsDir 验证 SetModelsDir：非法输入（空串/不存在路径/普通文件）
// 返回错误且不改写 customModelsDir；合法目录写入成功并使模型缓存失效。
func TestSetModelsDir(t *testing.T) {
	withTempCwd(t)
	saveConfigState(t)
	saveModelsState(t)
	app := &App{}

	// 先置缓存有效，便于断言 SetModelsDir 使缓存失效
	modelsMu.Lock()
	modelsCacheValid.Store(true)
	modelsMu.Unlock()

	// 空串
	if err := app.SetModelsDir(""); err == nil {
		t.Error("空串应返回错误")
	}
	// 不存在的路径
	if err := app.SetModelsDir(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("不存在的路径应返回错误")
	}
	// 普通文件
	filePath := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := app.SetModelsDir(filePath); err == nil {
		t.Error("普通文件应返回错误")
	}
	// 非法输入不得改写状态
	modelsDirMu.Lock()
	if customModelsDir != "" {
		t.Errorf("非法输入不应改写 customModelsDir, 实际 %q", customModelsDir)
	}
	modelsDirMu.Unlock()

	// 合法目录
	valid := t.TempDir()
	if err := app.SetModelsDir(valid); err != nil {
		t.Fatalf("合法目录应写入成功: %v", err)
	}
	modelsDirMu.Lock()
	if customModelsDir != valid {
		t.Errorf("customModelsDir = %q, want %q", customModelsDir, valid)
	}
	modelsDirMu.Unlock()
	if modelsCacheValid.Load() {
		t.Error("SetModelsDir 成功后模型缓存应失效")
	}
}

// ─── downloadSource 持久化 ────────────────────────────────────────

// TestLoadConfigDownloadSourceDefault 验证旧配置没有 downloadSource 字段时
// 下载源兜底为 hf（#12：旧数据兼容，不因缺字段报错或留下空值）。
func TestLoadConfigDownloadSourceDefault(t *testing.T) {
	withTempCwd(t)
	saveConfigState(t)

	if err := os.WriteFile(configFile, []byte(`{"serverConfig":{"host":"127.0.0.1"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	loadConfig()

	if got := activeDownloadSource(); got != sourceHF {
		t.Errorf("旧配置无 downloadSource 字段时应兜底 hf, 实际 %q", got)
	}
}

// TestSetDownloadSourcePersist 验证 SetDownloadSource 合法值写入并持久化往返：
// 设置 modelscope 后 activeDownloadSource 立即生效，saveConfig + loadConfig 后
// 仍为 modelscope（含非默认值在重启后恢复）。
func TestSetDownloadSourcePersist(t *testing.T) {
	withTempCwd(t)
	saveConfigState(t)

	app := &App{}
	if err := app.SetDownloadSource(sourceModelScope); err != nil {
		t.Fatal(err)
	}
	if got := activeDownloadSource(); got != sourceModelScope {
		t.Errorf("设置后 activeDownloadSource = %q, want modelscope", got)
	}

	// 重启模拟：读回配置文件
	downloadSourceMu.Lock()
	downloadSource = sourceHF
	downloadSourceMu.Unlock()
	loadConfig()
	if got := activeDownloadSource(); got != sourceModelScope {
		t.Errorf("持久化往返后 activeDownloadSource = %q, want modelscope", got)
	}
}

// TestSetDownloadSourceRejectsInvalid 验证 SetDownloadSource 拒绝白名单外的值
// 且不改写当前状态（非法值返回中文错误）。
func TestSetDownloadSourceRejectsInvalid(t *testing.T) {
	withTempCwd(t)
	saveConfigState(t)

	downloadSourceMu.Lock()
	downloadSource = sourceHF
	downloadSourceMu.Unlock()

	app := &App{}
	if err := app.SetDownloadSource("github"); err == nil {
		t.Error("非法下载源 github 应返回错误")
	}
	if err := app.SetDownloadSource(""); err == nil {
		t.Error("空下载源应返回错误")
	}
	if got := activeDownloadSource(); got != sourceHF {
		t.Errorf("非法值不应改写下载源, 实际 %q", got)
	}
}

// ─── 下载任务队列持久化 ───────────────────────────────────────────

// TestDownloadTasksPersistRoundTrip 验证下载任务队列 saveConfig/loadConfig 往返：
// 含终态任务（done）在内全部字段（ID/ModelID/FileName/DestDir/Source/Status/
// Progress/Total/Downloaded/SizeHuman/Error）保持一致，运行期字段（URL/ctx/
// cancel/resumeCh）不持久化、URL 在恢复时重建。
func TestDownloadTasksPersistRoundTrip(t *testing.T) {
	withTempCwd(t)
	saveConfigState(t)

	dlTasksMu.Lock()
	dlTasks = []*DlTask{
		{ID: "dl-1", ModelID: "author/model", FileName: "a.gguf", DestDir: "D:/models/author/model", Source: "hf", Status: "done", Progress: 100, Total: 100, Downloaded: 100, SizeHuman: "100 B"},
		{ID: "dl-2", ModelID: "author/model2", FileName: "b.gguf", DestDir: "D:/models/author/model2", Source: "modelscope", Status: "paused", Progress: 50, Total: 200, Downloaded: 100, SizeHuman: "200 B", Error: "曾失败"},
	}
	dlTasksMu.Unlock()

	saveConfig()

	// 清空全局，模拟全新启动
	dlTasksMu.Lock()
	dlTasks = nil
	dlTaskCounter = 0
	dlTasksMu.Unlock()

	loadConfig()

	dlTasksMu.Lock()
	restored := dlTasks
	dlTasksMu.Unlock()
	if len(restored) != 2 {
		t.Fatalf("恢复任务数 = %d, want 2", len(restored))
	}
	got := restored[0]
	if got.ID != "dl-1" || got.ModelID != "author/model" || got.FileName != "a.gguf" ||
		got.DestDir != "D:/models/author/model" || got.Source != "hf" || got.Status != "done" ||
		got.Progress != 100 || got.Total != 100 || got.Downloaded != 100 || got.SizeHuman != "100 B" {
		t.Errorf("done 任务字段读回不一致: %+v", got)
	}
	got2 := restored[1]
	if got2.Source != "modelscope" || got2.Status != "paused" || got2.Error != "曾失败" || got2.Downloaded != 100 {
		t.Errorf("paused 任务字段读回不一致: %+v", got2)
	}
	// URL 恢复时按 Source 重建（不持久化原始 URL）
	wantURL, err := buildModelDownloadURL("hf", "author/model", "a.gguf")
	if err != nil {
		t.Fatal(err)
	}
	if restored[0].URL != wantURL {
		t.Errorf("URL 应按 source 重建, got %q", restored[0].URL)
	}
}

// TestLoadConfigRestoresDownloadTasks 验证恢复规范化（#12）：
//   - downloading 状态（进程退出后 goroutine 已消亡）规整为 paused；
//   - 非法/空 status 规整为 paused；
//   - Source 为空兜底 hf；
//   - URL 按 source 重建正确。
func TestLoadConfigRestoresDownloadTasks(t *testing.T) {
	withTempCwd(t)
	saveConfigState(t)

	dlTasksMu.Lock()
	dlTasks = nil
	dlTaskCounter = 0
	dlTasksMu.Unlock()

	configJSON := `{
		"downloadTasks": [
			{"id":"dl-1","modelId":"author/model","fileName":"a.gguf","destDir":"D:/m/author/model","source":"hf","status":"downloading","progress":50,"total":100,"downloaded":50,"sizeHuman":"100 B"},
			{"id":"dl-2","modelId":"author/model","fileName":"b.gguf","destDir":"D:/m/author/model","source":"","status":"weird","progress":0},
			{"id":"dl-3","modelId":"author/model","fileName":"c.gguf","destDir":"D:/m/author/model","source":"modelscope","status":"queued","progress":0}
		]
	}`
	if err := os.WriteFile(configFile, []byte(configJSON), 0644); err != nil {
		t.Fatal(err)
	}
	loadConfig()

	dlTasksMu.Lock()
	defer dlTasksMu.Unlock()
	if len(dlTasks) != 3 {
		t.Fatalf("恢复任务数 = %d, want 3", len(dlTasks))
	}
	// downloading → paused
	if dlTasks[0].Status != "paused" {
		t.Errorf("dl-1 downloading 应规整为 paused, 实际 %q", dlTasks[0].Status)
	}
	if dlTasks[0].Source != "hf" {
		t.Errorf("dl-1 Source = %q, want hf", dlTasks[0].Source)
	}
	wantURL := hfMirrorBase + "/author/model/resolve/main/a.gguf"
	if dlTasks[0].URL != wantURL {
		t.Errorf("dl-1 URL 重建 = %q, want %q", dlTasks[0].URL, wantURL)
	}
	// 非法 status + 空 source → paused / hf
	if dlTasks[1].Status != "paused" {
		t.Errorf("dl-2 非法 status 应规整为 paused, 实际 %q", dlTasks[1].Status)
	}
	if dlTasks[1].Source != sourceHF {
		t.Errorf("dl-2 空 Source 应兜底 hf, 实际 %q", dlTasks[1].Source)
	}
	// queued 保持原样，modelscope URL 用默认 Legacy Base 重建
	if dlTasks[2].Status != "queued" {
		t.Errorf("dl-3 queued 应保持原样, 实际 %q", dlTasks[2].Status)
	}
	if !strings.HasPrefix(dlTasks[2].URL, "https://modelscope.cn/api/v1/models/") {
		t.Errorf("dl-3 modelscope URL 重建前缀错误: %q", dlTasks[2].URL)
	}
}

// TestDownloadTaskCounterNoConflict 验证恢复任务后 dlTaskCounter 与既有任务不冲突：
// 配置文件含 dl-3 任务，loadConfig 后 counter 至少为 3；再经 startHFDownload 入队
// 新任务，新任务 id 唯一（dl-4）且不覆盖既有任务。
func TestDownloadTaskCounterNoConflict(t *testing.T) {
	withTempCwd(t)
	saveConfigState(t)
	restoreSource := withModelScope404Server(t)
	defer restoreSource()

	configJSON := `{"downloadTasks":[{"id":"dl-3","modelId":"author/model","fileName":"x.gguf","destDir":"D:/m","source":"hf","status":"paused"}]}`
	if err := os.WriteFile(configFile, []byte(configJSON), 0644); err != nil {
		t.Fatal(err)
	}
	dlTasksMu.Lock()
	dlTasks = nil
	dlTaskCounter = 0
	dlTasksMu.Unlock()
	loadConfig()

	dlTasksMu.Lock()
	restoredCounter := dlTaskCounter
	dlTasksMu.Unlock()
	if restoredCounter < 3 {
		t.Errorf("恢复后 dlTaskCounter = %d, want >= 3（不得小于已恢复任务最大序号）", restoredCounter)
	}

	// 经真实入队路径新增任务：id 应唯一（dl-4）
	if err := startHFDownload("author/model", []string{"new.gguf"}); err != nil {
		t.Fatal(err)
	}
	dlTasksMu.Lock()
	var ids []string
	for _, tt := range dlTasks {
		ids = append(ids, tt.ID)
	}
	dlTasksMu.Unlock()
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Errorf("任务 id 重复: %v", ids)
		}
		seen[id] = true
	}
	if !seen["dl-4"] {
		t.Errorf("新入队任务 id 应为 dl-4（恢复 dl-3 后 counter=3）, 实际 ids=%v", ids)
	}
	if len(ids) != 2 {
		t.Errorf("任务总数 = %d, want 2（dl-3 恢复 + dl-4 新入队）: %v", len(ids), ids)
	}

	// 等待新任务 goroutine 进入终态（404 快速失败），避免残留；恢复的 dl-3 为
	// paused 无 goroutine，不参与等待。
	waitTaskTerminal(t, "dl-4", 5*time.Second)
}
