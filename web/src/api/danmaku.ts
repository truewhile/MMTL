import { api } from './client'

// DanmakuEpisode / DanmakuAnime mirror the dandanplay search response. When
// multiple anime match, the backend returns them as candidates and the player
// asks the user which to load (disambiguation).
export interface DanmakuEpisode {
  episodeId: number
  episodeTitle: string
}

export interface DanmakuAnime {
  animeId: number
  animeTitle: string
  episodes: DanmakuEpisode[]
}

// DanmakuFetchResult mirrors the backend /api/danmaku/:id response: renderer
// knobs (resolved from runtime settings) plus the raw upstream payload. The
// dandanplay protocol delivers Bilibili-format XML or its own JSON shape
// ({comments:[{p,m,t}]}); source_type is sniffed from the body by the backend
// ("auto" when there is nothing to sniff). When several anime matched,
// candidates is non-empty and raw is empty.
export interface DanmakuFetchResult {
  enabled: boolean
  source_type: 'auto' | 'xml' | 'json'
  opacity: string
  font_size: string
  area: string
  raw?: string
  candidates?: DanmakuAnime[]
}

export type DanmakuFetchOptions = {
  /** Overrides the media-derived search keyword (manual search). */
  kw?: string
  /** Forces a specific danmaku library chosen by the user. */
  episodeId?: number | string
}

// danmakuAPI fetches danmaku comments for a media item. The backend resolves
// the configured dandanplay source by the video's name (search for episode,
// then fetch its comment library) and returns the raw Bilibili-format XML;
// parsing into comment objects happens client-side in utils/parseDanmaku.
export const danmakuAPI = {
  fetch: (mediaId: string, options: DanmakuFetchOptions = {}) =>
    api
      .get<DanmakuFetchResult>(`/danmaku/${encodeURIComponent(mediaId)}`, {
        params: { kw: options.kw || undefined, episodeId: options.episodeId || undefined },
        timeout: 20_000,
      })
      .then((r) => r.data),

  // config returns the renderer knobs so the player can initialize its
  // danmaku control panel without admin privileges.
  config: () =>
    api
      .get<{
        enabled: boolean
        source?: string
        opacity: string
        font_size: string
        area: string
      }>('/danmaku/config')
      .then((r) => r.data),
}