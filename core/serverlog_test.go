package core

import (
	"sync"
	"testing"
)

// ─── serverLogWriter 按行缓冲 ─────────────────────────────────────

// resetServerLogs 清空服务日志环形缓冲（serverLogsMu 保护）并在测试结束后恢复
// 为空，避免 writer 相关用例的日志污染其他用例（与 TestAddServerLogRingBuffer
// 的隔离方式一致）。
func resetServerLogs(t *testing.T) {
	t.Helper()
	serverLogsMu.Lock()
	serverLogs = nil
	serverLogsMu.Unlock()
	t.Cleanup(func() {
		serverLogsMu.Lock()
		serverLogs = nil
		serverLogsMu.Unlock()
	})
}

// serverLogsCopy 在锁内拷贝当前服务日志，供断言使用（不得直接读全局 serverLogs）。
func serverLogsCopy() []string {
	serverLogsMu.Lock()
	defer serverLogsMu.Unlock()
	out := make([]string, len(serverLogs))
	copy(out, serverLogs)
	return out
}

// TestServerLogWriterSingleWriteMultiLine 验证单次 Write 含多行时按行拆分为
// 多条独立日志条目（各行 TrimSpace 去首尾空白），且 Write 返回 len(p), nil
// 满足 io.Writer 契约（对整块输入全部接受）。
func TestServerLogWriterSingleWriteMultiLine(t *testing.T) {
	resetServerLogs(t)
	w := &serverLogWriter{}
	p := []byte("line one\nline two\nline three\n")
	n, err := w.Write(p)
	if n != len(p) || err != nil {
		t.Fatalf("Write 返回 (%d, %v), want (%d, nil)", n, err, len(p))
	}
	got := serverLogsCopy()
	want := []string{"line one", "line two", "line three"}
	if len(got) != len(want) {
		t.Fatalf("日志条目数 = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("条目[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestServerLogWriterSplitWriteReassembles 验证 print_timing 行被多次 Write 拆
// 分（llama-server 输出按小块写入，一行会被拦腰截断）时，缓冲能重组为完整单条。
// 这是本轮修复的核心场景：分片「( 0.63 ms per token, 2362.80 tokens per second)」
// 单独作为条目出现时不再含 "prompt eval time" 标记，parseTPS 会把预填充数字
// 误当作解码速度；按行缓冲必须保证该场景下 addServerLog 收到的是一条完整行。
func TestServerLogWriterSplitWriteReassembles(t *testing.T) {
	resetServerLogs(t)
	w := &serverLogWriter{}

	// 前半句无换行结尾：不得产生任何日志条目，残片留在缓冲
	first := []byte("I slot print_timing:             eval time =     712.56 ms /    64 tokens (   11.13 ms")
	if n, err := w.Write(first); n != len(first) || err != nil {
		t.Fatalf("前半句 Write 返回 (%d, %v), want (%d, nil)", n, err, len(first))
	}
	if got := serverLogsCopy(); len(got) != 0 {
		t.Fatalf("无换行残片不应产生条目, got %v", got)
	}

	// 后半句补全换行：残片与后半句拼成一条完整行
	w.Write([]byte(" per token,    89.82 tokens per second)\n"))
	got := serverLogsCopy()
	if len(got) != 1 {
		t.Fatalf("日志条目数 = %d, want 1: %v", len(got), got)
	}
	want := "I slot print_timing:             eval time =     712.56 ms /    64 tokens (   11.13 ms per token,    89.82 tokens per second)"
	if got[0] != want {
		t.Errorf("重组行 = %q, want %q", got[0], want)
	}
}

// TestServerLogWriterSkipsBlankLines 验证空行与纯空白行（含 \t）被跳过，不产生
// 日志条目，避免环形日志被空白行污染。
func TestServerLogWriterSkipsBlankLines(t *testing.T) {
	resetServerLogs(t)
	w := &serverLogWriter{}
	w.Write([]byte("a\n\n   \n\t\nb\n"))
	got := serverLogsCopy()
	want := []string{"a", "b"}
	if len(got) != len(want) {
		t.Fatalf("日志条目数 = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("条目[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestServerLogWriterTrailingFragmentRetained 验证无换行结尾的尾部残片保留在缓冲
// 中不产生条目，待下一次 Write 补全后拼成一条完整行（不被丢弃、不被提前落库）。
func TestServerLogWriterTrailingFragmentRetained(t *testing.T) {
	resetServerLogs(t)
	w := &serverLogWriter{}
	frag := []byte("tail fragment")
	if n, err := w.Write(frag); n != len(frag) || err != nil {
		t.Fatalf("残片 Write 返回 (%d, %v), want (%d, nil)", n, err, len(frag))
	}
	if got := serverLogsCopy(); len(got) != 0 {
		t.Fatalf("残片不应产生条目, got %v", got)
	}
	w.Write([]byte(" completed\n"))
	got := serverLogsCopy()
	if len(got) != 1 || got[0] != "tail fragment completed" {
		t.Errorf("补全后应有一条完整行, got %v", got)
	}
}

// TestServerLogWriterConcurrentWrite 验证并发 Write 不 panic 且不丢行：50 个
// goroutine 各写一条含 3 行的完整块（每块在锁内原子处理，行不会跨块交错），
// 环形缓冲上限 200（150<200 不裁剪），日志总数应为 150 且每条都是三种预期行之一。
func TestServerLogWriterConcurrentWrite(t *testing.T) {
	resetServerLogs(t)
	w := &serverLogWriter{}
	const goroutines = 50
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := w.Write([]byte("c1 line\nc2 line\nc3 line\n")); err != nil {
				t.Errorf("并发 Write 返回错误: %v", err)
			}
		}()
	}
	wg.Wait()

	got := serverLogsCopy()
	if len(got) != goroutines*3 {
		t.Fatalf("日志条目数 = %d, want %d（不丢行）", len(got), goroutines*3)
	}
	valid := map[string]bool{"c1 line": true, "c2 line": true, "c3 line": true}
	for _, line := range got {
		if !valid[line] {
			t.Errorf("出现非预期行: %q", line)
		}
	}
}
