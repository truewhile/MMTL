// Package service — subtitle handling.
//
// SubtitleService finds external subtitle files next to a media file AND
// embedded text subtitle tracks inside the media container, exposing both as
// WebVTT so the browser <track> element can load them directly.
//
// External-subtitle discovery rules (matching the legacy Python defaults):
//
//  1. Same directory, same basename, different extension.
//  2. Same directory, ".sub/" or "subs/" subdirectory.
//  3. Sibling languages e.g. movie.zh.srt / movie.en.srt → exposed as
//     ?lang=zh / ?lang=en.
//
// Supported extensions: .srt, .ass, .ssa, .vtt.
//
// Embedded subtitles are probed with ffprobe and exposed as
// path "embedded:<stream-index>"; the browser endpoint extracts the stream
// via ffmpeg into a cached .vtt file.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/ShukeBta/MMTL/internal/config"
	"github.com/ShukeBta/MMTL/internal/repository"
)

// SubtitleService is the discovery + conversion entry point.
type SubtitleService struct {
	log  *zap.Logger
	repo *repository.Container
	cfg  *config.Config
}

// NewSubtitleService is the constructor.
func NewSubtitleService(cfg *config.Config, log *zap.Logger, repo *repository.Container) *SubtitleService {
	return &SubtitleService{log: log, repo: repo, cfg: cfg}
}

// SubtitleTrack describes one external subtitle file.
type SubtitleTrack struct {
	Lang  string `json:"lang"`
	Label string `json:"label"`
	Path  string `json:"path"`
	URL   string `json:"url"`
	Codec string `json:"codec"`
}

// extToCodec maps the file extension to the inner codec name.
var extToCodec = map[string]string{
	".srt": "srt",
	".vtt": "vtt",
	".ass": "ass",
	".ssa": "ssa",
}

// Discover lists every external subtitle file for a media row. The URL is
// relative; the caller should prepend /api/subtitles/<media_id>?path=...
// when serializing for the frontend.
func (s *SubtitleService) Discover(ctx context.Context, mediaID string) ([]SubtitleTrack, error) {
	m, err := s.repo.Media.FindByID(ctx, mediaID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, errors.New("media not found")
	}
	dir := filepath.Dir(m.Path)
	base := strings.TrimSuffix(filepath.Base(m.Path), filepath.Ext(m.Path))

	candidates := make([]string, 0, 16)
	candidates = append(candidates, dir)
	for _, sub := range []string{"subs", "Subs", "sub", ".sub"} {
		candidates = append(candidates, filepath.Join(dir, sub))
	}

	tracks := make([]SubtitleTrack, 0)
	for _, c := range candidates {
		entries, err := os.ReadDir(c)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(e.Name()))
			codec, ok := extToCodec[ext]
			if !ok {
				continue
			}
			fullName := strings.TrimSuffix(e.Name(), ext)
			if !strings.HasPrefix(strings.ToLower(fullName), strings.ToLower(base)) &&
				c == dir {
				// In the same directory we require a basename match;
				// inside subs/ subdirs we accept anything.
				continue
			}
			lang := detectLang(fullName, base)
			tracks = append(tracks, SubtitleTrack{
				Lang:  lang,
				Label: lang,
				Path:  filepath.Join(c, e.Name()),
				Codec: codec,
			})
		}
	}

	// 容器内嵌文本字幕轨（MKV/MP4 等封装内的字幕流）：本地真实文件才可
	// 探测提取；cloud:// 与 .strm 媒体跳过。探测失败静默忽略（无 ffprobe
	// 或没有字幕流都属正常）。
	if embedded, ok := s.discoverEmbeddedTracks(ctx, m.Path); ok {
		tracks = append(tracks, embedded...)
	}
	return tracks, nil
}

// embeddedCodecOK 只暴露可提取为 WebVTT 的文本字幕编解码器；位图字幕
// （PGS/DVDSUB/DVBSUB）浏览器无法渲染，跳过。
func embeddedCodecOK(codec string) bool {
	switch strings.ToLower(codec) {
	case "subrip", "srt", "mov_text", "text", "webvtt", "ass", "ssa", "ttml", "sami":
		return true
	default:
		return false
	}
}

// ffprobeSubtitleStream 是 ffprobe -show_streams 输出的字幕流字段。
type ffprobeSubtitleStream struct {
	Index int               `json:"index"`
	Codec string            `json:"codec_name"`
	Tags  map[string]string `json:"tags"`
}

type ffprobeSubtitleContainer struct {
	Streams []ffprobeSubtitleStream `json:"streams"`
}

// discoverEmbeddedTracks 用 ffprobe 探测媒体容器内的文本字幕轨。
// 返回 (tracks, ok)：ok=false 表示该媒体不适用（非本地文件/ffprobe 不可用）。
func (s *SubtitleService) discoverEmbeddedTracks(ctx context.Context, mediaPath string) ([]SubtitleTrack, bool) {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(mediaPath)), "cloud://") ||
		strings.HasSuffix(strings.ToLower(strings.TrimSpace(mediaPath)), ".strm") {
		return nil, false
	}
	bin, err := resolveLocalExecutable(s.cfg.App.FFprobePath, "ffprobe")
	if err != nil {
		return nil, false
	}
	if _, err := os.Stat(mediaPath); err != nil {
		return nil, false
	}

	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, bin, // #nosec G204 -- bin resolved by resolveLocalExecutable; args are fixed probes.
		"-v", "error",
		"-select_streams", "s",
		"-show_entries", "stream=index,codec_name:stream_tags=language,title",
		"-of", "json",
		"--", mediaPath,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	var container ffprobeSubtitleContainer
	if err := json.Unmarshal(out, &container); err != nil {
		return nil, false
	}

	tracks := make([]SubtitleTrack, 0, len(container.Streams))
	for _, stream := range container.Streams {
		if !embeddedCodecOK(stream.Codec) {
			continue
		}
		lang := strings.ToLower(strings.TrimSpace(stream.Tags["language"]))
		if lang == "" {
			lang = "und"
		}
		label := stream.Tags["title"]
		if label == "" {
			label = lang
		}
		tracks = append(tracks, SubtitleTrack{
			Lang:  lang,
			Label: "内置字幕 · " + label,
			Path:  "embedded:" + strconv.Itoa(stream.Index),
			Codec: stream.Codec,
		})
	}
	return tracks, true
}

// langTag matches the .zh / .zh-cn / .chs language sub-extensions.
var langTag = regexp.MustCompile(`(?i)\.([a-z]{2,3}(?:[-_][a-z]{2,4})?)$`)

func detectLang(name, base string) string {
	suffix := strings.TrimPrefix(name, base)
	suffix = strings.TrimPrefix(suffix, ".")
	if m := langTag.FindStringSubmatch("." + suffix); len(m) >= 2 {
		return strings.ToLower(m[1])
	}
	if suffix == "" {
		return "und" // undetermined
	}
	return strings.ToLower(suffix)
}

// Serve writes the subtitle file as WebVTT (.vtt). SRT/SSA files are
// converted minimally on the fly; embedded container tracks (path
// "embedded:<index>") are extracted via ffmpeg into a cached .vtt.
// Returns ErrSubtitleNotFound when the path is rejected (path traversal /
// not in the media directory).
func (s *SubtitleService) Serve(ctx context.Context, mediaID, sub string, w io.Writer) error {
	m, err := s.repo.Media.FindByID(ctx, mediaID)
	if err != nil || m == nil {
		return errors.New("media not found")
	}
	if strings.HasPrefix(sub, "embedded:") {
		return s.ServeEmbeddedToVTT(ctx, m.Path, sub, w)
	}
	abs, err := filepath.Abs(sub)
	if err != nil {
		return err
	}
	mediaDir, _ := filepath.Abs(filepath.Dir(m.Path))
	if !pathWithin(abs, mediaDir) {
		return fmt.Errorf("path escape")
	}

	f, err := os.Open(abs) // #nosec G304 -- abs is constrained to the media file directory with pathWithin.
	if err != nil {
		return err
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	switch strings.ToLower(filepath.Ext(abs)) {
	case ".vtt":
		_, err = w.Write(body)
	case ".srt":
		_, err = w.Write([]byte(srtToVTT(string(body))))
	case ".ass", ".ssa":
		_, err = w.Write([]byte(assToVTT(string(body))))
	default:
		return errors.New("unsupported subtitle format")
	}
	return err
}

// embeddedSubtitleCachePath 内嵌字幕提取后的 WebVTT 缓存路径
// （按媒体路径哈希 + 轨道号定位，跨媒体互不干扰）。
func (s *SubtitleService) embeddedSubtitleCachePath(mediaPath string, idx int) string {
	hash := fmt.Sprintf("%x", fnvHash(mediaPath))
	return filepath.Join(s.cfg.Cache.CacheDir, "subs", hash, fmt.Sprintf("s%d.vtt", idx))
}

// ServeEmbeddedToVTT 把容器内第 idx 个字幕轨提取为 WebVTT 输出。
// 提取结果缓存在 cache 目录，媒体文件更新（mtime 变化）后自动重新提取。
func (s *SubtitleService) ServeEmbeddedToVTT(ctx context.Context, mediaPath, streamRef string, w io.Writer) error {
	idx, err := strconv.Atoi(strings.TrimPrefix(streamRef, "embedded:"))
	if err != nil || idx < 0 {
		return errors.New("invalid embedded subtitle index")
	}
	ffmpegBin, err := resolveLocalExecutable(s.cfg.App.FFmpegPath, "ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg 不可用，无法提取内嵌字幕：%w", err)
	}
	info, err := os.Stat(mediaPath)
	if err != nil {
		return errors.New("media file not found")
	}

	cachePath := s.embeddedSubtitleCachePath(mediaPath, idx)

	if cached, statErr := os.Stat(cachePath); statErr == nil && !info.ModTime().After(cached.ModTime()) {
		f, openErr := os.Open(cachePath) // #nosec G304 -- cachePath is generated under the cache dir.
		if openErr == nil {
			defer f.Close()
			_, copyErr := io.Copy(w, f)
			return copyErr
		}
	}

	// 缓存未命中或媒体已更新：ffmpeg 提取到临时文件后原子改名。
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o750); err != nil {
		return err
	}
	tmp := cachePath + ".tmp"
	extractCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(extractCtx, ffmpegBin, // #nosec G204 -- bin resolved by resolveLocalExecutable; args fixed extraction.
		"-v", "error", "-y",
		"-i", mediaPath,
		"-map", "0:s:"+strconv.Itoa(idx),
		"-f", "webvtt",
		tmp,
	)
	if out, runErr := cmd.CombinedOutput(); runErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("提取内嵌字幕失败（轨道 %d，可能为位图字幕或轨道无效）：%s", idx, strings.TrimSpace(string(out)))
	}
	if err := os.Rename(tmp, cachePath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	f, err := os.Open(cachePath) // #nosec G304 -- cachePath is generated under the cache dir.
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}

// fnvHash 简单 32 位 FNV-1a 哈希，用于生成稳定的缓存子目录名。
func fnvHash(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

// ServeRaw writes the subtitle file in its original format without any
// WebVTT conversion. Emby/Jellyfin clients advertise the source codec (ASS,
// subrip, etc.) in MediaStreams, then fetch the subtitle bytes via the
// DeliveryUrl and parse them with a decoder matching that codec — so the bytes
// must be the unmodified source, not a conversion. Unlike Serve (used by the
// browser <track> path, which requires WebVTT), ServeRaw preserves the file
// exactly as-is. Same path-safety constraints as Serve.
func (s *SubtitleService) ServeRaw(ctx context.Context, mediaID, sub string, w io.Writer) error {
	m, err := s.repo.Media.FindByID(ctx, mediaID)
	if err != nil || m == nil {
		return errors.New("media not found")
	}
	abs, err := filepath.Abs(sub)
	if err != nil {
		return err
	}
	mediaDir, _ := filepath.Abs(filepath.Dir(m.Path))
	if !pathWithin(abs, mediaDir) {
		return fmt.Errorf("path escape")
	}
	f, err := os.Open(abs) // #nosec G304 -- abs is constrained to the media file directory with pathWithin.
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}
