package cloud115

import (
	"crypto/sha1"
	"encoding/hex"
	"io"
	"os"
)

// FileSHA1 计算文件完整 SHA1（小写 hex）。
func FileSHA1(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// FileSHA1Partial 计算文件 [start,end]（含）字节区间的 SHA1（小写 hex）。
// 用于 115 上传二次签名按 sign_check 指定的区间重算哈希。
func FileSHA1Partial(path string, start, end int64) (string, error) {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return "", err
	}
	length := end - start + 1
	h := sha1.New()
	if _, err := io.CopyN(h, f, length); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
