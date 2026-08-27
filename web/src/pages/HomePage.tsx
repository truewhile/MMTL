import { useEffect, useMemo, useState } from 'react'

import { libraryAPI } from '../api/library'
import { playbackAPI, type HistoryItem } from '../api/playback'
import type { Library, Media } from '../types'
import { groupSeries, type SeriesCard } from '../utils/groupSeries'
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
          libraryAPI.list().then((rows) => asArray<Library>(rows)).catch(() => [] as Library[]),
          playbackAPI.recentHistory().then((rows) => asArray<HistoryItem>(rows)).catch(() => [] as HistoryItem[]),
        ])

        if (cancelled) return
        setLibraries(libs)
        setHistory(hist.filter((h) => h && !h.completed && !!h.media))

        // Fetch media items for all libraries in parallel
        const isSeriesType = (type?: string) => type === 'tv' || type === 'anime' || type === 'variety'
        const results = await Promise.allSettled(
          libs.map(async (lib) => {
            // 剧集类媒体库（tv/anime/variety）：后端 /series 已按剧聚合，
            // 首页若用 episode 级 /media 的前 30 行再 groupSeries，同一部剧的
            // 多集会折叠成 1 张卡，导致整行只显示 1 个条目。
            // 改用 /series 分页拉取全部聚合后的剧集卡片。
            if (isSeriesType(lib.type)) {
              const cards: SeriesCard[] = []
              let total = 0
              const pageSize = 200
              for (let page = 1; page <= 10; page++) {
                const data = await libraryAPI.listSeries(lib.id, page, pageSize)
                const pageItems = asArray<SeriesCard>(data?.items)
                cards.push(...pageItems)
                total = data?.total ?? cards.length
                if (cards.length >= total || pageItems.length < pageSize) break
              }
              return { id: lib.id, cards, items: [], total }
            }
            const page = await libraryAPI.listMedia(lib.id, 1, 30)
            const items = asArray<Media>(page?.items)
            const cards = groupSeries(items)
            return {
              id: lib.id,
              cards,
              items,
              total: page?.total ?? items.length,
            }
          }),
        )

        if (cancelled) return
        const mapData: Record<string, { cards: SeriesCard[]; items: Media[]; total: number }> = {}
        for (const res of results) {
          if (res.status === 'fulfilled' && res.value) {
            mapData[res.value.id] = {
              cards: res.value.cards,
              items: res.value.items,
              total: res.value.total,
            }
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
      .filter((l) => l.carousel_enabled !== false)
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
