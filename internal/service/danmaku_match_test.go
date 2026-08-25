package service

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ShukeBta/MMTL/internal/model"
	"github.com/ShukeBta/MMTL/internal/service/cloud"
)

// overrideDanmakuOfficialBase points the "official" endpoint (match + fallback)
// at a local server for the duration of a test.
func overrideDanmakuOfficialBase(t *testing.T, base string) {
	t.Helper()
	old := danmakuOfficialBase
	danmakuOfficialBase = base
	t.Cleanup(func() { danmakuOfficialBase = old })
}

// writeDanmakuTestVideo writes a deterministic <16MB video-ish file and
// returns its path and the expected dandanplay hash (MD5 of the whole file,
// since the file is smaller than the 16MB prefix).
func writeDanmakuTestVideo(t *testing.T, name string) (path, wantHash string) {
	t.Helper()
	content := bytes.Repeat([]byte("MMTL-danmaku-hash-test-0123456789"), 500)
	path = filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, content, 0o644))
	sum := md5.Sum(content)
	return path, hex.EncodeToString(sum[:])
}

// danmakuOfficialServer serves /api/v2/match (with the given payload) and a
// comment library for the matched episode.
func danmakuOfficialServer(t *testing.T, matchBody, commentBody string, seen *string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/match", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if seen != nil {
			*seen = string(b)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, matchBody)
	})
	mux.HandleFunc("/api/v2/comment/25484", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, commentBody)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// seedDanmakuVideoMedia inserts a media row with a real path (for the hash
// layer) and optional episode number.
func seedDanmakuVideoMedia(t *testing.T, svc *DanmakuService, id, title, path string, size int64, episode int) {
	t.Helper()
	m := model.Media{Title: title, Path: path, SizeBytes: size, EpisodeNum: episode}
	m.ID = id
	require.NoError(t, svc.repo.DB.Create(&m).Error)
}

// 第 1 层：本地文件直接算 hash → 官方 /api/v2/match → 命中后拉弹幕。
func TestDanmakuFetchHashMatchLayer(t *testing.T) {
	videoPath, wantHash := writeDanmakuTestVideo(t, "测试动画.第01话.mkv")
	var seen string
	official := danmakuOfficialServer(t,
		`{"success":true,"isMatched":true,"matches":[{"episodeId":25484,"animeId":1001,"animeTitle":"测试动画","episodeTitle":"第1话"}]}`,
		`<?xml version="1.0"?><i><d p="0.5,1,16777215,user1">弹幕Hash命中</d></i>`,
		&seen)
	overrideDanmakuOfficialBase(t, official.URL)

	svc := newDanmakuTestService(t)
	ctx := context.Background()
	seedDanmakuVideoMedia(t, svc, "mH", "测试动画", videoPath, 32000, 1)

	res, err := svc.Fetch(ctx, "mH", "", "")
	require.NoError(t, err)
	require.True(t, res.Enabled)
	require.Equal(t, "xml", res.SourceType)
	require.Contains(t, res.Raw, "弹幕Hash命中")
	require.Empty(t, res.Candidates)
	require.Equal(t, "测试动画", res.AnimeTitle)
	require.Equal(t, "第1话", res.EpisodeTitle)
	require.Equal(t, int64(25484), res.EpisodeID)
	require.Equal(t, "hash", res.MatchMode)

	// match 请求体：文件名去扩展名并 URL 转义（官方接口要求，实测验证）、
	// hash、大小、matchMode 齐全。
	require.Contains(t, seen, `"fileName":"`+url.QueryEscape("测试动画.第01话")+`"`)
	require.Contains(t, seen, `"fileHash":"`+wantHash+`"`)
	require.Contains(t, seen, `"fileSize":32000`)
	require.Contains(t, seen, `"matchMode":"hashAndFileName"`)
}

// 第 1 层拉弹幕：配置了自定义源时优先自定义源，失败才回退官方。
func TestDanmakuFetchHashMatchUsesConfiguredSourceFirst(t *testing.T) {
	videoPath, _ := writeDanmakuTestVideo(t, "测试动画.第01话.mkv")

	cfgSrv := newDanmakuSourceServer(t) // /api/v2/comment/25484 → 弹幕A
	official := danmakuOfficialServer(t,
		`{"success":true,"isMatched":true,"matches":[{"episodeId":25484,"animeId":1001,"animeTitle":"测试动画"}]}`,
		`<?xml version="1.0"?><i><d p="0.5,1,16777215,user1">弹幕B官方</d></i>`,
		nil)
	overrideDanmakuOfficialBase(t, official.URL)

	svc := newDanmakuTestService(t)
	ctx := context.Background()
	require.NoError(t, svc.repo.Setting.Set(ctx, DanmakuSourceKey, cfgSrv.URL()))
	seedDanmakuVideoMedia(t, svc, "mC", "测试动画", videoPath, 32000, 0)

	res, err := svc.Fetch(ctx, "mC", "", "")
	require.NoError(t, err)
	// 配置源优先：弹幕来自自定义源而非官方。
	require.Contains(t, res.Raw, "弹幕A")
	require.NotContains(t, res.Raw, "弹幕B官方")
}

func TestDanmakuFetchHashMatchConfiguredFailsFallsBackOfficial(t *testing.T) {
	videoPath, _ := writeDanmakuTestVideo(t, "测试动画.第01话.mkv")

	// 配置源：搜索正常，但弹幕接口 500。
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/search/episodes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"hasMore":false,"animes":[{"animeId":1001,"animeTitle":"测试动画","episodes":[{"episodeId":25484,"episodeTitle":"第1话"}]}]}`)
	})
	mux.HandleFunc("/api/v2/comment/25484", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	cfgSrv := httptest.NewServer(mux)
	t.Cleanup(cfgSrv.Close)

	official := danmakuOfficialServer(t,
		`{"success":true,"isMatched":true,"matches":[{"episodeId":25484,"animeId":1001,"animeTitle":"测试动画"}]}`,
		`<?xml version="1.0"?><i><d p="0.5,1,16777215,user1">弹幕官方兜底</d></i>`,
		nil)
	overrideDanmakuOfficialBase(t, official.URL)

	svc := newDanmakuTestService(t)
	ctx := context.Background()
	require.NoError(t, svc.repo.Setting.Set(ctx, DanmakuSourceKey, cfgSrv.URL))
	seedDanmakuVideoMedia(t, svc, "mF", "测试动画", videoPath, 32000, 0)

	res, err := svc.Fetch(ctx, "mF", "", "")
	require.NoError(t, err)
	require.Contains(t, res.Raw, "弹幕官方兜底")
}

// 第 1 层未命中（matches 为空）→ 第 2 层按文件名+集数搜索。
func TestDanmakuFetchHashMissFallsBackToFileNameSearch(t *testing.T) {
	videoPath, _ := writeDanmakuTestVideo(t, "测试动画.第01话.mkv")

	cfgSrv := newDanmakuSourceServer(t) // 搜索 + 弹幕A
	official := danmakuOfficialServer(t,
		`{"success":true,"isMatched":false,"matches":[]}`,
		`<i></i>`, nil)
	overrideDanmakuOfficialBase(t, official.URL)

	svc := newDanmakuTestService(t)
	ctx := context.Background()
	require.NoError(t, svc.repo.Setting.Set(ctx, DanmakuSourceKey, cfgSrv.URL()))
	seedDanmakuVideoMedia(t, svc, "mM", "刮削标题", videoPath, 32000, 1)

	res, err := svc.Fetch(ctx, "mM", "", "")
	require.NoError(t, err)
	require.True(t, res.Enabled)
	require.Contains(t, res.Raw, "弹幕A")
	// 第 2 层命中：搜索请求按文件名进行。
	require.Contains(t, cfgSrv.lastSearch, "anime=")
}

// strm：通过解析出的直链 Range 拉 16MB 前缀算 hash → match → 拉弹幕。
// range server 模拟 115 CDN 防盗链：UA 不匹配直接 403（真实部署中
// 直链绑定换取时的 UA，不带绑定 UA 拉取会失败，见 pan115_openapi.go）。
func TestDanmakuFetchStrmHashViaDirectLink(t *testing.T) {
	content := bytes.Repeat([]byte("strm-video-bytes-0123456789"), 400)
	sum := md5.Sum(content)
	wantHash := hex.EncodeToString(sum[:])

	var gotRange, gotUA string
	rangeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRange = r.Header.Get("Range")
		gotUA = r.Header.Get("User-Agent")
		if gotUA != "bound-ua-115" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(content)
	}))
	t.Cleanup(rangeSrv.Close)

	var seen string
	official := danmakuOfficialServer(t,
		`{"success":true,"isMatched":true,"matches":[{"episodeId":25484,"animeId":1001,"animeTitle":"远程动画"}]}`,
		`<?xml version="1.0"?><i><d p="0.5,1,16777215,user1">弹幕Strm命中</d></i>`,
		&seen)
	overrideDanmakuOfficialBase(t, official.URL)

	svc := newDanmakuTestService(t)
	var gotProvider string
	svc.SetStrmResolver(func(_ context.Context, provider string, _ url.Values) (*StrmPlayResult, error) {
		gotProvider = provider
		// 播放链路 302 直链 + 绑定的 UA 头（防服务端直连 403）。
		return &StrmPlayResult{
			RedirectURL: rangeSrv.URL,
			Link:        &cloud.DirectLink{URL: rangeSrv.URL, Headers: map[string]string{"User-Agent": "bound-ua-115"}},
		}, nil
	})
	ctx := context.Background()
	strmPath := filepath.Join(t.TempDir(), "远程动画.第01话.strm")
	require.NoError(t, os.WriteFile(strmPath, []byte("http://example.invalid/api/strm/play/115/video.mkv?acct=1&pickcode=abc\n"), 0o644))
	seedDanmakuVideoMedia(t, svc, "mS", "远程动画", strmPath, 64, 0)
	// STRMURL 需要显式写回（扫库时才解析）。
	var media model.Media
	require.NoError(t, svc.repo.DB.First(&media, "id = ?", "mS").Error)
	media.STRMURL = "/api/strm/play/115/video.mkv?acct=1&pickcode=abc"
	require.NoError(t, svc.repo.DB.Save(&media).Error)

	res, err := svc.Fetch(ctx, "mS", "", "")
	require.NoError(t, err)
	require.True(t, res.Enabled)
	require.Contains(t, res.Raw, "弹幕Strm命中")
	require.Equal(t, "115", gotProvider)
	require.Contains(t, gotRange, "bytes=0-")
	require.Equal(t, "bound-ua-115", gotUA) // 防盗链 UA 必须透传
	require.Contains(t, seen, `"fileHash":"`+wantHash+`"`)
	require.Contains(t, seen, `"fileName":"`+url.QueryEscape("远程动画.第01话")+`"`)
	// strm 的 SizeBytes 是文本大小，不参与 match。
	require.Contains(t, seen, `"fileSize":0`)
}

// match 接口 fileName 语义：去扩展名；strm 文件名含视频扩展名时剥两层。
func TestDanmakuMatchFileName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/lib/某番剧.第01话.mkv", "某番剧.第01话"},
		{"/lib/某番剧.第01话.strm", "某番剧.第01话"},
		{"/lib/movie.mkv.strm", "movie"},
		{"plain", "plain"},
	}
	for _, c := range cases {
		require.Equal(t, c.want, danmakuMatchFileName(c.in), "path=%s", c.in)
	}
}

// hashLocalFile：本地视频直接读盘算前 16MB MD5，且第二次走缓存。
func TestDanmakuHashLocalFile(t *testing.T) {
	videoPath, wantHash := writeDanmakuTestVideo(t, "hashme.mkv")
	svc := newDanmakuTestService(t)
	got, ok := svc.hashLocalFile(videoPath)
	require.True(t, ok)
	require.Equal(t, wantHash, got)
	got2, ok := svc.hashLocalFile(videoPath)
	require.True(t, ok)
	require.Equal(t, wantHash, got2)
	missing, ok := svc.hashLocalFile(filepath.Join(t.TempDir(), "nope.mkv"))
	require.False(t, ok)
	require.Empty(t, missing)
}

// 配置源与官方同源时，回退不重复请求同一台服务器（bases 只含一份）。
func TestDanmakuSameBase(t *testing.T) {
	require.True(t, sameDanmakuBase("https://api.dandanplay.net", "https://api.dandanplay.net"))
	require.False(t, sameDanmakuBase("https://api.dandanplay.net", "https://dm.example.com"))
	require.False(t, sameDanmakuBase("", "https://api.dandanplay.net"))
}

// fetchCommentWithFallback：配置源与官方同源时不重复请求；
// 全失败时带出最后一跳错误。
func TestDanmakuFetchCommentWithFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	svc := newDanmakuTestService(t)
	ctx := context.Background()
	raw, st, err := svc.fetchCommentWithFallback(ctx, srv.URL, srv.URL, "25484")
	require.Error(t, err)
	require.Empty(t, raw)
	require.Equal(t, "auto", st)
}