package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ShukeBta/MMTL/internal/model"
	"github.com/ShukeBta/MMTL/internal/service/cloud115"
)

// Test115OAuthQRFlow 用 mock 的 115 开放平台接口走完设备码扫码授权：
// 创建账号（空凭据）→ 发起授权（取二维码）→ 轮询（未扫码/已扫码/已确认）
// → 确认后 token 写入账号 → 驱动可用。
func Test115OAuthQRFlow(t *testing.T) {
	svc := testStrmService(t)

	var scanCalls int
	pro := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open/user/info":
			w.Write([]byte(`{"state":true,"data":{"user_id":"115001","user_name":"测试用户"}}`))
		default:
			t.Errorf("unexpected pro path %s", r.URL.Path)
		}
	}))
	defer pro.Close()
	passport := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open/authDeviceCode":
			w.Write([]byte(`{"state":true,"data":{"uid":"U1","time":1700,"sign":"S1","qrcode":"https://img/qr.png"}}`))
		case "/open/deviceCodeToToken":
			w.Write([]byte(`{"state":true,"data":{"access_token":"open-at","refresh_token":"open-rt","expires_in":7200}}`))
		default:
			t.Errorf("unexpected passport path %s", r.URL.Path)
		}
	}))
	defer passport.Close()
	qr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scanCalls++
		switch scanCalls {
		case 1:
			w.Write([]byte(`{"state":true,"data":{"status":0}}`))
		case 2:
			w.Write([]byte(`{"state":true,"data":{"status":1}}`))
		default:
			w.Write([]byte(`{"state":true,"data":{"status":2}}`))
		}
	}))
	defer qr.Close()

	oldPro, oldPassport, oldQR := cloud115.ProAPIBase, cloud115.PassportAPIBase, cloud115.QRCodeAPIBase
	cloud115.ProAPIBase, cloud115.PassportAPIBase, cloud115.QRCodeAPIBase = pro.URL, passport.URL, qr.URL
	defer func() {
		cloud115.ProAPIBase, cloud115.PassportAPIBase, cloud115.QRCodeAPIBase = oldPro, oldPassport, oldQR
	}()

	// 1. 创建 115 账号（自定义 AppID，空凭据）
	acct, err := svc.CreateStrmAccount(context.Background(), "我的115", model.StrmProvider115, map[string]string{})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if HasStrmAccountCredential(acct) {
		t.Fatalf("new account should have no credential")
	}

	// 2. 发起设备码授权
	result, err := svc.Start115OAuth(context.Background(), acct.ID,
		string(cloud115.SourceTypeCustomAppID), "100195125", "", "")
	if err != nil {
		t.Fatalf("start oauth: %v", err)
	}
	if result.Mode != "qrcode" || result.QRCode == nil || result.QRCode.Qrcode == "" {
		t.Fatalf("bad start result: %#v", result)
	}

	// 3. 轮询：waiting -> scanned
	for _, want := range []string{"waiting", "scanned"} {
		status, err := svc.Poll115OAuth(context.Background(), result.SessionID)
		if err != nil {
			t.Fatalf("poll: %v", err)
		}
		if status.Status != want {
			t.Fatalf("poll status = %s, want %s", status.Status, want)
		}
	}

	// 4. 确认
	status, err := svc.Poll115OAuth(context.Background(), result.SessionID)
	if err != nil {
		t.Fatalf("poll confirm: %v", err)
	}
	if status.Status != "confirmed" {
		t.Fatalf("final status = %s, want confirmed", status.Status)
	}

	// 5. 账号已保存 token 与用户信息
	updated, err := svc.repo.StrmAccount.FindByID(context.Background(), acct.ID)
	if err != nil || updated == nil {
		t.Fatal("account missing")
	}
	cfg, err := svc.strmAccountConfig(updated)
	if err != nil {
		t.Fatal(err)
	}
	if cfg["access_token"] != "open-at" || cfg["refresh_token"] != "open-rt" {
		t.Fatalf("token not saved: %#v", cfg)
	}
	if cfg["user_name"] != "测试用户" && !strings.Contains(updated.Name, "115") && updated.Name != "测试用户" {
		t.Fatalf("user info not saved: %#v", cfg)
	}
	if !HasStrmAccountCredential(updated) {
		t.Fatal("credential should be present after auth")
	}
}

// Test115OAUTHCallbackRelay 中继回调链路：发起 → 回调（加密 payload）→ 轮询确认。
func Test115OAUTHCallbackRelay(t *testing.T) {
	svc := testStrmService(t)
	cloud115.RelayEncryptionKey = "callback-test-key"
	defer func() { cloud115.RelayEncryptionKey = "" }()

	acct, err := svc.CreateStrmAccount(context.Background(), "中继115", model.StrmProvider115, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.Start115OAuth(context.Background(), acct.ID,
		string(cloud115.SourceTypeBuiltInRelay), "QMediaSync", string(cloud115.ProviderQMediaSync),
		"http://127.0.0.1/api/strm/oauth/callback")
	if err != nil {
		t.Fatalf("start relay oauth: %v", err)
	}
	if result.Mode != "url" || result.AuthURL == "" {
		t.Fatalf("bad relay start: %#v", result)
	}
	if !strings.Contains(result.AuthURL, "oauth.qmediasync.cn") {
		t.Fatalf("unexpected relay url: %s", result.AuthURL)
	}

	// 模拟中继服务器回调（加密 data + authorization_id）
	payload := `{"data":{"access_token":"relay-at","refresh_token":"relay-rt","expires_in":7200}}`
	encrypted, err := cloud115.EncryptRelay(payload)
	if err != nil {
		t.Fatal(err)
	}
	cbErr := svc.Handle115OAuthCallback(context.Background(), map[string]string{
		"authorization_id": result.SessionID,
		"data":             encrypted,
	})
	if cbErr != nil {
		t.Fatalf("callback: %v", cbErr)
	}

	// 轮询确认
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status, err := svc.Poll115OAuth(context.Background(), result.SessionID)
		if err != nil {
			t.Fatalf("poll: %v", err)
		}
		if status.Status == "confirmed" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	updated, err := svc.repo.StrmAccount.FindByID(context.Background(), acct.ID)
	if err != nil || updated == nil {
		t.Fatal("account missing")
	}
	cfg, _ := svc.strmAccountConfig(updated)
	if cfg["access_token"] != "relay-at" {
		t.Fatalf("relay token not saved: %#v", cfg)
	}
}

// Test115TokenRefreshLoop 令牌刷新与失效处理。
func Test115TokenRefreshLoop(t *testing.T) {
	svc := testStrmService(t)
	var refreshCalls int
	passport := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/open/refreshToken" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		refreshCalls++
		if strings.Contains(r.PostFormValue("refresh_token"), "dead") {
			w.Write([]byte(`{"state":false,"code":40140119,"message":"已过期"}`))
			return
		}
		w.Write([]byte(`{"state":true,"data":{"access_token":"new-at","refresh_token":"new-rt","expires_in":7200}}`))
	}))
	defer passport.Close()
	old := cloud115.PassportAPIBase
	cloud115.PassportAPIBase = passport.URL
	defer func() { cloud115.PassportAPIBase = old }()

	cfg := map[string]string{
		"app_id": "100195125", "access_token": "at1", "refresh_token": "rt1",
	}
	acct, err := svc.CreateStrmAccount(context.Background(), "刷新测试", model.StrmProvider115, cfg)
	if err != nil {
		t.Fatal(err)
	}
	svc.refresh115TokensOnce(context.Background())
	updated, _ := svc.repo.StrmAccount.FindByID(context.Background(), acct.ID)
	updatedCfg, _ := svc.strmAccountConfig(updated)
	if updatedCfg["access_token"] != "new-at" || updatedCfg["refresh_token"] != "new-rt" {
		t.Fatalf("token not refreshed: %#v", updatedCfg)
	}

	// 失效的 refresh_token：access_token 清空 + 标记失败
	deadCfg := map[string]string{"app_id": "100195125", "access_token": "at-d", "refresh_token": "rt-dead"}
	deadAcct, err := svc.CreateStrmAccount(context.Background(), "失效测试", model.StrmProvider115, deadCfg)
	if err != nil {
		t.Fatal(err)
	}
	svc.refresh115TokensOnce(context.Background())
	deadUpdated, _ := svc.repo.StrmAccount.FindByID(context.Background(), deadAcct.ID)
	deadUpdatedCfg, _ := svc.strmAccountConfig(deadUpdated)
	if deadUpdatedCfg["access_token"] != "" {
		t.Fatalf("dead account access_token should be cleared: %#v", deadUpdatedCfg)
	}
	if deadUpdated.LastTestOK {
		t.Fatal("dead account should be marked failed")
	}
}
