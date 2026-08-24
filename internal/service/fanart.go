// Package service — Fanart.tv image provider.
//
// Fanart.tv (https://fanart.tv) hosts community-curated artwork. We use
// it to upgrade movie / tv posters and backdrops with higher-resolution
// alternatives once a TMDb / Bangumi match is established.
//
// Endpoints used:
//
//	GET /v3/movies/{tmdb_id}        (artwork keyed by TMDb id)
//	GET /v3/tv/{thetvdb_id}         (artwork keyed by TheTVDB id)
//
// The provider is enabled iff secrets.fanart_tv_api_key is non-empty.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/ShukeBta/MMTL/internal/config"
)

// FanartProvider talks to https://webservice.fanart.tv.
type FanartProvider struct {
	cfg    *config.Config
	log    *zap.Logger
	client *http.Client
}

// NewFanartProvider is the constructor.
func NewFanartProvider(cfg *config.Config, log *zap.Logger) *FanartProvider {
	return &FanartProvider{
		cfg:    cfg,
		log:    log,
		client: NewExternalHTTPClient(15 * time.Second),
	}
}

// Enabled reports whether an API key is configured.
func (f *FanartProvider) Enabled() bool { return f.cfg.Secrets.FanartAPIKey != "" }

// Artwork is the high-res image set returned by Fanart.tv.
type Artwork struct {
	Poster   string `json:"poster"`
	Backdrop string `json:"backdrop"`
	Logo     string `json:"logo"`
	Thumb    string `json:"thumb"`
}

// MovieArtwork looks up the artwork bundle for a TMDb movie id.
func (f *FanartProvider) MovieArtwork(ctx context.Context, tmdbID int) (*Artwork, error) {
	if !f.Enabled() || tmdbID <= 0 {
		return nil, nil
	}
	type entry struct {
		URL  string `json:"url"`
		Lang string `json:"lang"`
	}
	type page struct {
		MoviePoster []entry `json:"movieposter"`
		MovieBackgr []entry `json:"moviebackground"`
		HDLogo      []entry `json:"hdmovielogo"`
		MovieThumb  []entry `json:"moviethumb"`
	}
	u := fmt.Sprintf("https://webservice.fanart.tv/v3/movies/%d?api_key=%s",
		tmdbID, f.cfg.Secrets.FanartAPIKey)
	var p page
	if err := f.getJSON(ctx, u, &p); err != nil {
		return nil, err
	}
	a := &Artwork{}
	if len(p.MoviePoster) > 0 {
		a.Poster = p.MoviePoster[0].URL
	}
	if len(p.MovieBackgr) > 0 {
		a.Backdrop = p.MovieBackgr[0].URL
	}
	if len(p.HDLogo) > 0 {
		a.Logo = p.HDLogo[0].URL
	}
	if len(p.MovieThumb) > 0 {
		a.Thumb = p.MovieThumb[0].URL
	}
	return a, nil
}

// TVArtwork looks up artwork for a TheTVDB series id.
func (f *FanartProvider) TVArtwork(ctx context.Context, theTVDBID string) (*Artwork, error) {
	if !f.Enabled() || theTVDBID == "" {
		return nil, nil
	}
	type entry struct {
		URL  string `json:"url"`
		Lang string `json:"lang"`
	}
	type page struct {
		TVPoster  []entry `json:"tvposter"`
		ShowBack  []entry `json:"showbackground"`
		HDLogo    []entry `json:"hdtvlogo"`
		ClearLogo []entry `json:"clearlogo"`
		TVThumb   []entry `json:"tvthumb"`
	}
	u := fmt.Sprintf("https://webservice.fanart.tv/v3/tv/%s?api_key=%s",
		theTVDBID, f.cfg.Secrets.FanartAPIKey)
	var p page
	if err := f.getJSON(ctx, u, &p); err != nil {
		return nil, err
	}
	a := &Artwork{}
	if len(p.TVPoster) > 0 {
		a.Poster = p.TVPoster[0].URL
	}
	if len(p.ShowBack) > 0 {
		a.Backdrop = p.ShowBack[0].URL
	}
	if len(p.HDLogo) > 0 {
		a.Logo = p.HDLogo[0].URL
	} else if len(p.ClearLogo) > 0 {
		a.Logo = p.ClearLogo[0].URL
	}
	if len(p.TVThumb) > 0 {
		a.Thumb = p.TVThumb[0].URL
	}
	return a, nil
}

func (f *FanartProvider) getJSON(ctx context.Context, u string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "MMTL/0.1")
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("fanart %s: %d", u, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
