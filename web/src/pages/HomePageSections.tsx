import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { AnimatePresence, motion } from 'framer-motion'
import {
  ArrowRight,
  ChevronLeft,
  ChevronRight,
  Clock,
  Film,
  FolderOpen,
  Library as LibraryIcon,
  Music,
  Play,
  PlayCircle,
  Sparkles,
  Star,
  Tv,
} from 'lucide-react'

import { imageURL } from '../api/client'
import { MediaCard } from '../components/MediaCard'
import type { HistoryItem } from '../api/playback'
import type { Library, Media } from '../types'
import type { SeriesCard } from '../utils/groupSeries'
import { seriesCardLink } from '../utils/groupSeries'
import { getLibraryArtworks } from './librariesPageModel'

const TYPE_ICONS: Record<string, ReactNode> = {
  movie: <Film size={18} />,
  movies: <Film size={18} />,
  tv: <Tv size={18} />,
  series: <Tv size={18} />,
  anime: <PlayCircle size={18} />,
  shows: <Tv size={18} />,
  variety: <Tv size={18} />,
  music: <Music size={18} />,
  adult: <Film size={18} />,
}

const TYPE_LABELS: Record<string, string> = {
  movie: '电影',
  movies: '电影',
  tv: '剧集',
  series: '剧集',
  anime: '动漫',
  shows: '综艺',
  variety: '综艺',
  music: '音乐',
  adult: 'Adult',
}

export function HomeLoadingState() {
  return (
    <div className="flex items-center justify-center py-48">
      <motion.div
        animate={{ opacity: [0.4, 1, 0.4] }}
        transition={{ repeat: Infinity, duration: 1.5 }}
        className="flex flex-col items-center gap-4"
      >
        <div className="relative flex items-center justify-center">
          <div className="h-10 w-10 animate-spin rounded-full border-2 border-[var(--app-border)] border-t-[var(--app-active-bg)]" />
          <Film className="absolute h-4 w-4 text-brand-500" />
        </div>
        <span className="text-sm font-semibold uppercase tracking-widest text-[var(--app-muted)]">
          首页内容准备中…
        </span>
      </motion.div>
    </div>
  )
}

export function HomeEmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-32 text-center max-w-md mx-auto">
      <div className="mb-6 flex h-24 w-24 items-center justify-center rounded-3xl border border-[var(--app-border)] bg-[var(--app-panel-soft)] shadow-sm">
        <Film className="h-10 w-10 text-[var(--app-muted)]" />
      </div>
      <p className="text-xl font-bold text-[var(--app-text)]">您的家庭影视站暂无内容</p>
      <p className="mt-2 text-sm leading-relaxed text-[var(--app-muted)]">
        前往媒体库添加媒体目录，扫描后首页将展示海报轮播、媒体库和最新入库。
      </p>
      <Link to="/libraries" className="mt-8 btn-primary">
        前往媒体库
      </Link>
    </div>
  )
}

/* =========================================================================
   1. 海报轮播区 (Hero Carousel Section)
   ========================================================================= */

export function HomeCarouselSection({
  items,
  libraryMap,
}: {
  items: Media[]
  libraryMap: Map<string, Library>
}) {
  const [currentIndex, setCurrentIndex] = useState(0)
  const [isPaused, setIsPaused] = useState(false)

  const currentItem = items[currentIndex] || items[0]
  const count = items.length

  useEffect(() => {
    if (count <= 1 || isPaused) return
    const timer = setInterval(() => {
      setCurrentIndex((prev) => (prev + 1) % count)
    }, 5000)
    return () => clearInterval(timer)
  }, [count, isPaused])

  if (!currentItem) return null

  const handlePrev = () => {
    setCurrentIndex((prev) => (prev - 1 + count) % count)
  }

  const handleNext = () => {
    setCurrentIndex((prev) => (prev + 1) % count)
  }

  const visual = currentItem.backdrop_url || currentItem.poster_url || ''
  const poster = currentItem.poster_url || currentItem.backdrop_url || ''
  const lib = libraryMap.get(currentItem.display_library_id || currentItem.library_id)

  return (
    <section
      onMouseEnter={() => setIsPaused(true)}
      onMouseLeave={() => setIsPaused(false)}
      className="group relative overflow-hidden rounded-[2rem] border border-[var(--app-border)] bg-[var(--app-panel)] shadow-[0_24px_80px_var(--app-shadow)] min-h-[420px] md:min-h-[460px] flex flex-col justify-end"
    >
      {/* Background Backdrop Image with Crossfade */}
      <div className="absolute inset-0 z-0">
        <div className="theme-hero-bg h-full w-full" />
        <AnimatePresence mode="wait">
          {visual && (
            <motion.img
              key={visual + currentItem.id}
              initial={{ opacity: 0, scale: 1.08 }}
              animate={{ opacity: 0.38, scale: 1.02 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.8, ease: 'easeOut' }}
              src={imageURL(visual, currentItem.updated_at)}
              alt=""
              className="absolute inset-0 h-full w-full object-cover object-center blur-[1px]"
              referrerPolicy="no-referrer"
              onError={(e) => {
                e.currentTarget.style.display = 'none'
              }}
            />
          )}
        </AnimatePresence>
        <div className="theme-hero-overlay absolute inset-0" />
        <div className="theme-hero-fade absolute inset-x-0 bottom-0 h-36" />
      </div>

      {/* Main Content Layout */}
      <div className="relative z-10 grid gap-8 px-6 py-8 sm:px-8 md:grid-cols-[minmax(0,1fr)_260px] md:px-12 md:py-12 lg:grid-cols-[minmax(0,1fr)_320px] lg:px-14 lg:py-14">
        {/* Left Column: Metadata & Actions */}
        <div className="flex min-w-0 flex-col justify-center space-y-4 sm:space-y-5">
          {/* Tag / Library Label */}
          <div className="flex flex-wrap items-center gap-2">
            <div className="inline-flex w-fit items-center gap-1.5 rounded-full border border-[var(--app-brand-border)] bg-[var(--app-brand-soft)] px-3 py-1 text-xs font-bold uppercase tracking-widest text-[var(--app-brand-text)] shadow-sm backdrop-blur">
              <Sparkles size={12} fill="currentColor" />
              <span>焦点推荐</span>
            </div>
            {lib && (
              <span className="inline-flex items-center gap-1 rounded-full border border-[var(--app-border)] bg-[var(--app-panel)]/80 px-2.5 py-0.5 text-xs font-bold text-[var(--app-text)] shadow-sm backdrop-blur">
                {TYPE_ICONS[lib.type] || <FolderOpen size={12} />}
                <span>{lib.name}</span>
              </span>
            )}
          </div>

          {/* Title */}
          <div className="space-y-2">
            <AnimatePresence mode="wait">
              <motion.h1
                key={currentItem.title + currentItem.id}
                initial={{ opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -6 }}
                transition={{ duration: 0.3 }}
                className="font-display text-2xl font-extrabold leading-tight tracking-tight text-[var(--app-text)] sm:text-3xl md:text-4xl lg:text-5xl"
              >
                {currentItem.title}
              </motion.h1>
            </AnimatePresence>
            {currentItem.original_name && currentItem.original_name !== currentItem.title && (
              <p className="text-xs font-semibold text-[var(--app-muted)] tracking-wide">
                {currentItem.original_name}
              </p>
            )}
          </div>

          {/* Metadata Chips */}
          <div className="flex flex-wrap items-center gap-2.5 text-xs font-bold text-[var(--app-muted)]">
            {currentItem.rating > 0 && (
              <span className="inline-flex items-center gap-1 rounded-xl border border-amber-500/30 bg-amber-500/10 px-2.5 py-1 text-xs font-bold text-amber-500 shadow-sm">
                <Star size={12} fill="currentColor" />
                <span>{currentItem.rating.toFixed(1)}</span>
              </span>
            )}
            {currentItem.year > 0 && (
              <span className="rounded-xl border border-[var(--app-border)] bg-[var(--app-panel)] px-2.5 py-1 text-[var(--app-text)] shadow-sm">
                {currentItem.year} 年
              </span>
            )}
            {currentItem.video_codec && (
              <span className="rounded-lg border border-[var(--app-brand-border)] bg-[var(--app-brand-soft)] px-2 py-1 text-[10px] font-bold uppercase text-[var(--app-brand-text)]">
                {currentItem.video_codec}
              </span>
            )}
            {currentItem.container && (
              <span className="rounded-lg border border-[var(--app-border)] bg-[var(--app-panel)] px-2 py-1 font-mono text-[10px] uppercase text-[var(--app-subtle)]">
                {currentItem.container}
              </span>
            )}
          </div>

          {/* Overview */}
          <p className="line-clamp-3 max-w-2xl text-xs font-semibold leading-relaxed text-[var(--app-subtle)] sm:text-sm">
            {currentItem.overview || '家庭私人媒体中心收藏。支持多端播放、高码率串流与智能刮削。'}
          </p>

          {/* Action Buttons */}
          <div className="flex flex-wrap items-center gap-4 pt-2">
            <Link
              to={`/play/${currentItem.id}`}
              className="inline-flex items-center justify-center gap-2 rounded-xl bg-[var(--app-command-bg)] px-6 py-3.5 text-sm font-bold text-[var(--app-command-text)] shadow-lg transition-all hover:-translate-y-0.5 hover:shadow-brand-500/25"
            >
              <Play size={16} fill="currentColor" />
              <span>立即播放</span>
            </Link>
            <Link
              to={`/media/${currentItem.id}`}
              className="inline-flex items-center justify-center gap-2 rounded-xl border border-[var(--app-border)] bg-[var(--app-panel)] px-5 py-3.5 text-sm font-bold text-[var(--app-subtle)] shadow-sm transition-all hover:-translate-y-0.5 hover:border-brand-500/40 hover:text-[var(--app-text)]"
            >
              <span>查看详情</span>
              <ArrowRight size={16} />
            </Link>
          </div>
        </div>

        {/* Right Column: Floating 3D Poster Card */}
        <div className="relative order-first mx-auto flex w-full max-w-[200px] items-center md:order-none md:max-w-[240px] lg:max-w-[280px]">
          <div className="absolute -right-6 top-5 h-32 w-32 rounded-full bg-brand-500/20 blur-3xl" />
          <div className="relative aspect-[2/3] w-full overflow-hidden rounded-[1.7rem] border border-[var(--app-border)] bg-[var(--app-poster-shell)] p-2 shadow-[0_32px_80px_var(--app-shadow)]">
            <div
              className="flex h-full w-full flex-col items-center justify-center rounded-[1.25rem] text-center"
              style={{ background: 'var(--app-poster-empty)' }}
            >
              <Film className="mb-4 h-12 w-12 text-[#c9954a]" />
              <span className="px-6 font-display text-2xl font-black tracking-tight text-[var(--app-text)]">
                {currentItem.title}
              </span>
            </div>
            {poster && (
              <img
                src={imageURL(poster, currentItem.updated_at)}
                alt={currentItem.title}
                className="absolute inset-2 h-[calc(100%-1rem)] w-[calc(100%-1rem)] rounded-[1.25rem] object-cover"
                referrerPolicy="no-referrer"
                onError={(e) => {
                  e.currentTarget.style.display = 'none'
                }}
              />
            )}
          </div>
        </div>
      </div>

      {/* Carousel Controls & Dots */}
      {count > 1 && (
        <div className="relative z-20 flex items-center justify-between border-t border-[var(--app-border)]/40 bg-[var(--app-panel)]/40 px-6 py-3 backdrop-blur-md">
          {/* Navigation Indicators */}
          <div className="flex items-center gap-1.5">
            {items.map((_, idx) => (
              <button
                key={idx}
                type="button"
                onClick={() => setCurrentIndex(idx)}
                className={`h-2 rounded-full transition-all duration-300 ${
                  idx === currentIndex
                    ? 'w-7 bg-brand-500'
                    : 'w-2 bg-[var(--app-border)] hover:bg-[var(--app-muted)]'
                }`}
                title={`第 ${idx + 1} 张海报`}
              />
            ))}
          </div>

          {/* Slide Number & Arrow Buttons */}
          <div className="flex items-center gap-3">
            <span className="font-mono text-xs font-bold text-[var(--app-muted)]">
              {String(currentIndex + 1).padStart(2, '0')} / {String(count).padStart(2, '0')}
            </span>
            <div className="flex items-center gap-1">
              <button
                type="button"
                onClick={handlePrev}
                className="rounded-xl border border-[var(--app-border)] p-1.5 text-[var(--app-muted)] hover:bg-[var(--app-hover)] hover:text-[var(--app-text)] transition-colors"
                title="上一张"
              >
                <ChevronLeft size={16} />
              </button>
              <button
                type="button"
                onClick={handleNext}
                className="rounded-xl border border-[var(--app-border)] p-1.5 text-[var(--app-muted)] hover:bg-[var(--app-hover)] hover:text-[var(--app-text)] transition-colors"
                title="下一张"
              >
                <ChevronRight size={16} />
              </button>
            </div>
          </div>
        </div>
      )}
    </section>
  )
}

/* =========================================================================
   2. 媒体库卡片区 (Libraries Grid)
   ========================================================================= */

export function HomeLibrariesSection({
  libraries,
  libraryData,
  libraryCounts,
}: {
  libraries: Library[]
  libraryData?: Record<string, { cards: SeriesCard[]; items: Media[]; total: number }>
  libraryCounts: Record<string, number>
}) {
  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--app-border)] pb-3">
        <div className="flex items-center gap-2.5">
          <span className="rounded-xl border border-[var(--app-border)] bg-[var(--app-panel-soft)] p-1.5 text-[var(--app-text)]">
            <LibraryIcon size={18} />
          </span>
          <div>
            <h2 className="font-display text-xl font-extrabold tracking-tight text-[var(--app-text)]">
              媒体库
            </h2>
            <p className="text-xs text-[var(--app-muted)]">
              点击卡片浏览对应媒体库精选内容
            </p>
          </div>
        </div>

        <Link
          to="/libraries"
          className="group inline-flex items-center gap-1 text-xs font-bold text-[var(--app-subtle)] transition-colors hover:text-brand-500"
        >
          <span>全部媒体库</span>
          <ArrowRight size={14} className="transition-transform group-hover:translate-x-0.5" />
        </Link>
      </div>

      {/* Libraries Grid */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-6">
        {libraries.map((lib) => {
          const count = libraryCounts[lib.id] ?? 0
          const cards = libraryData?.[lib.id]?.cards || []
          const artwork = getLibraryArtworks(lib, cards)

          return (
            <Link
              key={lib.id}
              to={`/library/${lib.id}`}
              className="group relative flex flex-col justify-between overflow-hidden rounded-2xl border border-[var(--app-border)] bg-[var(--app-panel)] p-3 transition-all duration-300 hover:-translate-y-1 hover:border-brand-500/50 hover:bg-[var(--app-hover)]/40 hover:shadow-lg hover:shadow-brand-500/10"
            >
              {/* 封面图片展示区：和媒体库页面一样，显示设置好的或生成的图片 */}
              <div
                className={`relative h-28 w-full overflow-hidden rounded-xl bg-[linear-gradient(135deg,var(--app-panel-soft),var(--app-panel))] shadow-inner ${
                  artwork.length > 1 ? 'grid grid-cols-2 gap-0.5' : ''
                }`}
              >
                {artwork.length > 0 ? (
                  artwork.map(({ src, version }, index) => (
                    <img
                      key={`${src}-${index}`}
                      src={imageURL(src, version)}
                      alt=""
                      loading="lazy"
                      referrerPolicy="no-referrer"
                      className="h-full w-full object-cover transition-transform duration-500 group-hover:scale-105"
                      onError={(event) => {
                        event.currentTarget.style.visibility = 'hidden'
                      }}
                    />
                  ))
                ) : (
                  <div className="flex h-full w-full items-center justify-center text-brand-500">
                    {TYPE_ICONS[lib.type] || <FolderOpen size={28} />}
                  </div>
                )}

                {/* 浮动类型标签 */}
                <div className="absolute top-2 right-2 rounded-lg border border-white/20 bg-black/60 px-2 py-0.5 text-[10px] font-bold text-white backdrop-blur-md shadow-sm">
                  {TYPE_LABELS[lib.type] || '自定义'}
                </div>
              </div>

              {/* 媒体库信息 */}
              <div className="mt-3 flex flex-col justify-between">
                <h3 className="truncate font-display text-sm font-bold text-[var(--app-text)] group-hover:text-brand-500">
                  {lib.name}
                </h3>
                <p className="mt-0.5 text-xs text-[var(--app-muted)]">
                  {count > 0 ? `${count} 部媒体` : '暂无条目'}
                </p>
              </div>
            </Link>
          )
        })}
      </div>
    </section>
  )
}

/* =========================================================================
   3. 单个媒体库内容行 (Horizontal Scroll Row)
   ========================================================================= */

export function HomeLibraryRowSection({
  library,
  cards,
}: {
  library: Library
  cards: SeriesCard[]
}) {
  const scrollRef = useRef<HTMLDivElement>(null)

  if (!cards || cards.length === 0) return null

  const scroll = (direction: 'left' | 'right') => {
    if (scrollRef.current) {
      const scrollAmount = direction === 'left' ? -480 : 480
      scrollRef.current.scrollBy({ left: scrollAmount, behavior: 'smooth' })
    }
  }

  return (
    <section className="space-y-4">
      {/* Row Header */}
      <div className="flex items-center justify-between border-b border-[var(--app-border)] pb-3">
        <div className="flex items-center gap-2.5">
          <span className="rounded-xl border border-[var(--app-border)] bg-[var(--app-panel-soft)] p-1.5 text-brand-500">
            {TYPE_ICONS[library.type] || <FolderOpen size={18} />}
          </span>
          <div>
            <h2 className="font-display text-xl font-extrabold tracking-tight text-[var(--app-text)]">
              {library.name}
            </h2>
            <span className="text-xs text-[var(--app-muted)]">
              共 {cards.length} 部精选内容
            </span>
          </div>
        </div>

        <div className="flex items-center gap-3">
          <Link
            to={`/library/${library.id}`}
            className="group inline-flex items-center gap-1 text-xs font-bold text-[var(--app-subtle)] transition-colors hover:text-brand-500"
          >
            <span>查看全部</span>
            <ArrowRight size={14} className="transition-transform group-hover:translate-x-0.5" />
          </Link>

          {/* Scroll Buttons */}
          <div className="hidden sm:flex items-center gap-1">
            <button
              type="button"
              onClick={() => scroll('left')}
              className="rounded-xl border border-[var(--app-border)] p-1.5 text-[var(--app-muted)] hover:bg-[var(--app-hover)] hover:text-[var(--app-text)] transition-colors"
              title="向左滚动"
            >
              <ChevronLeft size={16} />
            </button>
            <button
              type="button"
              onClick={() => scroll('right')}
              className="rounded-xl border border-[var(--app-border)] p-1.5 text-[var(--app-muted)] hover:bg-[var(--app-hover)] hover:text-[var(--app-text)] transition-colors"
              title="向右滚动"
            >
              <ChevronRight size={16} />
            </button>
          </div>
        </div>
      </div>

      {/* Horizontal Carousel Row */}
      <div
        ref={scrollRef}
        className="flex gap-4 overflow-x-auto pb-3 pt-1 no-scrollbar scroll-smooth snap-x"
        style={{ scrollbarWidth: 'none', msOverflowStyle: 'none' }}
      >
        {cards.map((card) => (
          <div
            key={card.key}
            className="w-28 sm:w-32 md:w-36 lg:w-40 shrink-0 snap-start"
          >
            <MediaCard
              media={card.rep}
              count={card.count}
              linkTo={seriesCardLink(card)}
              compact
            />
          </div>
        ))}
      </div>
    </section>
  )
}

/* =========================================================================
   4. 继续观看行 (Continue Watching)
   ========================================================================= */

export function ContinueWatchingSection({ history }: { history: HistoryItem[] }) {
  const scrollRef = useRef<HTMLDivElement>(null)

  const scroll = (direction: 'left' | 'right') => {
    if (scrollRef.current) {
      const scrollAmount = direction === 'left' ? -400 : 400
      scrollRef.current.scrollBy({ left: scrollAmount, behavior: 'smooth' })
    }
  }

  return (
    <section className="space-y-4">
      <div className="flex items-center justify-between border-b border-[var(--app-border)] pb-3">
        <div className="flex items-center gap-2.5">
          <span className="rounded-xl border border-[var(--app-border)] bg-[var(--app-panel-soft)] p-1.5 text-[var(--app-text)]">
            <Clock size={18} />
          </span>
          <div>
            <h2 className="font-display text-xl font-extrabold tracking-tight text-[var(--app-text)]">
              继续观看
            </h2>
            <span className="text-xs text-[var(--app-muted)]">
              {history.length} 条播放记录
            </span>
          </div>
        </div>

        <div className="hidden sm:flex items-center gap-1">
          <button
            type="button"
            onClick={() => scroll('left')}
            className="rounded-xl border border-[var(--app-border)] p-1.5 text-[var(--app-muted)] hover:bg-[var(--app-hover)] hover:text-[var(--app-text)] transition-colors"
            title="向左滚动"
          >
            <ChevronLeft size={16} />
          </button>
          <button
            type="button"
            onClick={() => scroll('right')}
            className="rounded-xl border border-[var(--app-border)] p-1.5 text-[var(--app-muted)] hover:bg-[var(--app-hover)] hover:text-[var(--app-text)] transition-colors"
            title="向右滚动"
          >
            <ChevronRight size={16} />
          </button>
        </div>
      </div>

      <div
        ref={scrollRef}
        className="flex gap-4 overflow-x-auto pb-3 pt-1 no-scrollbar scroll-smooth"
        style={{ scrollbarWidth: 'none', msOverflowStyle: 'none' }}
      >
        {history.slice(0, 12).map((h) => {
          const media = h.media
          if (!media) return null
          const progress = h.duration_ms > 0 ? h.position_ms / h.duration_ms : 0
          return (
            <div key={h.id} className="w-64 sm:w-72 shrink-0">
              <ContinueCard media={media} progress={progress} />
            </div>
          )
        })}
      </div>
    </section>
  )
}

function ContinueCard({ media, progress }: { media: Media; progress: number }) {
  return (
    <Link
      to={`/media/${media.id}`}
      className="group flex items-center gap-3.5 rounded-2xl border border-[var(--app-border)] bg-[var(--app-panel)] p-3 shadow-sm transition-all duration-300 hover:border-brand-500/30 hover:bg-[var(--app-panel-soft)] hover:shadow-md"
    >
      <div className="relative h-20 w-14 shrink-0 overflow-hidden rounded-xl bg-[var(--app-panel-soft)]">
        {media.poster_url ? (
          <img
            src={imageURL(media.poster_url, media.updated_at)}
            alt=""
            className="h-full w-full object-cover transition-transform duration-500 group-hover:scale-105"
            loading="lazy"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-[var(--app-muted)]">
            <Film size={18} />
          </div>
        )}
        <div className="absolute inset-0 flex items-center justify-center bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity">
          <Play size={16} fill="currentColor" className="text-white" />
        </div>
      </div>

      <div className="min-w-0 flex-1 space-y-1">
        <p className="truncate text-xs font-bold text-[var(--app-text)] group-hover:text-brand-500">
          {media.title}
        </p>
        <div className="flex items-center gap-2 text-[10px] text-[var(--app-muted)]">
          {media.year > 0 && <span>{media.year}</span>}
          {media.season_num !== undefined && media.episode_num !== undefined && (
            <span>
              S{media.season_num}E{media.episode_num}
            </span>
          )}
        </div>

        {/* Progress Bar */}
        {progress > 0 && progress < 1 && (
          <div className="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-[var(--app-hover)]">
            <div
              className="h-full bg-brand-500 rounded-full"
              style={{ width: `${Math.round(progress * 100)}%` }}
            />
          </div>
        )}
      </div>
    </Link>
  )
}
