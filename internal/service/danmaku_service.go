package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/ShukeBta/MediaStationGo/internal/repository"
)

// Danmaku setting keys, managed through the admin settings UI (PUT
// /admin/settings). They are stored in the Setting table so the playback
// page can pull them without admin privileges.
const (
	DanmakuEnabledKey  = "danmaku.enabled"
	DanmakuSourceKey   = "danmaku.source"
	DanmakuOpacityKey  = "danmaku.opacity"
	DanmakuFontSizeKey = "danmaku.font_size"
	DanmakuAreaKey     = "danmaku.area"
)

// DanmakuDefaultSource is the official dandanplay endpoint used when the
// admin leaves the source address empty. Any server implementing the
// dandanplay protocol (search/episodes + comment/{episodeId}) may be used.
const DanmakuDefaultSource = "https://api.dandanplay.net"

// DanmakuRenderConfig carries the renderer knobs to the web player.
type DanmakuRenderConfig struct {
	Enabled  bool   `json:"enabled"`
	Source   string `json:"source,omitempty"`
	Opacity  string `json:"opacity"`
	FontSize string `json:"font_size"`
	Area     string `json:"area"`
}

// DanmakuFetchResult is what /api/danmaku/:id returns. Raw holds the upstream
// comment payload; parsing happens client-side. The dandanplay protocol has
// two payload shapes in the wild — the classic Bilibili-style XML and the
// newer dandanplay JSON ({"count":N,"comments":[{p,m,t,...}]}) — so
// source_type is sniffed from the body rather than fixed.
//
// Candidates is non-nil when multiple anime matched the search and the player
// must ask the user which one to use (disambiguation); Raw is empty then.
type DanmakuFetchResult struct {
	DanmakuRenderConfig
	SourceType string         `json:"source_type"`
	Raw        string         `json:"raw,omitempty"`
	Candidates []DanmakuAnime `json:"candidates,omitempty"`
}

// DanmakuAnime is one search hit (an anime) with its episode list, mirroring
// the dandanplay SearchEpisodesResponse shape.
type DanmakuAnime struct {
	AnimeID    int64            `json:"animeId"`
	AnimeTitle string           `json:"animeTitle"`
	Episodes   []DanmakuEpisode `json:"episodes"`
}

// DanmakuEpisode is one selectable danmaku library inside an anime.
type DanmakuEpisode struct {
	EpisodeID    int64  `json:"episodeId"`
	EpisodeTitle string `json:"episodeTitle"`
}

// DanmakuService fetches danmaku for a media item through the dandanplay
// protocol: search for an episode id by the video's name, then fetch the
// comment library XML. The React player parses and renders it.
type DanmakuService struct {
	log    *zap.Logger
	repo   *repository.Container
	client *http.Client
}

func danmakuHTTPClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second}
}

func NewDanmakuService(log *zap.Logger, repo *repository.Container) *DanmakuService {
	if log == nil {
		log = zap.NewNop()
	}
	return &DanmakuService{log: log, repo: repo, client: danmakuHTTPClient()}
}

// Config reads danmaku settings from the runtime settings table.
func (s *DanmakuService) Config(ctx context.Context) DanmakuRenderConfig {
	cfg := DanmakuRenderConfig{
		Opacity:  "1",
		FontSize: "24",
		Area:     "1",
	}
	if s == nil || s.repo == nil || s.repo.Setting == nil {
		return cfg
	}
	read := func(key, fallback string) string {
		v, err := s.repo.Setting.Get(ctx, key)
		if err != nil || v == "" {
			return fallback
		}
		return v
	}
	cfg.Enabled = ParseBoolSetting(read(DanmakuEnabledKey, "true"), true)
	cfg.Source = read(DanmakuSourceKey, "")
	cfg.Opacity = read(DanmakuOpacityKey, "1")
	cfg.FontSize = read(DanmakuFontSizeKey, "24")
	cfg.Area = read(DanmakuAreaKey, "1")
	return cfg
}

// Fetch retrieves danmaku for the given media. keyword overrides the
// media-derived search term (empty = use the video's own name); pass it from
// the player when the user searches for a custom title. episodeID forces a
// specific danmaku library chosen by the user (from a previous disambiguation
// response); empty means auto-resolution. The danmaku library is resolved
// through the dandanplay protocol:
//
//  1. search episodes by title (+ season/episode number)
//     2a. exactly one hit → fetch that episode's comment library
//     2b. several hits → return candidates (Raw empty) so the player asks the user
//     2c. explicit episodeID → fetch it directly
//  3. fetch the comment library XML
//
// When danmaku is disabled the result carries Enabled=false so the player can
// silently skip rendering.
func (s *DanmakuService) Fetch(ctx context.Context, mediaID, keyword, episodeID string) (*DanmakuFetchResult, error) {
	res := &DanmakuFetchResult{DanmakuRenderConfig: s.Config(ctx), SourceType: "auto"}
	if !res.Enabled {
		return res, nil
	}

	base := strings.TrimRight(strings.TrimSpace(res.Source), "/")
	if base == "" {
		base = DanmakuDefaultSource
	}

	target := strings.TrimSpace(episodeID)
	if target == "" {
		// 名称与集数优先来自媒体（original_name → title → 文件名），
		// 手动搜索关键词时仍沿用当前媒体的集数（同一部番剧同名搜索）。
		term, err := s.searchTerms(ctx, mediaID)
		if err != nil {
			return res, err
		}
		if kw := strings.TrimSpace(keyword); kw != "" {
			term.name = kw
		}
		if strings.TrimSpace(term.name) == "" {
			return res, nil
		}

		candidates, err := s.searchCandidates(ctx, base, term.name, term.episode)
		if err != nil {
			s.log.Warn("danmaku search failed", zap.String("media_id", mediaID), zap.String("name", term.name), zap.String("episode", term.episode), zap.Error(err))
			return res, err
		}
		// 多结果歧义：把候选交回播放器让用户选择（disambiguation）。
		if len(candidates) != 1 {
			res.Candidates = candidates
			return res, nil
		}
		if len(candidates[0].Episodes) == 0 {
			return res, errors.New("no danmaku library found for this video")
		}
		target = fmt.Sprintf("%d", candidates[0].Episodes[0].EpisodeID)
	}

	raw, err := s.fetchBody(ctx, fmt.Sprintf("%s/api/v2/comment/%s?withRelated=true", base, target), true)
	if err != nil {
		s.log.Warn("danmaku comment fetch failed", zap.String("media_id", mediaID), zap.String("episode_id", target), zap.Error(err))
		return res, err
	}
	res.Raw = raw
	res.SourceType = detectDanmakuSourceType(raw)
	return res, nil
}

// detectDanmakuSourceType guesses the comment payload format from its body.
// The dandanplay protocol historically returned Bilibili-style XML, but newer
// and self-hosted implementations return the dandanplay JSON shape
// ({"count":N,"comments":[{p,m,t,...}]}); sniff the leading byte instead of
// trusting the source. Unknown/empty bodies fall back to "auto" so the player
// can try both parsers.
func detectDanmakuSourceType(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "auto"
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		return "json"
	}
	return "xml"
}

type danmakuSearchTerms struct {
	name    string // 作品标题（original_name → title → 文件名）
	episode string // 集数编号（dandanplay episode 参数，正整数）
}

// searchTerms resolves the name and episode number used to look up the
// dandanplay library. Episode 0 (movies / unknown) is left empty so the
// search does not filter by episode.
func (s *DanmakuService) searchTerms(ctx context.Context, mediaID string) (danmakuSearchTerms, error) {
	var term danmakuSearchTerms
	if s == nil || s.repo == nil || s.repo.Media == nil {
		return term, errors.New("media repository unavailable")
	}
	m, err := s.repo.Media.FindByID(ctx, mediaID)
	if err != nil {
		return term, err
	}
	if m == nil {
		return term, errors.New("media not found")
	}
	if name := strings.TrimSpace(m.OriginalName); name != "" {
		term.name = name
	} else if name := strings.TrimSpace(m.Title); name != "" {
		term.name = name
	} else {
		term.name = strings.TrimSuffix(filepath.Base(m.Path), filepath.Ext(m.Path))
	}
	if m.EpisodeNum > 0 {
		term.episode = strconv.Itoa(m.EpisodeNum)
	}
	return term, nil
}

// searchCandidates returns every anime hit for a name via the dandanplay
// search endpoint, with the episode list per anime. The optional episode
// number filters the result to that specific episode. Responses follow
// SearchEpisodesResponse: { animes: [{ animeId, animeTitle, episodes:
// [{ episodeId, episodeTitle }] }] }.
func (s *DanmakuService) searchCandidates(ctx context.Context, base, name, episode string) ([]DanmakuAnime, error) {
	u := fmt.Sprintf("%s/api/v2/search/episodes?anime=%s&v2=true", base, url.QueryEscape(name))
	if episode != "" {
		u += "&episode=" + url.QueryEscape(episode)
	}
	body, err := s.fetchBody(ctx, u, false)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Animes []struct {
			AnimeID    int64  `json:"animeId"`
			AnimeTitle string `json:"animeTitle"`
			Episodes   []struct {
				EpisodeID    int64  `json:"episodeId"`
				EpisodeTitle string `json:"episodeTitle"`
			} `json:"episodes"`
		} `json:"animes"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, fmt.Errorf("danmaku search returned invalid JSON: %w", err)
	}
	var out []DanmakuAnime
	for _, anime := range resp.Animes {
		item := DanmakuAnime{AnimeID: anime.AnimeID, AnimeTitle: anime.AnimeTitle}
		for _, ep := range anime.Episodes {
			if ep.EpisodeID <= 0 {
				continue
			}
			item.Episodes = append(item.Episodes, DanmakuEpisode{
				EpisodeID:    ep.EpisodeID,
				EpisodeTitle: ep.EpisodeTitle,
			})
		}
		if len(item.Episodes) == 0 {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *DanmakuService) fetchBody(ctx context.Context, sourceURL string, followRedirect bool) (string, error) {
	client := s.client
	if !followRedirect {
		// Search returns plain JSON; the comment endpoint 302-redirects to a
		// danmaku accelerator CDN which the default client follows.
		copied := *s.client
		copied.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
		client = &copied
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "MediaStationGo/danmaku (+https://github.com/ShukeBta/MediaStationGo)")
	req.Header.Set("Accept", "application/json, application/xml, */*")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("danmaku source returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20)) // 8 MiB cap
	if err != nil {
		return "", err
	}
	return string(body), nil
}
