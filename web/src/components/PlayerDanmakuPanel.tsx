import { useEffect, useState } from 'react'
import { Check, ChevronRight, Loader2, MessageSquareText, Search, X } from 'lucide-react'

import type { DanmakuAnime, DanmakuEpisode } from '../api/danmaku'

// PlayerDanmakuPanel — the on-player danmaku control panel. It toggles
// loading, lets the user re-search by a custom keyword, and adjusts the
// renderer knobs (display area / opacity / font size) live. Values are
// controlled by PlayerPage so the panel is purely presentational.
//
// The knobs change immediately on drag; a "re-search" only fires when the
// user clicks the search button (or presses Enter) so sliders don't trigger
// network requests. When the backend returns multiple anime candidates
// (disambiguation), the panel shows the picker so the user can choose the
// right danmaku library by hand — mirroring danmaku-anywhere's selector.
type PlayerDanmakuPanelProps = {
  open: boolean
  onClose: () => void
  enabled: boolean
  onToggleEnabled: (v: boolean) => void
  search: string
  onSearch: (kw: string) => void
  searching: boolean
  area: number
  onAreaChange: (v: number) => void
  opacity: number
  onOpacityChange: (v: number) => void
  fontSize: number
  onFontSizeChange: (v: number) => void
  /** Multiple anime matched — user must pick one. */
  candidates: DanmakuAnime[]
  /** Human-readable label of the currently selected library. */
  selectedSource: string
  onSelectEpisode: (episodeId: number, animeTitle: string, episodeTitle: string) => void
  onResetAuto: () => void
}

export function PlayerDanmakuPanel({
  open,
  onClose,
  enabled,
  onToggleEnabled,
  search,
  onSearch,
  searching,
  area,
  onAreaChange,
  opacity,
  onOpacityChange,
  fontSize,
  onFontSizeChange,
  candidates,
  selectedSource,
  onSelectEpisode,
  onResetAuto,
}: PlayerDanmakuPanelProps) {
  const [draft, setDraft] = useState(search)
  // 展开的番剧（动画 → 集数两级树），默认全展开便于选择。
  const [openAnime, setOpenAnime] = useState<Set<number>>(new Set())

  // 面板打开时同步外部搜索词到输入框草稿，并展开全部候选。
  useEffect(() => {
    if (open) {
      setDraft(search)
      setOpenAnime(new Set(candidates.map((c) => c.animeId)))
    }
  }, [open, search, candidates])

  if (!open) return null

  const toggleAnime = (animeId: number) => {
    setOpenAnime((prev) => {
      const next = new Set(prev)
      if (next.has(animeId)) {
        next.delete(animeId)
      } else {
        next.add(animeId)
      }
      return next
    })
  }

  return (
    // 面板悬浮于视频上方：阻止点击冒泡，避免触发视频区域的播放/暂停切换。
    <div
      onClick={(e) => e.stopPropagation()}
      className="absolute right-4 top-16 z-30 w-72 rounded-2xl border border-white/15 bg-black/80 p-4 text-white shadow-2xl backdrop-blur"
    >
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-2 text-sm font-semibold">
          <MessageSquareText size={16} /> 弹幕设置
        </div>
        <button
          onClick={onClose}
          className="rounded-full p-1 text-white/60 transition hover:bg-white/10 hover:text-white"
          title="关闭"
        >
          <X size={16} />
        </button>
      </div>

      {/* 是否加载弹幕 */}
      <label className="mb-3 flex cursor-pointer items-center justify-between text-sm">
        <span className="text-white/85">加载弹幕</span>
        <input
          type="checkbox"
          checked={enabled}
          onChange={(e) => onToggleEnabled(e.target.checked)}
          className="h-4 w-4 accent-rose-500"
        />
      </label>

      {/* 搜索弹幕 */}
      <div className="mb-4">
        <div className="mb-1 text-xs text-white/60">搜索弹幕（留空 = 按视频名）</div>
        <div className="flex items-center gap-1">
          <input
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') onSearch(draft.trim())
            }}
            placeholder="输入番剧名…"
            className="min-w-0 flex-1 rounded-lg border border-white/15 bg-white/5 px-2.5 py-1.5 text-sm outline-none placeholder:text-white/35 focus:border-rose-400/60"
          />
          <button
            onClick={() => onSearch(draft.trim())}
            disabled={searching}
            className="flex items-center gap-1 rounded-lg bg-rose-500/90 px-2.5 py-1.5 text-xs font-medium transition hover:bg-rose-500 disabled:opacity-50"
          >
            {searching ? <Loader2 size={13} className="animate-spin" /> : <Search size={13} />}
            搜索
          </button>
        </div>
      </div>

      {/* 当前来源 / 候选手动选取 */}
      {selectedSource && (
        <div className="mb-3 flex items-center justify-between rounded-lg border border-white/10 bg-white/5 px-2.5 py-1.5 text-xs">
          <span className="min-w-0 truncate text-white/75" title={selectedSource}>
            当前: {selectedSource}
          </span>
          <button
            onClick={onResetAuto}
            className="ml-2 shrink-0 text-rose-300 transition hover:text-rose-200"
            title="清除手动选择，恢复自动匹配"
          >
            自动
          </button>
        </div>
      )}

      {candidates.length > 0 && (
        <div className="mb-4 rounded-lg border border-amber-400/25 bg-amber-400/5 p-2">
          <div className="mb-1.5 px-1 text-xs font-medium text-amber-200">
            搜到多部番剧，请选择弹幕来源：
          </div>
          <div className="max-h-52 overflow-y-auto pr-1">
            {candidates.map((anime, i) => (
              <div key={anime.animeId} className="mb-1">
                <button
                  onClick={() => toggleAnime(anime.animeId)}
                  className="flex w-full items-center gap-1 rounded-md px-1.5 py-1 text-left text-sm text-white/85 transition hover:bg-white/10"
                >
                  <ChevronRight
                    size={13}
                    className={
                      'shrink-0 transition-transform ' +
                      (openAnime.has(anime.animeId) ? 'rotate-90' : '')
                    }
                  />
                  <span className="min-w-0 flex-1 truncate">
                    {anime.animeTitle}
                    {anime.animeTitle === '' && `番剧 ${i + 1}`}
                  </span>
                  <span className="shrink-0 text-[10px] text-white/40">
                    {anime.episodes.length} 集
                  </span>
                </button>
                {openAnime.has(anime.animeId) && (
                  <div className="ml-5 border-l border-white/10 pl-2">
                    {anime.episodes.map((ep) => (
                      <EpisodeRow
                        key={ep.episodeId}
                        episode={ep}
                        onSelect={() =>
                          onSelectEpisode(ep.episodeId, anime.animeTitle, ep.episodeTitle)
                        }
                      />
                    ))}
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* 屏幕占比（显示区域） */}
      <SliderRow
        label="屏幕占比"
        value={area}
        min={0.1}
        max={1}
        step={0.05}
        format={(v) => `${Math.round(v * 100)}%`}
        onChange={onAreaChange}
      />
      {/* 透明度 */}
      <SliderRow
        label="透明度"
        value={opacity}
        min={0.1}
        max={1}
        step={0.05}
        format={(v) => `${Math.round(v * 100)}%`}
        onChange={onOpacityChange}
      />
      {/* 字体大小 */}
      <SliderRow
        label="字体大小"
        value={fontSize}
        min={14}
        max={48}
        step={1}
        format={(v) => `${Math.round(v)}px`}
        onChange={onFontSizeChange}
      />
    </div>
  )
}

function SliderRow({
  label,
  value,
  min,
  max,
  step,
  format,
  onChange,
}: {
  label: string
  value: number
  min: number
  max: number
  step: number
  format: (v: number) => string
  onChange: (v: number) => void
}) {
  return (
    <div className="mb-3">
      <div className="mb-1 flex items-center justify-between text-xs">
        <span className="text-white/60">{label}</span>
        <span className="font-mono text-white/85">{format(value)}</span>
      </div>
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className="w-full accent-rose-500"
      />
    </div>
  )
}

function EpisodeRow({
  episode,
  onSelect,
}: {
  episode: DanmakuEpisode
  onSelect: () => void
}) {
  return (
    <button
      onClick={onSelect}
      className="flex w-full items-center gap-1.5 rounded-md px-1.5 py-1 text-left text-xs text-white/70 transition hover:bg-rose-500/20 hover:text-white"
      title={`选择《${episode.episodeTitle}》的弹幕`}
    >
      <Check size={12} className="shrink-0 text-rose-400" />
      <span className="min-w-0 flex-1 truncate">{episode.episodeTitle}</span>
    </button>
  )
}