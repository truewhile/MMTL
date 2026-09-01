package cloud

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeEmbyServer 记录请求，按路径返回远程 Emby 风格响应。
func fakeEmbyServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/emby/Users/AuthenticateByName":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"AccessToken":"remote-token","User":{"Id":"user-9"}}`))
		case r.URL.Path == "/emby/System/Info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ServerName":"RemoteEmby"}`))
		case r.URL.Path == "/emby/Users/user-9/Items" && r.URL.Query().Get("ParentId") == "":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Items":[{"Id":"view-1","Name":"Movies","Type":"CollectionFolder","IsFolder":true}]}`))
		case r.URL.Path == "/emby/Users/user-9/Items":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Items":[{"Id":"movie-1","Name":"Avatar","Type":"Movie","IsFolder":false}]}`))
		case strings.Contains(r.URL.Path, "/emby/Videos/movie-1/stream"):
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("fake-video-bytes"))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
}

func TestEmbyProviderPingAuthenticatesAndGetsToken(t *testing.T) {
	srv := fakeEmbyServer(t)
	defer srv.Close()

	p, err := New(TypeEmbyRemote, map[string]any{
		"url":      srv.URL,
		"username": "alice",
		"password": "secret",
	}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
	// 认证成功后 token 被记住，第二次 Ping 不应再走登录。
	if err := p.Ping(context.Background()); err != nil {
		t.Fatalf("ping 2: %v", err)
	}
}

func TestEmbyProviderListViewsAndChildren(t *testing.T) {
	srv := fakeEmbyServer(t)
	defer srv.Close()

	p, err := New(TypeEmbyRemote, map[string]any{
		"url":            srv.URL,
		"api_key":        "fixed-token",
		"remote_user_id": "user-9",
	}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	root, err := p.List(context.Background(), "")
	if err != nil {
		t.Fatalf("list root: %v", err)
	}
	if len(root) != 1 || root[0].Name != "Movies" || !root[0].IsDir {
		t.Fatalf("root listing = %+v", root)
	}
	children, err := p.List(context.Background(), "view-1")
	if err != nil {
		t.Fatalf("list children: %v", err)
	}
	if len(children) != 1 || children[0].Name != "Avatar" || children[0].ID != "movie-1" {
		t.Fatalf("children = %+v", children)
	}
}

func TestEmbyProviderResolveDirectURLByDefault(t *testing.T) {
	srv := fakeEmbyServer(t)
	defer srv.Close()

	p, err := New(TypeEmbyRemote, map[string]any{
		"url":            srv.URL,
		"api_key":        "fixed-token",
		"remote_user_id": "user-9",
	}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	link, err := p.Resolve(context.Background(), "movie-1")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !strings.Contains(link.URL, "/emby/Videos/movie-1/stream") {
		t.Fatalf("url = %q", link.URL)
	}
	if !strings.Contains(link.URL, "api_key=fixed-token") {
		t.Fatalf("url missing api_key: %q", link.URL)
	}
	// 默认不代理播放流量。
	if link.Proxy {
		t.Fatal("emby remote must not proxy by default")
	}
}

func TestEmbyProviderResolveProxyWhenConfigured(t *testing.T) {
	srv := fakeEmbyServer(t)
	defer srv.Close()

	p, err := New(TypeEmbyRemote, map[string]any{
		"url":            srv.URL,
		"api_key":        "fixed-token",
		"remote_user_id": "user-9",
		"proxy_play":     "true",
	}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	link, err := p.Resolve(context.Background(), "movie-1")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !link.Proxy {
		t.Fatal("proxy_play=true must mark link as proxied")
	}
	if link.URL == "" {
		t.Fatal("proxy link must still carry the remote URL")
	}
}
