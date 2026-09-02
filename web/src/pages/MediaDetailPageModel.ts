import type { Media } from '../types'
import { mediaLibraryTarget } from '../utils/mediaNavigation'

export function mediaLibraryBackTarget(media: Media): string {
  const seriesTarget = mediaLibraryTarget(media)
  if (seriesTarget) return seriesTarget

  const libraryID = media.display_library_id || media.library_id
  if (!libraryID) return ''
  return `/library/${encodeURIComponent(libraryID)}`
}

export function mediaDetailScrapeMediaType(media: Media): string | undefined {
  return media.season_num > 0 || media.episode_num > 0 ? 'tv' : undefined
}
