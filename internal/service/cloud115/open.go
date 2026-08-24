// 115 开放平台只读 API：列目录、详情、下载直链、授权（二维码/换 token/刷新）。
package cloud115

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ─── 文件模型 ──────────────────────────────────────────────────────────────────

type FileType string

const (
	TypeFile FileType = "1"
	TypeDir  FileType = "0"
)

// RemoteFile 是 115 文件列表中的一个条目。
type RemoteFile struct {
	FileId   string   `json:"fid"`  // 文件 ID
	Pid      string   `json:"pid"`  // 父文件夹 ID
	Category FileType `json:"fc"`   // 0 文件夹 1 文件
	FileName string   `json:"fn"`   // 文件名
	PickCode string   `json:"pc"`   // 提取码
	Utime    int64    `json:"upt"`  // 修改时间
	Ptime    int64    `json:"uppt"` // 上传时间
	Sha1     string   `json:"sha1"` // SHA1
	FileSize int64    `json:"fs"`   // 大小
	Fta      string   `json:"fta"`  // 0/2 未上传完成，1 已完成
}

func (f RemoteFile) ModifiedAt() int64 {
	if f.Utime > 0 {
		return f.Utime
	}
	return f.Ptime
}

type FileListResp struct {
	RespBase
	Path []struct {
		Name   string `json:"name"`
		FileId string `json:"cid"`
	} `json:"path"`
	PathStr string `json:"path_str"`
}

// GetFsList 列目录（cid=0 为根目录）。
func (c *OpenClient) GetFsList(ctx context.Context, cid string, offset, limit int) ([]RemoteFile, string, error) {
	if cid == "" {
		cid = "0"
	}
	params := map[string]string{"cid": cid}
	if limit > 0 {
		params["limit"] = fmt.Sprint(limit)
	}
	if offset > 0 {
		params["offset"] = fmt.Sprint(offset)
	}
	params["cur"] = "1"
	params["show_dir"] = "1"
	resp, err := c.doAuthJSON(ctx, "GET", ProAPIBase+"/open/ufile/files", params, 2)
	if err != nil {
		return nil, "", err
	}
	// state=false（token 过期、限流、业务失败等）绝不能当作空目录返回：
	// 同步流程会据此认为远端已清空并清理本地 .strm/元数据文件。
	if !resp.State {
		return nil, "", NewOpenAPIResponseError(resp.Code, resp.Errno, resp.Message, resp.Error, "115 接口调用失败")
	}
	var list FileListResp
	list.RespBase = *resp
	if len(resp.Raw) > 0 {
		_ = json.Unmarshal(resp.Raw, &list)
	}
	files, err := openList[RemoteFile](resp.Data)
	if err != nil {
		return nil, "", fmt.Errorf("115: 解析文件列表失败：%w", err)
	}
	pathStr := make([]string, 0, len(list.Path))
	for _, item := range list.Path {
		if item.FileId == "0" {
			continue
		}
		pathStr = append(pathStr, item.Name)
	}
	return files, strings.Join(pathStr, "/"), nil
}

// GetFsDetailByCid 查询文件（夹）详情。
func (c *OpenClient) GetFsDetailByCid(ctx context.Context, fileId string) (*RemoteFileDetail, error) {
	params := map[string]string{"file_id": fileId}
	resp, err := c.doAuthJSON(ctx, "GET", ProAPIBase+"/open/folder/get_info", params, 2)
	if err != nil {
		return nil, err
	}
	return openFirstList[RemoteFileDetail](resp.Data)
}

// RemoteFileDetail 是文件详情。
type RemoteFileDetail struct {
	FileId   string   `json:"file_id"`
	FileName string   `json:"file_name"`
	PickCode string   `json:"pick_code"`
	Sha1     string   `json:"sha1"`
	Category FileType `json:"file_category"`
	SizeByte int64    `json:"size_byte"`
	Paths    []struct {
		FileId string `json:"file_id"`
		Name   string `json:"file_name"`
	} `json:"paths"`
}

// ─── 下载直链 ──────────────────────────────────────────────────────────────────

type downloadURLData struct {
	FileName string `json:"file_name"`
	PickCode string `json:"pick_code"`
	Sha1     string `json:"sha1"`
	URL      struct {
		URL string `json:"url"`
	} `json:"url"`
}

// GetDownloadURL 获取下载直链（pickcode）。命中缓存直接返回，
// 避免对同一文件反复换取直链触发 115 风控。
func (c *OpenClient) GetDownloadURL(ctx context.Context, pickCode string) (string, error) {
	return c.GetDownloadURLWithUA(ctx, pickCode, "")
}

// GetDownloadURLWithUA 支持按调用方/播放器 User-Agent 换取对应的 115 CDN 直链（用于 115 防盗链白名单校验）。
func (c *OpenClient) GetDownloadURLWithUA(ctx context.Context, pickCode, ua string) (string, error) {
	ua = strings.TrimSpace(ua)
	if cached := GetDownloadURLCache(pickCode, ua); cached != "" {
		return cached, nil
	}
	params := map[string]string{"pick_code": pickCode}
	resp, err := c.doAuthJSONWithUA(ctx, "POST", ProAPIBase+"/open/ufile/downurl", params, 1, ua)
	if err != nil {
		return "", err
	}
	var data map[string]downloadURLData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return "", fmt.Errorf("115: 解析下载地址失败：%w", err)
	}
	first := firstOrEmpty(data)
	if first.URL.URL == "" {
		return "", fmt.Errorf("115: 下载地址为空（文件可能未上传完成或已被删除）")
	}
	SetDownloadURLCache(pickCode, first.URL.URL, ua)
	return first.URL.URL, nil
}

// ─── 授权（设备码扫码） ──────────────────────────────────────────────────────

// QrCodeScanStatus 扫码状态。
type QrCodeScanStatus int

const (
	QrCodeScanStatusExpired    QrCodeScanStatus = 5
	QrCodeScanStatusNotScanned QrCodeScanStatus = 2
	QrCodeScanStatusScanned    QrCodeScanStatus = 3
	QrCodeScanStatusConfirmed  QrCodeScanStatus = 4
)

func (s QrCodeScanStatus) String() string {
	switch s {
	case QrCodeScanStatusNotScanned:
		return "waiting"
	case QrCodeScanStatusScanned:
		return "scanned"
	case QrCodeScanStatusConfirmed:
		return "confirmed"
	default:
		return "expired"
	}
}

func (s QrCodeScanStatus) Tip() string {
	switch s {
	case QrCodeScanStatusNotScanned:
		return "等待扫码"
	case QrCodeScanStatusScanned:
		return "已扫码，请在 115 客户端确认"
	case QrCodeScanStatusConfirmed:
		return "授权成功"
	default:
		return "二维码已过期"
	}
}

// QrCodeData 是设备码二维码数据。
type QrCodeData struct {
	Uid    string `json:"uid"`
	Time   int64  `json:"time"`
	Sign   string `json:"sign"`
	Qrcode string `json:"qrcode"` // 二维码图片 URL
}

// QrCodeDataReturn 含 PKCE code_verifier。
type QrCodeDataReturn struct {
	QrCodeData
	CodeVerifier string `json:"code_verifier"`
}

// TokenData 是 115 访问令牌。
type TokenData struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// GetQrCode 获取设备码登录二维码。
func (c *OpenClient) GetQrCode() (*QrCodeDataReturn, error) {
	codeVerifier := RandomString(64)
	params := map[string]string{
		"client_id":             c.AppID,
		"code_challenge":        genCodeChallenge(codeVerifier),
		"code_challenge_method": "sha256",
	}
	resp, err := c.doJSON(context.Background(), "POST", PassportAPIBase+"/open/authDeviceCode", params, false, 1)
	if err != nil {
		return nil, err
	}
	code, err := openFirstList[QrCodeData](resp.Data)
	if err != nil {
		return nil, err
	}
	return &QrCodeDataReturn{QrCodeData: *code, CodeVerifier: codeVerifier}, nil
}

// QrCodeScanStatus 查询扫码状态。
func (c *OpenClient) QrCodeScanStatus(codeData *QrCodeData) (QrCodeScanStatus, error) {
	if codeData == nil {
		return QrCodeScanStatusExpired, fmt.Errorf("空二维码数据")
	}
	params := map[string]string{
		"uid":  codeData.Uid,
		"time": fmt.Sprint(codeData.Time),
		"sign": codeData.Sign,
	}
	resp, err := c.doJSON(context.Background(), "GET", QRCodeAPIBase+"/get/status/", params, false, 1)
	if err != nil {
		return QrCodeScanStatusExpired, err
	}
	status, err := openFirstList[struct {
		Status int `json:"status"` // 0 未扫码 1 已扫码 2 已确认
	}](resp.Data)
	if err != nil {
		return QrCodeScanStatusExpired, err
	}
	switch status.Status {
	case 1:
		return QrCodeScanStatusScanned, nil
	case 2:
		return QrCodeScanStatusConfirmed, nil
	case 0:
		return QrCodeScanStatusNotScanned, nil
	default:
		return QrCodeScanStatusExpired, nil
	}
}

// GetToken 用设备码换访问令牌。
func (c *OpenClient) GetToken(qrCode *QrCodeDataReturn) (*TokenData, error) {
	if qrCode == nil || qrCode.Uid == "" {
		return nil, fmt.Errorf("空二维码数据")
	}
	params := map[string]string{
		"uid":           qrCode.Uid,
		"code_verifier": qrCode.CodeVerifier,
	}
	resp, err := c.doJSON(context.Background(), "POST", PassportAPIBase+"/open/deviceCodeToToken", params, false, 0)
	if err != nil {
		return nil, err
	}
	token, err := openFirstList[TokenData](resp.Data)
	if err != nil {
		return nil, err
	}
	c.SetAuthToken(token.AccessToken, token.RefreshToken)
	return token, nil
}

// RefreshToken 刷新访问令牌。
func (c *OpenClient) RefreshToken(refreshToken string) (*TokenData, error) {
	if refreshToken == "" {
		refreshToken = c.RefreshTokenStr
	}
	if refreshToken == "" {
		return nil, fmt.Errorf("没有可用的 refresh_token")
	}
	params := map[string]string{"refresh_token": refreshToken}
	resp, err := c.doJSON(context.Background(), "POST", PassportAPIBase+"/open/refreshToken", params, false, 0)
	if err != nil && resp == nil {
		return nil, err
	}
	if resp == nil {
		return nil, err
	}
	if !resp.State {
		apiErr := NewOpenAPIResponseError(resp.Code, resp.Errno, resp.Message, resp.Error, "115 开放平台刷新访问凭证失败")
		if IsRefreshTokenDead(apiErr) {
			c.SetAuthToken("", "")
		}
		return nil, apiErr
	}
	token, err := openFirstList[TokenData](resp.Data)
	if err != nil {
		return nil, err
	}
	c.SetAuthToken(token.AccessToken, token.RefreshToken)
	return token, nil
}

// ─── 用户信息 ──────────────────────────────────────────────────────────────────

// UserInfo 是 115 用户信息。
type UserInfo struct {
	UserId   json.Number `json:"user_id"`
	UserName string      `json:"user_name"`
}

// FetchUserInfo 获取用户信息。
func (c *OpenClient) FetchUserInfo(ctx context.Context) (*UserInfo, error) {
	resp, err := c.doAuthJSON(ctx, "GET", ProAPIBase+"/open/user/info", nil, 1)
	if err != nil {
		return nil, err
	}
	var info UserInfo
	if err := json.Unmarshal(resp.Data, &info); err != nil {
		return nil, fmt.Errorf("115: 解析用户信息失败：%w", err)
	}
	return &info, nil
}
