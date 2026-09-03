import type { ReactNode } from 'react'
import { Film } from 'lucide-react'

import { MediaCard } from '../components/MediaCard'
import { MEDIA_GRID_CLASS, VirtualMediaGrid } from '../components/VirtualMediaGrid'
import type { Media } from '../types'
import type { SeriesCard } from '../utils/groupSeries'

// 超过该数量后切换为虚拟滚动：DOM 只挂载视口内的卡片，大库不再线性劣化。
const VIRTUALIZE_THRESHOLD = 200

type LibraryMediaSectionsProps = {
  isSeries: boolean
  items: Media[]
  seriesCards: SeriesCard[]
  selectedSeries: SeriesCard | null
  loading: boolean
  cardActions: (media: Media) => ReactNode
  onSeriesClick: (series: SeriesCard) => void
}

export function LibraryMediaSections({
  isSeries,
  items,
  seriesCards,
  selectedSeries,
  loading,
  cardActions,
  onSeriesClick,
}: LibraryMediaSectionsProps) {
  return (
    <>
      {!isSeries && items.length > 0 && (
        <MediaGrid count={items.length} renderItem={(index) => {
          const media = items[index]
          return <MediaCard key={media.id} media={media} actions={cardActions(media)} />
        }} />
      )}

      {!isSeries && items.length === 0 && (
        <LibraryEmptyState message="该媒体库暂无内容，触发一次扫描后再来看看" />
      )}

      {isSeries && seriesCards.length > 0 && !selectedSeries && (
        <MediaGrid count={seriesCards.length} renderItem={(index) => {
          const series = seriesCards[index]
          return (
            <MediaCard
              key={series.key}
              media={series.rep}
              count={series.count}
              actions={cardActions(series.rep)}
              onClick={() => onSeriesClick(series)}
            />
          )
        }} />
      )}

      {isSeries && seriesCards.length === 0 && !loading && (
        <LibraryEmptyState message="该库尚未发现任何剧集，触发一次扫描后再来看看" />
      )}
    </>
  )
}

function MediaGrid({ count, renderItem }: { count: number; renderItem: (index: number) => ReactNode }) {
  if (count <= VIRTUALIZE_THRESHOLD) {
    return <div className={MEDIA_GRID_CLASS}>{Array.from({ length: count }, (_, index) => renderItem(index))}</div>
  }
  return <VirtualMediaGrid totalCount={count} renderItem={renderItem} />
}

function LibraryEmptyState({ message }: { message: string }) {
  return (
    <div className="flex flex-col items-center justify-center py-24 text-center">
      <Film className="mb-4 h-12 w-12 text-gray-500" />
      <p className="text-ink-50">{message}</p>
    </div>
  )
}
