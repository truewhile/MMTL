package service

import (
	"errors"
	"testing"
)

func TestIs115Blocked(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{errors.New("115 接口返回 HTTP 405：<!doctypehtml>...访问被阻断"), true},
		{errors.New("115 接口返回 HTTP 405"), true},
		{errors.New("115 接口错误（770004）：访问频率过高"), true},
		{errors.New("115 接口错误（406）：达到访问上限"), true},
		{errors.New("下载失败：http 403"), false},
		{errors.New("解析下载地址失败：115 接口调用失败"), false},
		{errors.New("database is locked (517)"), false},
		{nil, false},
	}
	for _, c := range cases {
		if got := is115Blocked(c.err); got != c.want {
			t.Errorf("is115Blocked(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

func TestIsHTTPDownloadFailure(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{errors.New("http 403"), true},
		{errors.New("http 404"), true},
		{errors.New("http 410"), true},
		{errors.New("http 500"), true},
		{errors.New("Get \"https://x\": connection refused"), false},
		{nil, false},
	}
	for _, c := range cases {
		if got := isHTTPDownloadFailure(c.err); got != c.want {
			t.Errorf("isHTTPDownloadFailure(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}