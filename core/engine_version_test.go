package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// probeHelperStderrSrc 探针测试用正常 stub：仅把版本行写入 stderr 后退出
// （llama-server 的 --version 输出全部走 stderr，stdout 为空），退出码 0。
const probeHelperStderrSrc = `package main

import "os"

func main() {
	os.Stderr.WriteString("version: b1234 (build 1234)\n")
}
`

// probeHelperHangSrc 探针测试用挂起 stub：无限睡眠，模拟把 -v 误当版本标志
// 启动完整 HTTP 服务器后永不退出的异常二进制。
const probeHelperHangSrc = `package main

import "time"

func main() {
	time.Sleep(60 * time.Second)
}
`

// buildProbeHelper 编译探针测试用 stub 可执行文件：src 为独立 main 包源码，
// 编译产物放 t.TempDir()，测试结束自动清理。只有真实启动子进程才能覆盖
// probeLlamaVersion 默认实现中「合并 stdout+stderr」与「超时 kill 子进程」
// 两条路径，仅替换 probeLlamaVersion 变量无法触达这两个默认实现分支。
func buildProbeHelper(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(srcFile, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(dir, "probe-helper")
	if runtime.GOOS == "windows" {
		// go build 在 Windows 上产出 probe-helper.exe，探针按该路径
		// 直接 exec，必须带上扩展名才能被 CreateProcess 找到
		helper += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", helper, srcFile).CombinedOutput(); err != nil {
		t.Fatalf("go build 探针 helper 失败: %v\n%s", err, out)
	}
	return helper
}

// TestFillLlamaCppVersionReadsVersionFromStderr 验证版本探测合并了 stderr：
// llama-server 的 --version 输出全部走 stderr（stdout 为空），此前 runCmd
// 只捕获 stdout 导致版本号丢失、主页一直显示"未找到"。用真实可执行 stub
// （仅写 stderr 并退出）走 fillLlamaCppVersion 全链路，断言解析出的版本行
// 为 "version: b1234 (build 1234)"。旧实现（stdout 为空 → 回退 -v 无限挂起）
// 在该场景下会永久阻塞，本用例即为该根因的回归保护。
func TestFillLlamaCppVersionReadsVersionFromStderr(t *testing.T) {
	helper := buildProbeHelper(t, probeHelperStderrSrc)

	info := LlamaCppInfo{}
	fillLlamaCppVersion(&info, helper)

	want := "version: b1234 (build 1234)"
	if info.Version != want {
		t.Errorf("Version = %q, want 从 stderr 合并提取的版本行 %q", info.Version, want)
	}
}

// TestFillLlamaCppVersionProbesVersionOnce 验证版本探测只调用一次、不再回退
// `-v`：注入记录型 probeLlamaVersion（与 githubReleasesAPI / renameFile /
// updateRepoAPI 同风格的包级 var），模拟 --version 无输出的最坏场景，断言
// fillLlamaCppVersion 恰好触发一次探测且 Version 保持为空、不影响 Installed。
// 旧实现会在 --version 输出为空时二次执行 `-v`（新版 llama-server 的 -v 会
// 启动完整 HTTP 服务器并无限运行，getLlamaCppInfo 永不返回），本用例为该
// 回归的防护。
func TestFillLlamaCppVersionProbesVersionOnce(t *testing.T) {
	origProbe := probeLlamaVersion
	var calls int
	probeLlamaVersion = func(_ string) string {
		calls++
		return ""
	}
	defer func() { probeLlamaVersion = origProbe }()

	info := LlamaCppInfo{}
	fillLlamaCppVersion(&info, "/fake/llama-server")

	if calls != 1 {
		t.Fatalf("探测调用次数 = %d, want 1（--version 无输出时不得回退 -v 二次探测）", calls)
	}
	if info.Version != "" {
		t.Errorf("--version 无输出时 Version = %q, want 空", info.Version)
	}
	if info.Installed {
		t.Error("版本探测失败不应影响 Installed 判定，此处应保持 false")
	}
}

// TestProbeLlamaVersionTimeoutReturnsEmpty 验证超时保护：注入无限挂起的 stub
// （模拟异常二进制），把 llamaVersionProbeTimeout 临时缩短为 300ms，调用
// probeLlamaVersion 默认实现，断言超时 kill 子进程后返回空串且整体耗时远小于
// 挂起时长——检测链不被任何异常二进制冻结。测试确定性结束：挂起 stub 本身睡
// 60s，若超时保护回归失效，本用例会在 60s 后才失败，直接暴露问题而非无限阻塞。
func TestProbeLlamaVersionTimeoutReturnsEmpty(t *testing.T) {
	helper := buildProbeHelper(t, probeHelperHangSrc)

	origTimeout := llamaVersionProbeTimeout
	llamaVersionProbeTimeout = 300 * time.Millisecond
	defer func() { llamaVersionProbeTimeout = origTimeout }()

	start := time.Now()
	out := probeLlamaVersion(helper)
	elapsed := time.Since(start)

	if out != "" {
		t.Errorf("超时 kill 后探测输出 = %q, want 空", out)
	}
	if elapsed >= 10*time.Second {
		t.Errorf("超时探测耗时 = %v, 应被 300ms 超时快速终止（不得无限阻塞）", elapsed)
	}
}
