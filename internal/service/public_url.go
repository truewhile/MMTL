package service

import (
	"context"
	"net/url"
	"strings"

	"github.com/ShukeBta/MMTL/internal/config"
	"github.com/ShukeBta/MMTL/internal/repository"
)

// PublicServerURL returns the operator-configured public MMTL base
// URL. It is intentionally read from Setting first so Docker users only need to
// fill one domain in the admin settings page after deployment.
func PublicServerURL(ctx context.Context, repo *repository.Container, cfg *config.Config) string {
	for _, key := range []string{"app.server_url", "server.url", "public.server_url", "strm.base_url"} {
		if repo != nil && repo.Setting != nil {
			if value, err := repo.Setting.Get(ctx, key); err == nil && strings.TrimSpace(value) != "" {
				return strings.TrimRight(strings.TrimSpace(value), "/")
			}
		}
	}
	if cfg != nil && strings.TrimSpace(cfg.App.ServerURL) != "" {
		return strings.TrimRight(strings.TrimSpace(cfg.App.ServerURL), "/")
	}
	return ""
}

// BuildPublicAPIURL builds an API URL. Without a public base it returns a
// relative same-origin URL; with a configured domain it returns an absolute URL.
func BuildPublicAPIURL(ctx context.Context, repo *repository.Container, cfg *config.Config, apiPath string, query url.Values) string {
	apiPath = "/" + strings.TrimLeft(strings.TrimSpace(apiPath), "/")
	if query != nil && len(query) > 0 {
		apiPath += "?" + query.Encode()
	}
	base := PublicServerURL(ctx, repo, cfg)
	if base == "" {
		return apiPath
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return apiPath
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(strings.Split(apiPath, "?")[0], "/")
	if query != nil && len(query) > 0 {
		u.RawQuery = query.Encode()
	}
	return u.String()
}
