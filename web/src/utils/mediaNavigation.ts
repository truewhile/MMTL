import type { Media } from '../types'
import { isRemoteEmbyID } from './remoteEmby'
import { getSeriesKey, isEpisodeLike, pathLooksEpisodic } from './groupSeries'

/** Whether opening the library series detail view is a better target than /media/:id. */
export function prefersLibrarySeriesView(media: Media): boolean {
  if (isEpisodeLike(media)) return true
  if (pathLooksEpisodic(media)) return true
  if (media.series_id) return true
  return false
}

/** Series key used by /library/:id?series=... for both local and remote libraries. */
export function resolveLibrarySeriesKey(media: Media): string {
  if (isRemoteEmbyID(media.id) && !isEpisodeLike(media)) {
    return media.id
  }
  return getSeriesKey(media)
}

/** Best library URL for a media item, or null when /media/:id is preferred. */
export function mediaLibraryTarget(media: Media): string | null {
  const libraryID = media.display_library_id || media.library_id
  if (!libraryID || !prefersLibrarySeriesView(media)) return null

  const seriesKey = resolveLibrarySeriesKey(media)
  const base = `/library/${encodeURIComponent(libraryID)}`
  return seriesKey ? `${base}?series=${encodeURIComponent(seriesKey)}` : base
}

/** Link target for favourites / cards that should open the library series view when possible. */
export function favouriteMediaLink(media: Media): string {
  return mediaLibraryTarget(media) ?? `/media/${media.id}`
}
