// 网页端远程 Emby 库映射。
//
// 网页端（React UI）的媒体库/媒体浏览走项目自有 REST API（/api/libraries、
// /api/libraries/:id/media、/api/media/:id 等），数据结构为 model.Library /
// model.Media / SeriesCard。远程 Emby 挂载的数据不落库，因此这里把远程
// Emby 的 JSON item 映射为与本地完全一致的结构，让网页端无感知地浏览
// 远程库；播放统一走 /api/stream/{伪装ID}（302 到远程 Emby 原地址）。
package service

import (
	"context"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"github.com/ShukeBta/MMTL/internal/model"
)

// RemoteLibraryView 是一个网页端可见的远程媒体库（对应一个挂载的远程 View）。
type RemoteLibraryView struct {
	Library        model.Library
	MountID        string
	AccountID      string
	RemoteID       string
	CollectionType string
	AccountName    string
}

// RemoteLibraries 把所有启用挂载的远程媒体库映射为网页媒体库列表
// （只有显式挂载的库才出现在本项目媒体库中）。
func (r *EmbyRemoteService) RemoteLibraries(ctx context.Context) ([]RemoteLibraryView, error) {
	mounts, err := r.ListMounts(ctx)
	if err != nil || len(mounts) == 0 {
		return nil, err
	}
	// 按账号分组，每账号拉一次 Views 做匹配。
	byAccount := map[string][]*model.EmbyMount{}
	for i := range mounts {
		m := mounts[i]
		if !m.Enabled {
			continue
		}
		byAccount[m.AccountID] = append(byAccount[m.AccountID], &mounts[i])
	}
	out := make([]RemoteLibraryView, 0, len(mounts))
	for accountID, accountMounts := range byAccount {
		acct := r.AccountByID(ctx, accountID)
		if acct == nil {
			continue
		}
		cfg, cfgErr := r.configOf(acct)
		if cfgErr != nil {
			continue
		}
		views, viewErr := r.RemoteViews(ctx, acct)
		if viewErr != nil {
			if r.log != nil {
				r.log.Warn("web remote emby views failed",
					zap.String("account", acct.Name), zap.Error(viewErr))
			}
			continue
		}
		viewByName := map[string]map[string]any{}
		for _, v := range views {
			viewByName[remoteItemString(v, "Id")] = v
		}
		for _, mount := range accountMounts {
			v, ok := viewByName[mount.RemoteViewID]
			if !ok {
				continue // 远程已删除该媒体库
			}
			lib := r.mapRemoteMountToLibrary(mount, acct, cfg, v)
			if lib == nil {
				continue
			}
			out = append(out, RemoteLibraryView{
				Library:        *lib,
				MountID:        mount.ID,
				AccountID:      acct.ID,
				RemoteID:       mount.RemoteViewID,
				CollectionType: mount.CollectionType,
				AccountName:    acct.Name,
			})
		}
	}
	return out, nil
}

// RemoteLibraryByID 按伪装 ID 查远程库视图（详情接口用）。
func (r *EmbyRemoteService) RemoteLibraryByID(ctx context.Context, mountID, remoteViewID string) (*RemoteLibraryView, error) {
	views, err := r.RemoteLibraries(ctx)
	if err != nil {
		return nil, err
	}
	for _, v := range views {
		if v.MountID == mountID && v.RemoteID == remoteViewID {
			cp := v
			return &cp, nil
		}
	}
	return nil, nil
}

// mapRemoteMountToLibrary 把挂载信息 + 远程 View item 映射为网页库结构。
func (r *EmbyRemoteService) mapRemoteMountToLibrary(mount *model.EmbyMount, acct *model.StrmAccount, cfg *EmbyRemoteConfig, item map[string]any) *model.Library {
	if mount == nil {
		return nil
	}
	name := strings.TrimSpace(mount.Name)
	if name == "" {
		name = strings.TrimSpace(remoteItemString(item, "Name"))
	}
	if name == "" {
		name = acct.Name
	} else if !strings.Contains(name, acct.Name) {
		name = acct.Name + " · " + name
	}
	libType := "movie"
	switch mount.CollectionType {
	case "tvshows":
		libType = "tv"
	case "music":
		libType = "music"
	}
	lib := &model.Library{
		Base:      model.Base{ID: EncodeEmbyRemoteID(mount.ID, mount.RemoteViewID)},
		Name:      name,
		Type:      libType,
		Enabled:   true,
		SortOrder: 1000, // 远程库排在本地库之后
	}
	// 远程媒体库封面只有真实存在图片标签才下发。
	if remoteItemHasImageTag(item, "Primary") {
		lib.CoverURL = r.remoteItemImageURL(cfg, mount.RemoteViewID, "Primary")
	}
	return lib
}

// MapRemoteItemToMedia 把远程 Emby item JSON 映射为本地 Media 结构。
// poster/backdrop 只有在远程确实存在图片标签时才填 URL（避免对无图条目
// 发出必失败的图片请求导致前端破图）；剧集回退到系列海报（SeriesPrimaryImage）。
func (r *EmbyRemoteService) MapRemoteItemToMedia(ctx context.Context, mount *model.EmbyMount, acct *model.StrmAccount, cfg *EmbyRemoteConfig, item map[string]any) model.Media {
	encodeScope := acct.ID
	if mount != nil {
		encodeScope = mount.ID
	}
	remoteID := remoteItemString(item, "Id")
	// 条目可能已被 RewriteEmbyRemoteIDs 伪装（图片/嵌套 ID 需要原始远程 ID）。
	if _, rid, ok := DecodeEmbyRemoteID(remoteID); ok {
		remoteID = rid
	}
	seriesID := remoteItemString(item, "SeriesId")
	if _, rid, ok := DecodeEmbyRemoteID(seriesID); ok {
		seriesID = rid
	}
	media := model.Media{
		Base:         model.Base{ID: EncodeEmbyRemoteID(encodeScope, remoteID)},
		Title:        remoteItemString(item, "Name"),
		OriginalName: remoteItemString(item, "OriginalTitle"),
		Overview:     remoteItemString(item, "Overview"),
		Year:         remoteItemInt(item, "ProductionYear"),
		Rating:       float32(remoteItemFloat(item, "CommunityRating")),
		Path:         remoteItemString(item, "Path"),
		Genres:       remoteItemGenres(item),
		ScrapeStatus: "done",
	}
	// 只有远程明确存在图片标签才下发图片 URL。
	if remoteItemHasImageTag(item, "Primary") {
		media.PosterURL = r.remoteItemImageURL(cfg, remoteID, "Primary")
	}
	if remoteItemHasImageTag(item, "Backdrop") || len(remoteBackdropTags(item)) > 0 {
		media.BackdropURL = r.remoteItemImageURL(cfg, remoteID, "Backdrop")
	}
	if ticks := remoteItemInt64(item, "RunTimeTicks"); ticks > 0 {
		media.DurationSec = int(ticks / 10_000_000)
	}
	if date, ok := embyPremiereDate(remoteItemString(item, "PremiereDate")); ok {
		media.ReleaseDate = date.Format("2006-01-02")
	}
	if providerIDs, ok := item["ProviderIds"].(map[string]any); ok {
		if v := anyString(providerIDs["Tmdb"]); v != "" {
			media.TMDbID, _ = strconv.Atoi(v)
		}
		if v := anyString(providerIDs["Imdb"]); v != "" {
			media.TheTVDBID = v
		}
		if v := anyString(providerIDs["Douban"]); v != "" {
			media.DoubanID = v
		}
	}
	switch remoteItemString(item, "Type") {
	case "Episode":
		media.SeasonNum = remoteItemInt(item, "ParentIndexNumber")
		media.EpisodeNum = remoteItemInt(item, "IndexNumber")
		media.EpisodeTitle = remoteItemString(item, "Name")
		if seriesName := remoteItemString(item, "SeriesName"); seriesName != "" {
			media.Title = seriesName
		}
		// 单集通常无独立海报：若远程返回 SeriesPrimaryImageTag（需要
		// Fields=SeriesPrimaryImage）且系列有图，则回退到系列海报。
		if media.PosterURL == "" && seriesID != "" &&
			strings.TrimSpace(remoteItemString(item, "SeriesPrimaryImageTag")) != "" {
			media.PosterURL = r.remoteItemImageURL(cfg, seriesID, "Primary")
		}
	default: // Movie / Series / Season / Folder
		media.SeasonNum = 0
		media.EpisodeNum = 0
	}
	return media
}

// RemoteLibraryMedia 拉远程库直属条目（电影库=Movie，剧集库=Series），映射分页。
func (r *EmbyRemoteService) RemoteLibraryMedia(ctx context.Context, mount *model.EmbyMount, acct *model.StrmAccount, remoteViewID string, itemTypes string, offset, limit int) ([]model.Media, int64, error) {
	cfg, err := r.configOf(acct)
	if err != nil {
		return nil, 0, err
	}
	if itemTypes == "" {
		itemTypes = "Movie,Series" // 未知类型时两者都取（前端自行按 episode-like 分组）
	}
	q := url.Values{}
	q.Set("ParentId", remoteViewID)
	q.Set("IncludeItemTypes", itemTypes)
	q.Set("Recursive", "false")
	q.Set("StartIndex", strconv.Itoa(offset))
	q.Set("Limit", strconv.Itoa(limit))
	q.Set("Fields", "Overview,Genres,ProviderIds,Path,SeriesPrimaryImage")
	var body struct {
		Items            []map[string]any `json:"Items"`
		TotalRecordCount int64            `json:"TotalRecordCount"`
	}
	if err := r.doGet(ctx, acct, cfg, "/Users/"+url.PathEscape(r.remoteUserID(cfg))+"/Items", q, &body); err != nil {
		return nil, 0, err
	}
	items := make([]model.Media, 0, len(body.Items))
	for _, it := range body.Items {
		RewriteEmbyRemoteIDs(it, mount.ID) // 嵌套/关联 ID 一并伪装
		items = append(items, r.MapRemoteItemToMedia(ctx, mount, acct, cfg, it))
	}
	return items, body.TotalRecordCount, nil
}

// RemoteMediaDetail 拉远程单条目映射为 Media（网页详情页）。
func (r *EmbyRemoteService) RemoteMediaDetail(ctx context.Context, mount *model.EmbyMount, acct *model.StrmAccount, remoteID string) (*model.Media, error) {
	cfg, err := r.configOf(acct)
	if err != nil {
		return nil, err
	}
	path := "/Users/" + url.PathEscape(r.remoteUserID(cfg)) + "/Items/" + url.PathEscape(remoteID)
	path += "?Fields=Overview,Genres,ProviderIds,People,Studios,Path"
	var out map[string]any
	if err := r.doGet(ctx, acct, cfg, path, nil, &out); err != nil {
		return nil, err
	}
	RewriteEmbyRemoteIDs(out, mount.ID)
	m := r.MapRemoteItemToMedia(ctx, mount, acct, cfg, out)
	return &m, nil
}

// RemoteEpisodes 拉远程条目下的集列表（Series/Season/Folder→子集；Episode→同系列；
// Movie→自身单条），按季/集排序，与本地 ListMediaEpisodes 行为一致。
func (r *EmbyRemoteService) RemoteEpisodes(ctx context.Context, mount *model.EmbyMount, acct *model.StrmAccount, remoteID string) ([]model.Media, error) {
	detail, err := r.RemoteMediaDetail(ctx, mount, acct, remoteID)
	if err != nil {
		return nil, err
	}
	// 用远程详情载荷精判类型（Episode→同系列；Series/Season/Folder→子集；Movie→单条）。
	itemType := r.remoteItemType(ctx, acct, remoteID)
	if itemType == "" {
		itemType = remoteItemTypeOf(detail)
	}
	var parentID string
	switch itemType {
	case "Episode":
		parentID = r.remoteItemSeriesID(ctx, acct, remoteID)
		if parentID == "" {
			parentID = remoteID
		}
	case "Season", "Folder", "Series":
		parentID = remoteID
	default: // Movie
		return []model.Media{*detail}, nil
	}
	rows, _, err := r.remoteEpisodesOf(ctx, mount, acct, parentID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].SeasonNum != rows[j].SeasonNum {
			return rows[i].SeasonNum < rows[j].SeasonNum
		}
		if rows[i].EpisodeNum != rows[j].EpisodeNum {
			return rows[i].EpisodeNum < rows[j].EpisodeNum
		}
		return rows[i].Title < rows[j].Title
	})
	return rows, nil
}

func (r *EmbyRemoteService) remoteEpisodesOf(ctx context.Context, mount *model.EmbyMount, acct *model.StrmAccount, parentID string) ([]model.Media, int64, error) {
	cfg, err := r.configOf(acct)
	if err != nil {
		return nil, 0, err
	}
	q := url.Values{}
	q.Set("ParentId", parentID)
	q.Set("IncludeItemTypes", "Episode")
	q.Set("Recursive", "true")
	q.Set("StartIndex", "0")
	q.Set("Limit", "500")
	q.Set("Fields", "Overview,Genres,ProviderIds,Path,SeriesPrimaryImage")
	var body struct {
		Items            []map[string]any `json:"Items"`
		TotalRecordCount int64            `json:"TotalRecordCount"`
	}
	if err := r.doGet(ctx, acct, cfg, "/Users/"+url.PathEscape(r.remoteUserID(cfg))+"/Items", q, &body); err != nil {
		return nil, 0, err
	}
	items := make([]model.Media, 0, len(body.Items))
	for _, it := range body.Items {
		RewriteEmbyRemoteIDs(it, mount.ID)
		m := r.MapRemoteItemToMedia(ctx, mount, acct, cfg, it)
		items = append(items, m)
	}
	return items, body.TotalRecordCount, nil
}

// RemoteSeriesCards 远程剧集库的系列卡片（ChildCount 作为集数）。
func (r *EmbyRemoteService) RemoteSeriesCards(ctx context.Context, mount *model.EmbyMount, acct *model.StrmAccount, remoteViewID string) ([]SeriesCard, error) {
	cfg, err := r.configOf(acct)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("ParentId", remoteViewID)
	q.Set("IncludeItemTypes", "Series")
	q.Set("Recursive", "false")
	q.Set("StartIndex", "0")
	q.Set("Limit", "1000")
	q.Set("Fields", "Overview,Genres,ProviderIds,Path,RecursiveItemCount,SeriesPrimaryImage")
	var body struct {
		Items []map[string]any `json:"Items"`
	}
	if err := r.doGet(ctx, acct, cfg, "/Users/"+url.PathEscape(r.remoteUserID(cfg))+"/Items", q, &body); err != nil {
		return nil, err
	}
	cards := make([]SeriesCard, 0, len(body.Items))
	for _, it := range body.Items {
		RewriteEmbyRemoteIDs(it, mount.ID)
		m := r.MapRemoteItemToMedia(ctx, mount, acct, cfg, it)
		// 集数优先用递归条目数（ChildCount 只算直属 Season 文件夹数）。
		count := remoteItemInt(it, "RecursiveItemCount")
		if count == 0 {
			count = remoteItemInt(it, "ChildCount")
		}
		if count == 0 {
			count = 1
		}
		cards = append(cards, SeriesCard{Key: m.ID, Rep: m, LinkMedia: m, Count: count})
	}
	return cards, nil
}

// RemoteLatestCards 远程库最新条目（首页预览卡片），映射 SeriesCard。
func (r *EmbyRemoteService) RemoteLatestCards(ctx context.Context, mount *model.EmbyMount, acct *model.StrmAccount, remoteViewID string, limit int) ([]SeriesCard, error) {
	cfg, err := r.configOf(acct)
	if err != nil {
		return nil, err
	}
	items, err := r.RemoteLatest(ctx, mount, acct, remoteViewID, limit)
	if err != nil {
		return nil, err
	}
	cards := make([]SeriesCard, 0, len(items))
	for _, it := range items {
		m := r.MapRemoteItemToMedia(ctx, mount, acct, cfg, it)
		cards = append(cards, SeriesCard{Key: m.ID, Rep: m, LinkMedia: m, Count: 0})
	}
	return cards, nil
}

// WebStreamURL 远程条目的网页播放地址（302 直连远程 Emby 流端点）。
func (r *EmbyRemoteService) WebStreamURL(ctx context.Context, acct *model.StrmAccount, remoteID string) (string, error) {
	cfg, err := r.configOf(acct)
	if err != nil {
		return "", err
	}
	if err := r.ensureToken(ctx, acct, cfg); err != nil {
		return "", err
	}
	return r.embyBase(cfg) + "/Videos/" + url.PathEscape(remoteID) +
		"/stream?api_key=" + url.QueryEscape(cfg.Token) + "&Static=true", nil
}

// remoteItemType 轻量查询远程条目 Type（避免依赖映射载荷）。
func (r *EmbyRemoteService) remoteItemType(ctx context.Context, acct *model.StrmAccount, remoteID string) string {
	cfg, err := r.configOf(acct)
	if err != nil {
		return ""
	}
	var out map[string]any
	if err := r.doGet(ctx, acct, cfg, "/Users/"+url.PathEscape(r.remoteUserID(cfg))+"/Items/"+url.PathEscape(remoteID), nil, &out); err != nil {
		return ""
	}
	return remoteItemString(out, "Type")
}

// remoteItemSeriesID 轻量查询 Episode 的 SeriesId。
func (r *EmbyRemoteService) remoteItemSeriesID(ctx context.Context, acct *model.StrmAccount, remoteID string) string {
	cfg, err := r.configOf(acct)
	if err != nil {
		return ""
	}
	var out map[string]any
	if err := r.doGet(ctx, acct, cfg, "/Users/"+url.PathEscape(r.remoteUserID(cfg))+"/Items/"+url.PathEscape(remoteID), nil, &out); err != nil {
		return ""
	}
	return remoteItemString(out, "SeriesId")
}

// ─── 远程 item JSON 取值辅助 ────────────────────────────────────────────────

func anyString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func remoteItemString(item map[string]any, key string) string {
	if item == nil {
		return ""
	}
	if s, ok := item[key].(string); ok {
		return s
	}
	return ""
}

func remoteItemInt(item map[string]any, key string) int {
	if item == nil {
		return 0
	}
	switch v := item[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func remoteItemInt64(item map[string]any, key string) int64 {
	if item == nil {
		return 0
	}
	switch v := item[key].(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	}
	return 0
}

func remoteItemFloat(item map[string]any, key string) float64 {
	if item == nil {
		return 0
	}
	switch v := item[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}
	return 0
}

// remoteItemGenres 合并 GenreItems / Genres 数组为逗号分隔字符串（前端 parseCSV 消费）。
func remoteItemGenres(item map[string]any) string {
	seen := map[string]bool{}
	var parts []string
	collect := func(arr any) {
		list, ok := arr.([]any)
		if !ok {
			return
		}
		for _, it := range list {
			var name string
			if m, isMap := it.(map[string]any); isMap {
				name = remoteItemString(m, "Name")
			} else if s, isStr := it.(string); isStr {
				name = s
			}
			if name != "" && !seen[name] {
				seen[name] = true
				parts = append(parts, name)
			}
		}
	}
	collect(item["GenreItems"])
	collect(item["Genres"])
	return strings.Join(parts, ",")
}

// remoteItemImageURL 构造远程条目图片绝对地址（带 api_key；前端经 /api/img 代理）。
func (r *EmbyRemoteService) remoteItemImageURL(cfg *EmbyRemoteConfig, remoteID, imageType string) string {
	if remoteID == "" {
		return ""
	}
	imageType = strings.ToLower(imageType)
	if imageType == "" {
		imageType = "primary"
	}
	return r.embyBase(cfg) + "/Items/" + url.PathEscape(remoteID) + "/Images/" + url.PathEscape(imageType) +
		"?api_key=" + url.QueryEscape(cfg.Token)
}

// remoteItemHasImageTag 远程 item 是否带某类型图片标签（Emby 的 ImageTags map）。
func remoteItemHasImageTag(item map[string]any, typ string) bool {
	if item == nil {
		return false
	}
	switch tags := item["ImageTags"].(type) {
	case map[string]any:
		_, ok := tags[typ]
		return ok
	case map[string]string:
		_, ok := tags[typ]
		return ok
	}
	return false
}

// remoteBackdropTags 远程 item 的 BackdropImageTags 数组。
func remoteBackdropTags(item map[string]any) []any {
	if item == nil {
		return nil
	}
	switch tags := item["BackdropImageTags"].(type) {
	case []any:
		return tags
	case []string:
		out := make([]any, 0, len(tags))
		for _, s := range tags {
			out = append(out, s)
		}
		return out
	}
	return nil
}

// remoteItemTypeOf 从映射后的 Media 推断远程类型（无详情载荷时兜底）。
func remoteItemTypeOf(m *model.Media) string {
	if m == nil {
		return "Movie"
	}
	if m.EpisodeNum > 0 || m.SeasonNum > 0 {
		return "Episode"
	}
	return "Movie"
}
// ─── 供 handler 层使用的远程 View 条目取值（导出薄封装） ──────────────────────

// RemoteItemIDString 提取远程 View 条目的 Id。
func RemoteItemIDString(item map[string]any) string { return remoteItemString(item, "Id") }

// RemoteItemNameString 提取远程 View 条目的 Name。
func RemoteItemNameString(item map[string]any) string { return remoteItemString(item, "Name") }

// RemoteItemCollectionType 提取远程 View 条目的 CollectionType。
func RemoteItemCollectionType(item map[string]any) string { return remoteItemString(item, "CollectionType") }

// RemoteItemChildCount 提取远程 View 条目的 ChildCount。
func RemoteItemChildCount(item map[string]any) int { return remoteItemInt(item, "ChildCount") }
