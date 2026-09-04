import { useCallback, useEffect, useMemo, useState } from 'react'
import toast from 'react-hot-toast'

import { libraryAPI } from '../api/library'
import type { Library, Media } from '../types'
import { peekLibrary, resolveLibrary } from '../utils/libraryCache'
import { groupSeries, isEpisodeLike, type SeriesCard } from '../utils/groupSeries'

export function useLibraryData(libraryID: string, selectedSeries: SeriesCard | null) {
  const [library, setLibrary] = useState<Library | null>(null)
  const [items, setItems] = useState<Media[]>([])
  const [serverSeriesCards, setServerSeriesCards] = useState<SeriesCard[]>([])
  const [seriesEpisodeItems, setSeriesEpisodeItems] = useState<Media[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [loadingAll, setLoadingAll] = useState(false)
  const [loadingSeriesEpisodes, setLoadingSeriesEpisodes] = useState(false)

  const isSeriesLibrary = isSeriesLibraryType(library?.type)
  const hasEpisodicItems = useMemo(() => items.some(isEpisodeLike), [items])
  const isSeries = isSeriesLibrary || serverSeriesCards.length > 0 || hasEpisodicItems

  const seriesCards = useMemo(() => {
    if (isSeriesLibrary) return serverSeriesCards
    if (!isSeries || items.length === 0) return []
    return groupSeries(items)
  }, [isSeries, isSeriesLibrary, items, serverSeriesCards])

  // reloadCurrentLibrary 通过自增 tick 重跑加载（原实现靠克隆 library 对象
  // 触发第二个 effect，这里合并成单个 bootstrap effect 后改用显式信号）。
  const [reloadTick, setReloadTick] = useState(0)

  useEffect(() => {
    if (!libraryID) return
    let cancelled = false
    setLoading(true)
    setLoadingAll(false)
    setLibrary(null)
    setItems([])
    setServerSeriesCards([])
    setSeriesEpisodeItems([])

    const bootstrap = async () => {
      // 库信息先查会话缓存（首页/全部媒体库页已拉过全量列表），命中则
      // 同步就绪，内容请求在挂载当帧即发出；未命中才退回单独请求，
      // 消除原先"先等库信息、再等内容"的两段串行首屏等待。
      let lib = peekLibrary(libraryID)
      if (lib) setLibrary(lib)
      try {
        const resolved = await resolveLibrary(libraryID)
        if (cancelled) return
        if (!lib) {
          lib = resolved
          setLibrary(lib)
        }
      } catch {
        if (!cancelled) {
          setLoading(false)
          toast.error('媒体库不存在或无权限')
        }
        return
      }

      const seriesLibrary = isSeriesLibraryType(lib.type)
      setLoadingAll(true)
      try {
        if (seriesLibrary) {
          const collected = await loadAllSeriesCards(libraryID, lib.is_remote_emby, (next) => {
            if (cancelled) return
            setTotal(next.total)
            if (next.firstPage) {
              setServerSeriesCards(next.items)
              setLoading(false)
            }
          })
          if (!cancelled) setServerSeriesCards(collected.items)
          return
        }

        const collected = await loadAllMedia(libraryID, lib.is_remote_emby, (next) => {
          if (cancelled) return
          setTotal(next.total)
          if (next.firstPage) {
            setItems(next.items)
            setLoading(false)
          }
        })
        if (!cancelled) setItems(collected.items)
      } catch {
        if (!cancelled) toast.error('媒体库加载失败')
      } finally {
        if (!cancelled) {
          setLoading(false)
          setLoadingAll(false)
        }
      }
    }

    void bootstrap()
    return () => { cancelled = true }
  }, [libraryID, reloadTick])

  useEffect(() => {
    if (!libraryID || !isSeriesLibrary || !selectedSeries) {
      setSeriesEpisodeItems([])
      setLoadingSeriesEpisodes(false)
      return
    }
    let cancelled = false
    setLoadingSeriesEpisodes(true)
    setSeriesEpisodeItems([])
    libraryAPI.listSeriesEpisodes(libraryID, selectedSeries.key)
      .then((r) => {
        if (!cancelled) setSeriesEpisodeItems(r.items ?? [])
      })
      .catch(() => {
        if (!cancelled) toast.error('剧集列表加载失败')
      })
      .finally(() => {
        if (!cancelled) setLoadingSeriesEpisodes(false)
      })
    return () => { cancelled = true }
  }, [libraryID, isSeriesLibrary, selectedSeries])

  const reloadCurrentLibrary = useCallback(() => {
    setReloadTick((tick) => tick + 1)
  }, [])

  const loadingAllText = loadingAll && !loading && (isSeriesLibrary ? total > serverSeriesCards.length : total > items.length)
    ? (isSeriesLibrary
      ? `正在继续加载剧集卡片：${serverSeriesCards.length} / ${total}`
      : `正在继续加载全部条目：${items.length} / ${total}`)
    : ''

  return {
    library,
    items,
    seriesEpisodeItems,
    total,
    loading,
    loadingSeriesEpisodes,
    isSeriesLibrary,
    isSeries,
    seriesCards,
    loadingAllText,
    reloadCurrentLibrary,
  }
}

function isSeriesLibraryType(type?: string) {
  return type === 'tv' || type === 'anime' || type === 'variety'
}

function yieldToBrowser(): Promise<void> {
  return new Promise((resolve) => {
    if (typeof requestIdleCallback !== 'undefined') {
      requestIdleCallback(() => resolve(), { timeout: 48 })
    } else {
      setTimeout(resolve, 0)
    }
  })
}

async function loadAllSeriesCards(
  libraryID: string,
  isRemoteEmby: boolean | undefined,
  onPage: (state: { items: SeriesCard[]; total: number; firstPage: boolean }) => void,
) {
  const pageSize = isRemoteEmby ? 100 : 500
  let page = 1
  let collected: SeriesCard[] = []
  for (;;) {
    const data = await libraryAPI.listSeries(libraryID, page, pageSize)
    // 后端对空库可能返回 items: null（Go nil slice）；不兜底会 concat 出 [null] 并崩溃。
    const pageItems = data.items ?? []
    collected = collected.concat(pageItems)
    onPage({ items: collected, total: data.total ?? collected.length, firstPage: page === 1 })
    if (collected.length >= (data.total ?? 0) || pageItems.length < pageSize) break
    page += 1
    await yieldToBrowser()
  }
  return { items: collected }
}

async function loadAllMedia(
  libraryID: string,
  isRemoteEmby: boolean | undefined,
  onPage: (state: { items: Media[]; total: number; firstPage: boolean }) => void,
) {
  const pageSize = isRemoteEmby ? 100 : 2000
  let page = 1
  let collected: Media[] = []
  for (;;) {
    const data = await libraryAPI.listMedia(libraryID, page, pageSize)
    // 后端对空库可能返回 items: null（Go nil slice）；不兜底会 concat 出 [null] 并崩溃。
    const pageItems = data.items ?? []
    collected = collected.concat(pageItems)
    onPage({ items: collected, total: data.total ?? collected.length, firstPage: page === 1 })
    if (collected.length >= (data.total ?? 0) || pageItems.length < pageSize) break
    page += 1
    await yieldToBrowser()
  }
  return { items: collected }
}
