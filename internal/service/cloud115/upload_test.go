package cloud115

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileSHA1(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum, err := FileSHA1(path)
	if err != nil {
		t.Fatal(err)
	}
	// sha1("hello") = aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d
	if sum != "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d" {
		t.Errorf("unexpected sha1: %s", sum)
	}
}

func TestFileSHA1Partial(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "b.txt")
	// 10 bytes: "0123456789"
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	// bytes [2,4] = "234"
	sum, err := FileSHA1Partial(path, 2, 4)
	if err != nil {
		t.Fatal(err)
	}
	if sum != "0ec09ef9836da03f1add21e3ef607627e687e790" {
		t.Errorf("unexpected partial sha1: %s", sum)
	}
}

// TestFileSHA1PartialSmallerThanWindow 回归测试：经典 bug 是 io.CopyN 在文件不足
// length 字节时返回 io.EOF。115 上传固定用 [0,128*1024-1] 窗口计算 preid，导致所有
// 小于 128 KiB 的元数据文件（如海报/缩略图）上传必然失败。
func TestFileSHA1PartialSmallerThanWindow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.bin")
	// 6 字节小文件，不足 128 KiB 窗口
	if err := os.WriteFile(path, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum, err := FileSHA1Partial(path, 0, 128*1024-1)
	if err != nil {
		t.Fatalf("compute partial sha1 for small file should not fail: %v", err)
	}
	// 应等于整个文件（6 字节）的 sha1
	if sum != "1f8ac10f23c5b5bc1167bda84b833e5c057a77d2" {
		t.Errorf("unexpected partial sha1: %s", sum)
	}
}

func TestParseSignCheckRange(t *testing.T) {
	rng, err := parseSignCheckRange("0-131071")
	if err != nil {
		t.Fatal(err)
	}
	if rng.Start != 0 || rng.End != 131071 {
		t.Errorf("unexpected range: %+v", rng)
	}
	if _, err := parseSignCheckRange("bad"); err == nil {
		t.Error("expected error for bad range")
	}
	if _, err := parseSignCheckRange("100-50"); err == nil {
		t.Error("expected error for end<start")
	}
}

func TestCalculateMultipartPartSize(t *testing.T) {
	// small file: 1 MiB -> partSize 32MiB, 1 part
	ps, parts, err := CalculateMultipartPartSize(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	if ps != defaultMultipartPartSize {
		t.Errorf("partSize=%d, want %d", ps, defaultMultipartPartSize)
	}
	if parts != 1 {
		t.Errorf("parts=%d, want 1", parts)
	}
	// zero-size -> 1 part
	_, parts, err = CalculateMultipartPartSize(0)
	if err != nil {
		t.Fatal(err)
	}
	if parts != 1 {
		t.Errorf("zero-size parts=%d, want 1", parts)
	}
	// negative -> error
	if _, _, err := CalculateMultipartPartSize(-1); err == nil {
		t.Error("expected error for negative size")
	}
}

func TestBaseNameOf(t *testing.T) {
	if got := baseNameOf("/a/b/file.nfo"); got != "file.nfo" {
		t.Errorf("got %s", got)
	}
	if got := baseNameOf("a\\b\\c.jpg"); got != "c.jpg" {
		t.Errorf("got %s", got)
	}
	if got := baseNameOf("top.txt"); got != "top.txt" {
		t.Errorf("got %s", got)
	}
}
