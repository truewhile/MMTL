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
