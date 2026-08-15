package core

import (
	"strings"
	"testing"
)

// TestRunCmdTrimsOutput 验证 runCmd 能捕获子进程 stdout 并去除首尾空白。
// 使用 `go env GOHOSTOS`：跨平台可用，无需外部二进制与网络。
func TestRunCmdTrimsOutput(t *testing.T) {
	out := runCmd("go", "env", "GOHOSTOS")
	if strings.TrimSpace(out) != out {
		t.Fatalf("runCmd 返回未 trim 的输出: %q", out)
	}
	if out == "" {
		t.Fatal("runCmd(go env GOHOSTOS) 返回空输出")
	}
}

// TestRunCmdMissingBinary 验证命令不存在时 runCmd 返回空字符串而非 panic。
func TestRunCmdMissingBinary(t *testing.T) {
	out := runCmd("llama-desktop-no-such-binary-xyz", "--version")
	if out != "" {
		t.Fatalf("不存在的命令应返回空输出，实际: %q", out)
	}
}
