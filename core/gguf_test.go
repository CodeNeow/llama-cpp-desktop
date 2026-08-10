package core

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// ggufKV 是测试用 GGUF 元数据键值对：valueType 为 GGUF value 类型编号，
// raw 为该类型的底层字节（由 helper 序列化）。
type ggufKV struct {
	key       string
	valueType uint32
	raw       []byte
}

// putU32 / putU64 以 little-endian 写入 uint，与 readGGUFMeta 的读取方式一致。
func putU32(buf *bytes.Buffer, v uint32) { binary.Write(buf, binary.LittleEndian, v) }
func putU64(buf *bytes.Buffer, v uint64) { binary.Write(buf, binary.LittleEndian, v) }

// strKV 构造一个 GGUF string 类型的 KV（valueType=8）。
func strKV(key, val string) ggufKV {
	var buf bytes.Buffer
	putU64(&buf, uint64(len(val)))
	buf.WriteString(val)
	return ggufKV{key: key, valueType: 8, raw: buf.Bytes()}
}

// u32KV 构造一个 GGUF uint32 类型的 KV（valueType=4）。
func u32KV(key string, val uint32) ggufKV {
	var buf bytes.Buffer
	putU32(&buf, val)
	return ggufKV{key: key, valueType: 4, raw: buf.Bytes()}
}

// u64KV 构造一个 GGUF uint64 类型的 KV（valueType=10）。
func u64KV(key string, val uint64) ggufKV {
	var buf bytes.Buffer
	putU64(&buf, val)
	return ggufKV{key: key, valueType: 10, raw: buf.Bytes()}
}

// boolKV 构造一个 GGUF bool 类型的 KV（valueType=7），用于覆盖跳过分支。
func boolKV(key string, val byte) ggufKV {
	return ggufKV{key: key, valueType: 7, raw: []byte{val}}
}

// buildGGUF 按 GGUF 文件头格式拼接字节：magic/version/tensorCount/kvCount + KV 序列。
// 文件头四字节为 "GGUF"，以 little-endian 读取时为 0x46554747（GGUF 魔数）。
func buildGGUF(version uint32, kvs ...ggufKV) []byte {
	var buf bytes.Buffer
	buf.WriteString("GGUF")
	putU32(&buf, version)
	putU64(&buf, 0) // tensorCount
	putU64(&buf, uint64(len(kvs)))
	for _, kv := range kvs {
		putU64(&buf, uint64(len(kv.key)))
		buf.WriteString(kv.key)
		putU32(&buf, kv.valueType)
		buf.Write(kv.raw)
	}
	return buf.Bytes()
}

// writeTempGGUF 把 GGUF 字节写入临时目录下的指定文件名，返回完整路径。
func writeTempGGUF(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("写入测试 GGUF 失败: %v", err)
	}
	return path
}

// TestReadGGUFMeta 验证 readGGUFMeta 从合法 GGUF 文件头提取
// name/arch/quant 三个目标字段，并跳过不感兴趣的 KV。
func TestReadGGUFMeta(t *testing.T) {
	dir := t.TempDir()
	path := writeTempGGUF(t, dir, "model.gguf", buildGGUF(3,
		strKV("general.name", "Qwen2.5-7B-Instruct"),
		u32KV("general.size", 12345), // 未知 key，验证跳过
		boolKV("general.foo", 1),     // bool 类型，验证跳过
		strKV("general.architecture", "Qwen"),
		u32KV("general.file_type", 15), // 15 → Q4_K_M
	))

	meta := readGGUFMeta(path)
	if meta == nil {
		t.Fatal("readGGUFMeta 返回 nil，期望解析成功")
	}
	if meta["name"] != "Qwen2.5-7B-Instruct" {
		t.Errorf("name = %q, want Qwen2.5-7B-Instruct", meta["name"])
	}
	if meta["arch"] != "Qwen" {
		t.Errorf("arch = %q, want Qwen", meta["arch"])
	}
	if meta["quant"] != "Q4_K_M" {
		t.Errorf("quant = %q, want Q4_K_M", meta["quant"])
	}
}

// TestReadGGUFMetaUint64Quant 验证 file_type 以 uint64（类型 10）编码时同样能解析。
func TestReadGGUFMetaUint64Quant(t *testing.T) {
	dir := t.TempDir()
	path := writeTempGGUF(t, dir, "model.gguf", buildGGUF(2,
		strKV("general.name", "bge-small-zh"),
		u64KV("general.file_type", 8), // 8 → Q8_0
	))

	meta := readGGUFMeta(path)
	if meta == nil {
		t.Fatal("readGGUFMeta 返回 nil")
	}
	if meta["quant"] != "Q8_0" {
		t.Errorf("quant = %q, want Q8_0", meta["quant"])
	}
}

// TestReadGGUFMetaInvalidMagic 验证非 GGUF 文件返回 nil（不 panic）。
func TestReadGGUFMetaInvalidMagic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.gguf")
	if err := os.WriteFile(path, []byte("NOTGUF DATA"), 0644); err != nil {
		t.Fatal(err)
	}
	if meta := readGGUFMeta(path); meta != nil {
		t.Error("非法魔数应返回 nil")
	}
}

// TestReadGGUFMetaUnsupportedVersion 验证不受支持的 GGUF 版本返回 nil。
func TestReadGGUFMetaUnsupportedVersion(t *testing.T) {
	dir := t.TempDir()
	path := writeTempGGUF(t, dir, "v1.gguf", buildGGUF(1, strKV("general.name", "old")))
	if meta := readGGUFMeta(path); meta != nil {
		t.Error("版本 1 的 GGUF 应返回 nil")
	}
}

// TestReadGGUFMetaMissingFile 验证文件不存在时返回 nil。
func TestReadGGUFMetaMissingFile(t *testing.T) {
	if meta := readGGUFMeta(filepath.Join(t.TempDir(), "nope.gguf")); meta != nil {
		t.Error("不存在的文件应返回 nil")
	}
}

// ─── ggufQuantName ───────────────────────────────────────────────

// TestGGUFQuantName 验证 GGUF file_type 编号到量化名的映射与未知值回退。
func TestGGUFQuantName(t *testing.T) {
	cases := []struct {
		in   uint32
		want string
	}{
		{0, "F32"},
		{1, "F16"},
		{15, "Q4_K_M"},
		{26, "IQ4_NL"},
		{999, "Q999"},
	}
	for _, c := range cases {
		if got := ggufQuantName(c.in); got != c.want {
			t.Errorf("ggufQuantName(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestReadGGUFStringTooLong 验证超过 1MB 的长度上限被拒绝并返回错误。
func TestReadGGUFStringTooLong(t *testing.T) {
	var buf bytes.Buffer
	putU64(&buf, 1024*1024+1)
	if _, err := readGGUFString(&buf); err == nil {
		t.Error("超长字符串应返回错误")
	}
}

// TestReadGGUFMetaRejectsHugeKVCount 验证 kvCount 超过 4096 上限的
// 恶意/损坏 GGUF 文件被直接拒绝（#7.2）：readGGUFMeta 返回 nil 且不
// panic。此前 kvCount 来自文件无上限，可放大解析开销；修复后在循环前
// 判断并放弃解析。
func TestReadGGUFMetaRejectsHugeKVCount(t *testing.T) {
	dir := t.TempDir()

	// 手工写 GGUF 文件头：magic + version 3 + tensorCount 0 + kvCount 5000，
	// 不写任何 KV 键值（解析应在读 KV 前终止）。
	var buf bytes.Buffer
	buf.WriteString("GGUF")
	putU32(&buf, 3)
	putU64(&buf, 0)    // tensorCount
	putU64(&buf, 5000) // kvCount 超上限
	path := writeTempGGUF(t, dir, "huge.gguf", buf.Bytes())

	if meta := readGGUFMeta(path); meta != nil {
		t.Errorf("kvCount 超上限应返回 nil, 实际 %v", meta)
	}
}
