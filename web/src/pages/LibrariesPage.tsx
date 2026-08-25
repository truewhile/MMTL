import { useCallback, useEffect, useMemo, useState } from 'react'

import { libraryAPI } from '../api/library'
import { toolsAPI } from '../api/tools'
import { openManageLibrariesDialog } from '../components/manageLibrariesDialog'
import {
  LibrariesContent,
  LibrariesEmptyState,
  LibrariesHeader,
} from './LibrariesPageSections'
import { isSeriesLibraryType, latestLibraryCards, type LibraryPreview } from './librariesPageModel'

export function LibrariesPage() {
  const [previews, setPreviews] = useState<LibraryPreview[]>([])
  const [loading, setLoading] = useState(true)
  const [repairing, setRepairing] = useState(false)
  const [repairEpisodeArtwork, setRepairEpisodeArtwork] = useState(false)
  const [repairMsg, setRepairMsg] = useState('')

  const loadLibraries = useCallback(async () => {
    setLoading(true)
    try {
      const libs = await libraryAPI.list()
      const rows = await Promise.all(libs.map(async (library) => {
        try {
          if (isSeriesLibraryType(library.type)) {
            const [seriesPage, mediaPage] = await Promise.all([
              libraryAPI.listSeries(library.id, 1, 10),
              libraryAPI.listMedia(library.id, 1, 1, { groupVersions: false }),
            ])
            return { library, items: [], total: mediaPage.total, cards: seriesPage.items ?? [] } satisfies LibraryPreview
          }
          const page = await libraryAPI.listMedia(library.id, 1, 160, { groupVersions: false })
          const cards = latestLibraryCards(page.items)
          return { library, items: page.items, total: page.total, cards } satisfies LibraryPreview
        } catch {
          return { library, items: [], total: 0, cards: [] } satisfies LibraryPreview
        }
      }))
      setPreviews(rows)
    } finally {
      setLoading(false)
    }
  }, [])

  async function handleRepairRescrape() {
    if (repairing) return
    setRepairing(true)
    setRepairMsg('')
    try {
      await toolsAPI.repairAndRescrapeAll({ episode_images: repairEpisodeArtwork, refresh_matched: true })
      setRepairMsg('已开始全库修复+重刮，进度可在任务中查看。')
    } catch {
      setRepairMsg('启动失败，请稍后重试。')
    } finally {
      setRepairing(false)
    }
  }

  const handleManageLibraries = async () => {
    await openManageLibrariesDialog()
    await loadLibraries()
  }

  useEffect(() => {
    loadLibraries().catch(() => undefined)
  }, [loadLibraries])

  const total = useMemo(() => previews.reduce((sum, preview) => sum + preview.total, 0), [previews])

  if (loading) {
    return <p className="px-2 py-8 text-sm text-sand-500">媒体库加载中…</p>
  }

  return (
    <div className="space-y-8">
      <LibrariesHeader
        previewCount={previews.length}
        total={total}
        repairMsg={repairMsg}
        repairEpisodeArtwork={repairEpisodeArtwork}
        repairing={repairing}
        onRepairEpisodeArtworkChange={setRepairEpisodeArtwork}
        onRepairRescrape={handleRepairRescrape}
        onManageLibraries={handleManageLibraries}
      />

      {previews.length === 0 ? (
        <LibrariesEmptyState />
      ) : (
        <LibrariesContent previews={previews} />
      )}
    </div>
  )
}
