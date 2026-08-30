import { useEffect, useMemo, useState } from 'react'
import { useLocation, useParams, useSearchParams } from 'react-router-dom'
import { motion } from 'framer-motion'

import { historyAPI } from '../api/history'
import type { Media } from '../types'
import { useAuthStore } from '../stores/auth'
import type { SeriesCard } from '../utils/groupSeries'
import {
  sortMediaList,
  sortSeriesList,
  type SortField,
  type SortOrder,
} from '../utils/mediaSort'
import { LibraryPageDialogs } from './LibraryPageDialogs'
import { LibraryPageHeader } from './LibraryPageHeader'
import { LibraryMediaSections } from './LibraryMediaSections'
import { LibrarySeriesDetailSection } from './LibrarySeriesDetailSection'
import { useLibraryData } from './useLibraryData'
import { useLibraryScanStatus } from './useLibraryScanStatus'
import { useLibrarySeriesSelection } from './useLibrarySeriesSelection'
import { useLibraryAdminActions } from './useLibraryAdminActions'

export function LibraryPage() {
  const { id = '' } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const location = useLocation()
  const role = useAuthStore((s) => s.user?.role)

  const [scrapeDialogOpen, setScrapeDialogOpen] = useState(false)
  const [manualSeriesScrapeOpen, setManualSeriesScrapeOpen] = useState(false)
  const [seriesMetadataEditOpen, setSeriesMetadataEditOpen] = useState(false)
  const [manualMovie, setManualMovie] = useState<Media | null>(null)

  // 排序状态（支持按库记忆）
  const [sortField, setSortField] = useState<SortField>(() => {
    const saved = localStorage.getItem(`mmtl_lib_sort_field_${id}`) || localStorage.getItem('mmtl_lib_sort_field')
    return (saved as SortField) || 'title'
  })
  const [sortOrder, setSortOrder] = useState<SortOrder>(() => {
    const saved = localStorage.getItem(`mmtl_lib_sort_order_${id}`) || localStorage.getItem('mmtl_lib_sort_order')
    return (saved as SortOrder) || 'asc'
  })
  const [randomSeed, setRandomSeed] = useState(() => Date.now())
  const [historyMap, setHistoryMap] = useState<Map<string, string>>(new Map())

  useEffect(() => {
    historyAPI
      .list(1000)
      .then((historyItems) => {
        const map = new Map<string, string>()
        for (const item of historyItems ?? []) {
          if (item.media_id && item.watched_at) {
            if (!map.has(item.media_id) || new Date(item.watched_at) > new Date(map.get(item.media_id)!)) {
              map.set(item.media_id, item.watched_at)
            }
          }
        }
        setHistoryMap(map)
      })
      .catch(() => {})
  }, [])

  const handleSortChange = (field: SortField, order: SortOrder) => {
    setSortField(field)
    setSortOrder(order)
    if (id) {
      localStorage.setItem(`mmtl_lib_sort_field_${id}`, field)
      localStorage.setItem(`mmtl_lib_sort_order_${id}`, order)
    }
    localStorage.setItem('mmtl_lib_sort_field', field)
    localStorage.setItem('mmtl_lib_sort_order', order)
    if (field === 'random') {
      setRandomSeed(Date.now())
    }
  }

  const handleReshuffle = () => {
    setRandomSeed(Date.now())
  }

  // 剧集模式：选中某个剧集后展开详情
  const [selectedSeries, setSelectedSeries] = useState<SeriesCard | null>(null)
  const [selectedSeason, setSelectedSeason] = useState<number | null>(null)

  const {
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
  } = useLibraryData(id, selectedSeries)

  const sortedItems = useMemo(() => {
    return sortMediaList(items, sortField, sortOrder, randomSeed, historyMap)
  }, [items, sortField, sortOrder, randomSeed, historyMap])

  const sortedSeriesCards = useMemo(() => {
    return sortSeriesList(seriesCards, sortField, sortOrder, randomSeed, historyMap)
  }, [seriesCards, sortField, sortOrder, randomSeed, historyMap])

  const {
    scanning,
    scanProgress,
    handleScan,
  } = useLibraryScanStatus({
    libraryID: id,
    isAdmin: role === 'admin',
    onLibraryChanged: reloadCurrentLibrary,
  })

  const {
    selectedEpisodes,
    visibleEpisodes,
    selectedSeriesEpisodes,
    selectedSeriesMediaIDs,
    handleSeriesClick,
    clearSelectedSeries,
  } = useLibrarySeriesSelection({
    items,
    seriesEpisodeItems,
    isSeriesLibrary,
    isSeries,
    loading,
    seriesCards: sortedSeriesCards,
    searchParams,
    setSearchParams,
    selectedSeries,
    setSelectedSeries,
    selectedSeason,
    setSelectedSeason,
    onClearSeriesState: () => setSeriesMetadataEditOpen(false),
  })

  const {
    scraping,
    scrapeEpisodeArtwork,
    repairing,
    seriesToolBusy,
    setScrapeEpisodeArtwork,
    handleRepairRescrape,
    handleSeriesSmartScrape,
    handleSeriesProbe,
    handleSeriesNFO,
    handleSeriesOrganize,
    handleSeriesSoftDelete,
    movieActions,
  } = useLibraryAdminActions({
    libraryID: id,
    role,
    library,
    selectedSeries,
    selectedSeriesEpisodes,
    reloadCurrentLibrary,
    clearSelectedSeries,
    setManualMovie,
  })

  if (loading) {
    return (
      <div className="flex items-center justify-center py-32">
        <motion.div animate={{ opacity: [0.4, 1, 0.4] }} transition={{ repeat: Infinity, duration: 2 }} className="flex items-center gap-3">
          <div className="h-2 w-2 rounded-full bg-brand-500" />
          <span className="text-sm text-sand-500">加载中…</span>
        </motion.div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {!selectedSeries && (
        <LibraryPageHeader
          library={library}
          itemCount={isSeries ? sortedSeriesCards.length : total}
          loadingAllText={loadingAllText}
          scanProgress={scanProgress}
          isAdmin={role === 'admin'}
          scrapeEpisodeArtwork={scrapeEpisodeArtwork}
          scanning={scanning}
          scraping={scraping}
          repairing={repairing}
          sortField={sortField}
          sortOrder={sortOrder}
          onSortChange={handleSortChange}
          onReshuffle={handleReshuffle}
          onScrapeEpisodeArtworkChange={setScrapeEpisodeArtwork}
          onScan={handleScan}
          onScrape={() => setScrapeDialogOpen(true)}
          onRepairRescrape={handleRepairRescrape}
        />
      )}

      <LibraryMediaSections
        isSeries={isSeries}
        items={sortedItems}
        seriesCards={sortedSeriesCards}
        selectedSeries={selectedSeries}
        loading={loading}
        movieActions={movieActions}
        onSeriesClick={handleSeriesClick}
      />

      <LibrarySeriesDetailSection
        selectedSeries={selectedSeries}
        selectedEpisodes={selectedEpisodes}
        selectedSeason={selectedSeason}
        visibleEpisodes={visibleEpisodes}
        allEpisodes={selectedSeriesEpisodes}
        loadingEpisodes={loadingSeriesEpisodes}
        playbackFrom={`${location.pathname}${location.search}`}
        isAdmin={role === 'admin'}
        seriesToolBusy={seriesToolBusy}
        onBack={clearSelectedSeries}
        onSmartScrape={handleSeriesSmartScrape}
        onManualScrape={() => setManualSeriesScrapeOpen(true)}
        onMetadataEdit={() => setSeriesMetadataEditOpen(true)}
        onProbe={handleSeriesProbe}
        onNFO={handleSeriesNFO}
        onOrganize={handleSeriesOrganize}
        onSoftDelete={handleSeriesSoftDelete}
        onSeasonChange={setSelectedSeason}
      />

      <LibraryPageDialogs
        scrapeDialogOpen={scrapeDialogOpen}
        library={library}
        manualSeriesScrapeOpen={manualSeriesScrapeOpen}
        seriesMetadataEditOpen={seriesMetadataEditOpen}
        manualMovie={manualMovie}
        selectedSeries={selectedSeries}
        selectedSeriesMediaIDs={selectedSeriesMediaIDs}
        libraryType={library?.type}
        scrapeEpisodeArtwork={scrapeEpisodeArtwork}
        onScrapeEpisodeArtworkChange={setScrapeEpisodeArtwork}
        onCloseScrapeDialog={() => setScrapeDialogOpen(false)}
        onCloseManualSeriesScrape={() => setManualSeriesScrapeOpen(false)}
        onCloseSeriesMetadataEdit={() => setSeriesMetadataEditOpen(false)}
        onCloseManualMovie={() => setManualMovie(null)}
        onApplied={reloadCurrentLibrary}
      />
    </div>
  )
}
