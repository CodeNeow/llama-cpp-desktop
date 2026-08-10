package core

import (
	"os"
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
func saveConfigState(t *testing.T) (origModels map[string]ModelConfig, origServer ServerConfig, origTheme string, origDir string) {
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
	})
	return
}

// TestSaveLoadConfigRoundTrip 验证 saveConfig 写入、loadConfig 读回的一致性。
func TestSaveLoadConfigRoundTrip(t *testing.T) {
	withTempCwd(t)
	saveConfigState(t)

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
	if currentTheme != "dark" {
		t.Errorf("无主题时应回退 dark, 实际 %q", currentTheme)
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
