import { useEffect, useMemo, useState } from 'react'

import { libraryAPI, type LibraryWithPreview } from '../api/library'
import { historyAPI } from '../api/history'
import type { HistoryItem } from '../api/playback'
import type { Library, Media } from '../types'
import type { SeriesCard } from '../utils/groupSeries'
import {
  ContinueWatchingSection,
  HomeCarouselSection,
  HomeEmptyState,
  HomeLibrariesSection,
  HomeLibraryRowSection,
  HomeLoadingState,
} from './HomePageSections'

const hasArtwork = (media?: Media | null) => !!(media?.poster_url || media?.backdrop_url)
const asArray = <T,>(value: unknown): T[] => (Array.isArray(value) ? (value as T[]) : [])

export function HomePage() {
  const [libraries, setLibraries] = useState<Library[]>([])
  const [libraryData, setLibraryData] = useState<Record<string, { cards: SeriesCard[]; items: Media[]; total: number }>>({})
  const [history, setHistory] = useState<HistoryItem[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false

    async function load() {
      setLoading(true)
      try {
        const [libs, hist] = await Promise.all([
          libraryAPI
            .list({ withPreview: true, previewLimit: 20 })
            .then((rows) => asArray<LibraryWithPreview>(rows))
            .catch(() => [] as LibraryWithPreview[]),
          historyAPI
            .continueWatching(12)
            .then((rows) =>
              rows.map(
                (row): HistoryItem => ({
                  ...row.history,
                  created_at: '',
                  updated_at: '',
                  media: row.media,
                }),
              ),
            )
            .catch(() => [] as HistoryItem[]),
        ])

        if (cancelled) return
        setLibraries(libs)
        setHistory(hist.filter((h) => h && !h.completed))

        const mapData: Record<string, { cards: SeriesCard[]; items: Media[]; total: number }> = {}
        for (const lib of libs) {
          mapData[lib.id] = {
            cards: lib.cards ?? [],
            items: [],
            total: lib.total ?? 0,
          }
        }
        setLibraryData(mapData)
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    load()
    return () => {
      cancelled = true
    }
  }, [])

  // Quick lookup map for libraries
  const libraryMap = useMemo(() => {
    const map = new Map<string, Library>()
    for (const lib of libraries) {
      map.set(lib.id, lib)
    }
    return map
  }, [libraries])

  // Counts lookup
  const libraryCounts = useMemo(() => {
    const counts: Record<string, number> = {}
    for (const lib of libraries) {
      counts[lib.id] = libraryData[lib.id]?.total ?? 0
    }
    return counts
  }, [libraries, libraryData])

  // Compute items to show in the Hero Carousel. Only libraries flagged
  // carousel_enabled contribute; a library's items array is empty for
  // series-type libs (loaded via /series), so gate on cards instead.
  const carouselItems = useMemo(() => {
    const candidateMedia: Media[] = []
    const effectiveSelectedIds = libraries
      .filter((l) => l.carousel_enabled === true)
      .map((l) => l.id)

    for (const libId of effectiveSelectedIds) {
      const data = libraryData[libId]
      if (data && data.cards.length > 0) {
        for (const card of data.cards) {
          if (hasArtwork(card.rep)) {
            candidateMedia.push(card.rep)
          }
        }
      }
    }

    // Fallback to all loaded items with artwork
    if (candidateMedia.length === 0) {
      for (const lib of libraries) {
        const data = libraryData[lib.id]
        if (data) {
          for (const card of data.cards) {
            candidateMedia.push(card.rep)
          }
        }
      }
    }

    return candidateMedia.slice(0, 10)
  }, [libraries, libraryData])

  const empty =
    !loading &&
    libraries.length === 0 &&
    history.length === 0

  if (loading) {
    return <HomeLoadingState />
  }

  if (empty) {
    return <HomeEmptyState />
  }

  return (
    <div className="space-y-12 pb-16">
      {/* 1. 顶部海报轮播区 */}
      {carouselItems.length > 0 && (
        <HomeCarouselSection items={carouselItems} libraryMap={libraryMap} />
      )}

      {/* 2. 继续观看（若有历史） */}
      {history.length > 0 && <ContinueWatchingSection history={history} />}

      {/* 3. 媒体库卡片区 */}
      {libraries.length > 0 && (
        <HomeLibrariesSection
          libraries={libraries}
          libraryData={libraryData}
          libraryCounts={libraryCounts}
        />
      )}

      {/* 4. 各媒体库内容展示行 */}
      <div className="space-y-10">
        {libraries.map((lib) => {
          const cards = libraryData[lib.id]?.cards || []
          if (cards.length === 0) return null
          return (
            <HomeLibraryRowSection
              key={lib.id}
              library={lib}
              cards={cards}
            />
          )
        })}
      </div>
    </div>
  )
}
