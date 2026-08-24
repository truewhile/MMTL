// STRM 管理服务：网盘账号、同步目录、STRM 生成与元数据下载/上传队列。
//
// 设计参考 QMediaSync 的 STRM 同步：同步目录扫描网盘（115 / CloudDrive2 /
// OpenList，驱动复用 internal/service/cloud 包）或本地目录，视频文件生成
// 指向本服务播放端点的一行 URL 的 .strm 文件；元数据文件（nfo/图片/字幕）
// 经下载/上传队列与远端双向同步。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/ShukeBta/MediaStationGo/internal/config"
	"github.com/ShukeBta/MediaStationGo/internal/model"
	"github.com/ShukeBta/MediaStationGo/internal/repository"
	"github.com/ShukeBta/MediaStationGo/internal/service/cloud"
)

// strm 全局设置键（存于 Setting 表，strm.* 前缀）。
const (
	StrmSettingBaseURL         = "strm.base_url"
	StrmSettingVideoExt        = "strm.video_ext"
	StrmSettingMetaExt         = "strm.meta_ext"
	StrmSettingExcludeName     = "strm.exclude_name"
	StrmSettingMinVideoSizeMB  = "strm.min_video_size_mb"
	StrmSettingAddPath         = "strm.add_path"
	StrmSettingDownloadMeta    = "strm.download_meta"
	StrmSettingUploadMeta      = "strm.upload_meta"
	StrmSettingDeleteDir       = "strm.delete_dir"
	StrmSettingDownloadThreads = "strm.download_threads"
	StrmSettingUploadThreads   = "strm.upload_threads"
)

const (
	StrmDefaultVideoExt = "mkv,mp4,avi,rmvb,rm,mov,ts,wmv,flv,m4v,iso,mpg,mpeg,webm"
	StrmDefaultMetaExt  = "nfo,jpg,jpeg,png,srt,ass,ssa,sub,txt,bmp,webp"
	StrmDefaultExclude  = "sample,trailer,预告"
)

// StrmSettingDefs 是全局 strm 设置的默认值表，供设置对话框展示。
var StrmSettingDefs = map[string]struct {
	Default string
	Label   string
	Kind    string // text / number / bool / choice
	Choices []string
	Help    string
}{
	StrmSettingBaseURL:         {Default: "", Label: "STRM 链接基础地址", Kind: "text", Help: "生成的 strm 文件指向的播放地址（默认留空自动取服务器公网地址 app.server_url）。例如 http://192.168.1.10:8096"},
	StrmSettingVideoExt:        {Default: StrmDefaultVideoExt, Label: "视频扩展名", Kind: "text", Help: "逗号分隔；命中即生成 .strm，其余文件视为元数据"},
	StrmSettingMetaExt:         {Default: StrmDefaultMetaExt, Label: "元数据扩展名", Kind: "text", Help: "逗号分隔；进入下载/上传队列的文件类型（nfo/图片/字幕）"},
	StrmSettingExcludeName:     {Default: StrmDefaultExclude, Label: "排除文件名", Kind: "text", Help: "逗号分隔；文件名包含任一关键词即跳过"},
	StrmSettingMinVideoSizeMB:  {Default: "0", Label: "最小视频大小(MB)", Kind: "number", Help: "小于该大小的视频文件不生成 STRM，0 表示不限"},
	StrmSettingAddPath:         {Default: "1", Label: "STRM 链接 path 参数", Kind: "choice", Choices: []string{"1", "2", "3"}, Help: "1=附带完整远端路径 2=仅文件名 3=不带 path"},
	StrmSettingDownloadMeta:    {Default: "true", Label: "下载元数据", Kind: "bool", Help: "同步时把远端 nfo/图片/字幕下载到本地输出目录"},
	StrmSettingUploadMeta:      {Default: "false", Label: "上传元数据", Kind: "bool", Help: "同步时把本地元数据上传到远端（需网盘支持写入）"},
	StrmSettingDeleteDir:       {Default: "false", Label: "清理空目录", Kind: "bool", Help: "清理远端已删除的多余 .strm/元数据后，删除空目录"},
	Strm115RelayKeySetting:     {Default: "", Label: "115 中继授权共享密钥", Kind: "text", Help: "QMediaSync/MQFamily 中继授权的共享 AES 密钥（OAUTH_RELAY_ENCRYPTION_KEY）；不配置则中继授权不可用"},
	StrmSettingDownloadThreads: {Default: "3", Label: "下载队列线程数", Kind: "number", Help: "元数据下载并发数"},
	StrmSettingUploadThreads:   {Default: "2", Label: "上传队列线程数", Kind: "number", Help: "元数据上传并发数"},
}

// StrmAccountSecretKeys 是账号配置中需要加密存储的字段。
var StrmAccountSecretKeys = []string{"cookie", "password", "token", "access_token", "refresh_token"}

// StrmService 提供 STRM 管理的能力。
type StrmService struct {
	log      *zap.Logger
	repo     *repository.Container
	cfg      *config.Config
	crypto   *CryptoService
	http     *http.Client
	stopOnce sync.Once
	stopCh   chan struct{}
	baseCtx  context.Context // 服务级长期上下文（同步/队列不随 HTTP 请求取消）

	mu            sync.Mutex
	running       map[string]context.CancelFunc // sync path id -> cancel
	oauthSessions map[string]*strm115AuthSession
}

// NewStrmService constructs the STRM service.
func NewStrmService(cfg *config.Config, log *zap.Logger, repos *repository.Container, crypto *CryptoService) *StrmService {
	return &StrmService{
		log:           log,
		repo:          repos,
		cfg:           cfg,
		crypto:        crypto,
		http:          &http.Client{Timeout: 90 * time.Second},
		stopCh:        make(chan struct{}),
		baseCtx:       context.Background(),
		running:       map[string]context.CancelFunc{},
		oauthSessions: map[string]*strm115AuthSession{},
	}
}

// Start 启动下载/上传队列 worker、定时同步巡检、115 token 刷新与队列清理。
func (s *StrmService) Start(ctx context.Context) {
	s.sync115RelayKey(ctx)
	downloadThreads := s.strmIntSetting(ctx, StrmSettingDownloadThreads, 3)
	if downloadThreads < 1 {
		downloadThreads = 1
	}
	if downloadThreads > 8 {
		downloadThreads = 8
	}
	uploadThreads := s.strmIntSetting(ctx, StrmSettingUploadThreads, 2)
	if uploadThreads < 1 {
		uploadThreads = 1
	}
	if uploadThreads > 4 {
		uploadThreads = 4
	}
	for i := 0; i < downloadThreads; i++ {
		go s.downloadWorker(ctx)
	}
	for i := 0; i < uploadThreads; i++ {
		go s.uploadWorker(ctx)
	}
	go s.cronLoop(ctx)
	go s.queueCleanupLoop(ctx)
	go s.refresh115TokensLoop(ctx)
	s.log.Info("strm service started",
		zap.Int("download_threads", downloadThreads),
		zap.Int("upload_threads", uploadThreads))
}

func (s *StrmService) Stop() {
	s.stopOnce.Do(func() { close(s.stopCh) })
}

// ─── 网盘账号 ──────────────────────────────────────────────────────────────────

// strmAccountConfigJSON 序列化账号配置并加密敏感字段。
func (s *StrmService) strmAccountConfigJSON(values map[string]string, encrypt bool) (string, error) {
	cfg := make(map[string]string, len(values))
	for k, v := range values {
		if v == "" {
			continue
		}
		if encrypt && strmContains(StrmAccountSecretKeys, k) {
			cfg[k] = s.crypto.Encrypt(v)
		} else {
			cfg[k] = v
		}
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// strmAccountConfig 解密账号配置为驱动可直接消费的 map。
func (s *StrmService) strmAccountConfig(acct *model.StrmAccount) (map[string]string, error) {
	cfg := map[string]string{}
	if acct == nil || strings.TrimSpace(acct.Config) == "" {
		return cfg, nil
	}
	if err := json.Unmarshal([]byte(acct.Config), &cfg); err != nil {
		return nil, fmt.Errorf("decode account config: %w", err)
	}
	for _, k := range StrmAccountSecretKeys {
		if v, ok := cfg[k]; ok {
			cfg[k] = s.crypto.Decrypt(v)
		}
	}
	return cfg, nil
}

// HasStrmAccountCredential 报告账号是否已配置核心凭据（用于前端展示）。
func HasStrmAccountCredential(acct *model.StrmAccount) bool {
	switch acct.Provider {
	case model.StrmProvider115:
		// 115 开放平台：已授权（含 access_token）才算配置完成
		return strings.Contains(acct.Config, `"access_token"`)
	case model.StrmProviderOpenList:
		return strings.Contains(acct.Config, `"token"`) || strings.Contains(acct.Config, `"password"`)
	default:
		return strings.Contains(acct.Config, `"password"`) || strings.Contains(acct.Config, `"token"`)
	}
}

// CreateStrmAccount 创建网盘账号（校验提供方 + 凭据）。
func (s *StrmService) CreateStrmAccount(ctx context.Context, name, provider string, config map[string]string) (*model.StrmAccount, error) {
	provider = strings.TrimSpace(provider)
	if provider == "" || provider == model.StrmProviderLocal {
		return nil, errors.New("请选择网盘类型")
	}
	if strings.TrimSpace(name) == "" {
		name = providerLabel(provider)
	}
	enc, err := s.strmAccountConfigJSON(config, true)
	if err != nil {
		return nil, err
	}
	acct := &model.StrmAccount{
		Name:     strings.TrimSpace(name),
		Provider: provider,
		Config:   enc,
		Enabled:  true,
	}
	if err := s.repo.StrmAccount.Create(ctx, acct); err != nil {
		return nil, err
	}
	return acct, nil
}

// UpdateStrmAccount 更新账号；config 为空表示保留原凭据。
func (s *StrmService) UpdateStrmAccount(ctx context.Context, id, name string, enabled *bool, config map[string]string) (*model.StrmAccount, error) {
	acct, err := s.repo.StrmAccount.FindByID(ctx, id)
	if err != nil || acct == nil {
		return nil, errNotFoundOr(err, "网盘账号不存在")
	}
	if strings.TrimSpace(name) != "" {
		acct.Name = strings.TrimSpace(name)
	}
	if enabled != nil {
		acct.Enabled = *enabled
	}
	if len(config) > 0 {
		enc, err := s.strmAccountConfigJSON(config, true)
		if err != nil {
			return nil, err
		}
		acct.Config = enc
	}
	if err := s.repo.StrmAccount.Update(ctx, acct); err != nil {
		return nil, err
	}
	return acct, nil
}

// DeleteStrmAccount 删除账号；仍被同步目录引用时拒绝。
func (s *StrmService) DeleteStrmAccount(ctx context.Context, id string) error {
	paths, err := s.repo.StrmSyncPath.List(ctx)
	if err != nil {
		return err
	}
	for _, p := range paths {
		if p.AccountID == id {
			return fmt.Errorf("该账号仍被同步目录「%s」引用，请先删除对应同步目录", p.Name)
		}
	}
	if err := s.repo.StrmAccount.Delete(ctx, id); err != nil {
		return err
	}
	return nil
}

// TestStrmAccount 连通性测试（Ping），结果写回账号。
func (s *StrmService) TestStrmAccount(ctx context.Context, id string) *model.StrmAccount {
	acct, err := s.repo.StrmAccount.FindByID(ctx, id)
	if err != nil || acct == nil {
		return nil
	}
	now := time.Now()
	acct.LastTestAt = &now
	provider, err := s.providerFor(ctx, acct)
	if err != nil {
		acct.LastTestResult = err.Error()
		acct.LastTestOK = false
	} else if err := provider.Ping(ctx); err != nil {
		acct.LastTestResult = err.Error()
		acct.LastTestOK = false
	} else {
		acct.LastTestResult = "ok"
		acct.LastTestOK = true
	}
	_ = s.repo.StrmAccount.Update(ctx, acct)
	return acct
}

// ListAccounts 返回全部网盘账号。
func (s *StrmService) ListAccounts(ctx context.Context) ([]model.StrmAccount, error) {
	return s.repo.StrmAccount.List(ctx)
}

// providerFor 依据账号配置构建网盘驱动。
func (s *StrmService) providerFor(ctx context.Context, acct *model.StrmAccount) (cloud.Provider, error) {
	cfg, err := s.strmAccountConfig(acct)
	if err != nil {
		return nil, err
	}
	anyCfg := make(map[string]any, len(cfg)+1)
	for k, v := range cfg {
		anyCfg[k] = v
	}
	anyCfg["ua"] = defaultStrmUA
	provider, err := cloud.New(acct.Provider, anyCfg, s.http)
	if err != nil {
		return nil, err
	}
	return provider, nil
}

// ─── 全局设置 ──────────────────────────────────────────────────────────────────

// GetStrmSettings 返回全局 strm 设置（含默认值）。
func (s *StrmService) GetStrmSettings(ctx context.Context) (map[string]string, error) {
	out := map[string]string{}
	for key, def := range StrmSettingDefs {
		value, err := s.repo.Setting.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(value) == "" {
			value = def.Default
		}
		out[key] = value
	}
	return out, nil
}

// UpdateStrmSettings 校验并保存全局 strm 设置。
func (s *StrmService) UpdateStrmSettings(ctx context.Context, values map[string]string) error {
	for key, value := range values {
		def, ok := StrmSettingDefs[key]
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch def.Kind {
		case "number":
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 {
				return fmt.Errorf("%s 必须是正整数", def.Label)
			}
		case "bool":
			if value != "true" && value != "false" {
				return fmt.Errorf("%s 必须是 true/false", def.Label)
			}
		case "choice":
			if !strmContains(def.Choices, value) {
				return fmt.Errorf("%s 取值不合法", def.Label)
			}
		}
		if err := s.repo.Setting.Set(ctx, key, value); err != nil {
			return err
		}
	}
	s.sync115RelayKey(ctx)
	return nil
}

// strmSetting 读取单个 strm 设置（带默认值）。
func (s *StrmService) strmSetting(ctx context.Context, key string) string {
	value, err := s.repo.Setting.Get(ctx, key)
	if err == nil && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	def, ok := StrmSettingDefs[key]
	if ok {
		return def.Default
	}
	return ""
}

func (s *StrmService) strmIntSetting(ctx context.Context, key string, fallback int) int {
	value := s.strmSetting(ctx, key)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return fallback
	}
	return n
}

// ─── 同步目录 ──────────────────────────────────────────────────────────────────

// ListSyncPaths 返回全部同步目录。
func (s *StrmService) ListSyncPaths(ctx context.Context) ([]model.StrmSyncPath, error) {
	return s.repo.StrmSyncPath.List(ctx)
}

// ListSyncRecords 返回同步记录。
func (s *StrmService) ListSyncRecords(ctx context.Context, pathID string, limit int) ([]model.StrmSyncRecord, error) {
	return s.repo.StrmSyncRecord.List(ctx, pathID, limit)
}

// CreateSyncPath 校验并创建同步目录。
func (s *StrmService) CreateSyncPath(ctx context.Context, p *model.StrmSyncPath) (*model.StrmSyncPath, error) {
	if err := s.validateSyncPath(ctx, p); err != nil {
		return nil, err
	}
	if strings.TrimSpace(p.Name) == "" {
		p.Name = "同步目录 " + time.Now().Format("01-02 15:04")
	}
	if p.EnableCron && strings.TrimSpace(p.Cron) == "" {
		return nil, errors.New("启用定时同步需要填写 cron 表达式")
	}
	if err := s.repo.StrmSyncPath.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// UpdateSyncPath 更新同步目录（运行中禁止修改），保留同步状态字段。
func (s *StrmService) UpdateSyncPath(ctx context.Context, id string, p *model.StrmSyncPath) (*model.StrmSyncPath, error) {
	existing, err := s.repo.StrmSyncPath.FindByID(ctx, id)
	if err != nil || existing == nil {
		return nil, errNotFoundOr(err, "同步目录不存在")
	}
	if s.IsSyncRunning(id) {
		return nil, errors.New("该目录正在同步中，请先取消")
	}
	if err := s.validateSyncPath(ctx, p); err != nil {
		return nil, err
	}
	p.ID = existing.ID
	p.CreatedAt = existing.CreatedAt
	p.LastSyncAt = existing.LastSyncAt
	p.LastSyncStatus = existing.LastSyncStatus
	p.LastSyncMessage = existing.LastSyncMessage
	if p.EnableCron && strings.TrimSpace(p.Cron) == "" {
		return nil, errors.New("启用定时同步需要填写 cron 表达式")
	}
	if err := s.repo.StrmSyncPath.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// DeleteSyncPath 删除同步目录（运行中禁止删除）。
func (s *StrmService) DeleteSyncPath(ctx context.Context, id string) error {
	if s.IsSyncRunning(id) {
		return errors.New("该目录正在同步中，请先取消")
	}
	return s.repo.StrmSyncPath.Delete(ctx, id)
}

// ─── 工具 ──────────────────────────────────────────────────────────────────────
func (s *StrmService) validateSyncPath(ctx context.Context, p *model.StrmSyncPath) error {
	p.Provider = strings.TrimSpace(p.Provider)
	if p.Provider == "" {
		return errors.New("请选择同步类型")
	}
	if p.Provider == model.StrmProviderLocal {
		p.AccountID = ""
		if strings.TrimSpace(p.RemotePath) == "" {
			return errors.New("本地同步需要填写源目录")
		}
	} else {
		if strings.TrimSpace(p.AccountID) == "" {
			return errors.New("请选择网盘账号")
		}
		acct, err := s.repo.StrmAccount.FindByID(ctx, p.AccountID)
		if err != nil || acct == nil {
			return errNotFoundOr(err, "网盘账号不存在")
		}
		if acct.Provider != p.Provider {
			return errors.New("网盘账号类型与同步类型不一致")
		}
	}
	if strings.TrimSpace(p.LocalPath) == "" {
		return errors.New("请填写本地输出目录")
	}
	if err := ensureLocalDir(p.LocalPath); err != nil {
		return fmt.Errorf("本地输出目录不可用：%w", err)
	}
	return nil
}

// strmDefaultBaseURL 兜底默认：所有地址配置都为空时使用本机监听地址。
func strmDefaultBaseURL(cfg *config.Config) string {
	port := 8080
	if cfg != nil && cfg.App.Port > 0 {
		port = cfg.App.Port
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

// strmEffectiveConfig 合并全局设置与同步目录覆盖，得出生效配置。
func (s *StrmService) strmEffectiveConfig(ctx context.Context, p *model.StrmSyncPath) (*strmPathConfig, error) {
	cfg := &strmPathConfig{
		BaseURL:     firstNonEmpty(p.StrmBaseURL, s.strmSetting(ctx, StrmSettingBaseURL), PublicServerURL(ctx, s.repo, s.cfg), strmDefaultBaseURL(s.cfg)),
		VideoExt:    csvSplit(firstNonEmpty(p.VideoExt, s.strmSetting(ctx, StrmSettingVideoExt), StrmDefaultVideoExt)),
		MetaExt:     csvSplit(firstNonEmpty(p.MetaExt, s.strmSetting(ctx, StrmSettingMetaExt), StrmDefaultMetaExt)),
		ExcludeName: csvSplit(firstNonEmpty(p.ExcludeName, s.strmSetting(ctx, StrmSettingExcludeName), StrmDefaultExclude)),
		MinSize:     p.MinVideoSizeMB * 1 << 20,
		AddPath:     p.AddPath,
	}
	if cfg.MinSize <= 0 && p.MinVideoSizeMB <= 0 {
		m := s.strmIntSetting(ctx, StrmSettingMinVideoSizeMB, 0)
		cfg.MinSize = int64(m) * 1 << 20
	}
	if cfg.AddPath < 1 || cfg.AddPath > 3 {
		if v, err := strconv.Atoi(s.strmSetting(ctx, StrmSettingAddPath)); err == nil && v >= 1 && v <= 3 {
			cfg.AddPath = v
		} else {
			cfg.AddPath = 1
		}
	}
	// 目录级开关是具体值（前端默认从全局默认值带入），不再叠加全局设置
	cfg.DownloadMeta = p.DownloadMeta
	cfg.UploadMeta = p.UploadMeta
	cfg.DeleteDir = p.DeleteDir
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return cfg, nil
}

func csvSplit(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if p := strings.TrimSpace(strings.ToLower(part)); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func strmContains(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

func errNotFoundOr(err error, msg string) error {
	if err != nil {
		return err
	}
	return errors.New(msg)
}

// providerLabel 提供方中文名（前端同名映射）。
func providerLabel(provider string) string {
	switch provider {
	case model.StrmProvider115:
		return "115 网盘"
	case model.StrmProviderCloudDrive:
		return "CloudDrive2"
	case model.StrmProviderOpenList:
		return "OpenList"
	case model.StrmProviderLocal:
		return "本地目录"
	default:
		return provider
	}
}

// StrmProviderLabels 提供给方的展示标签。
var StrmProviderLabels = map[string]string{
	model.StrmProvider115:        "115 网盘",
	model.StrmProviderCloudDrive: "CloudDrive2",
	model.StrmProviderOpenList:   "OpenList",
	model.StrmProviderLocal:      "本地目录",
}

const defaultStrmUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36 MMTL-Strm/1.0"

// ensureLocalDir 创建本地输出目录。
func ensureLocalDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

// joinLocalRel 拼接本地目标路径并校验不越出根目录。
func joinLocalRel(root, rel string) (string, error) {
	root = filepath.Clean(root)
	target := filepath.Clean(filepath.Join(root, filepath.FromSlash(rel)))
	if target != root && !strings.HasPrefix(target, root+string(filepath.Separator)) {
		return "", errors.New("本地路径越界")
	}
	return target, nil
}

// strmPathConfig 是同步目录的生效配置快照。
type strmPathConfig struct {
	BaseURL      string
	VideoExt     []string
	MetaExt      []string
	ExcludeName  []string
	MinSize      int64
	AddPath      int
	DownloadMeta bool
	UploadMeta   bool
	DeleteDir    bool
}

// ─── 本地目录浏览（添加同步目录用，兼容 Windows/Linux） ─────────────────────────

// StrmLocalDirEntry 是本地目录选择器的一个条目。
type StrmLocalDirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// StrmLocalDirList 是本地目录浏览结果。
type StrmLocalDirList struct {
	Roots    bool                `json:"roots"`             // true=正在显示根/盘符列表
	Parent   string              `json:"parent,omitempty"`  // 上级目录（为空表示没有）
	Current  string              `json:"current,omitempty"` // 当前目录
	Children []StrmLocalDirEntry `json:"children"`
}

// ListStrmLocalDirs 列出本地目录的子目录；path 为空时返回根/盘符列表。
func (s *StrmService) ListStrmLocalDirs(ctx context.Context, path string) (*StrmLocalDirList, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		if isWindows() {
			// 列出存在的盘符
			children := make([]StrmLocalDirEntry, 0, 4)
			for _, letter := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
				vol := string(letter) + ":\\"
				if _, err := os.Stat(vol); err == nil {
					children = append(children, StrmLocalDirEntry{Name: vol, Path: vol})
				}
			}
			if len(children) == 0 {
				children = append(children, StrmLocalDirEntry{Name: "C:\\", Path: "C:\\"})
			}
			return &StrmLocalDirList{Roots: true, Children: children}, nil
		}
		return &StrmLocalDirList{Roots: true, Children: []StrmLocalDirEntry{{Name: "/", Path: "/"}}}, nil
	}

	clean := filepath.Clean(path)
	info, err := os.Stat(clean)
	if err != nil {
		return nil, fmt.Errorf("目录不可访问：%w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("所选路径不是目录")
	}
	entries, err := os.ReadDir(clean)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败：%w", err)
	}
	children := make([]StrmLocalDirEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), ".") && entry.Name() != "." && entry.Name() != ".." {
			continue // 隐藏目录不显示（避免噪音）
		}
		children = append(children, StrmLocalDirEntry{
			Name: entry.Name(),
			Path: filepath.Join(clean, entry.Name()),
		})
	}
	sort.Slice(children, func(i, j int) bool { return children[i].Name < children[j].Name })

	parent := filepath.Dir(clean)
	if parent == clean {
		parent = "" // 已到根/盘符根
	}
	return &StrmLocalDirList{Parent: parent, Current: clean, Children: children}, nil
}

func isWindows() bool {
	return runtime.GOOS == "windows"
}
