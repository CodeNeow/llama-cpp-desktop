package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
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
	if err := app.SaveModelConfig("m2", ModelConfig{CacheTypeV: "q4_1"}); err == nil {
		t.Error("CacheTypeV=q4_1 不在白名单应返回错误")
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
