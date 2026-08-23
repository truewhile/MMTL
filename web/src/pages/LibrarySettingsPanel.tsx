import { useEffect, useState } from 'react'
import {
  Check,
  CheckSquare,
  Film,
  FolderOpen,
  HeartHandshake,
  Layers,
  Loader2,
  Music,
  Save,
  SlidersHorizontal,
  Square,
  Tv,
} from 'lucide-react'
import toast from 'react-hot-toast'

import { adminAPI } from '../api/admin'
import { imageURL } from '../api/client'
import { libraryAPI } from '../api/library'
import type { Library, Setting } from '../types'
import { groupSeries, type SeriesCard } from '../utils/groupSeries'
import { getLibraryArtworks } from './librariesPageModel'

const CAROUSEL_STORAGE_KEY = 'mmtl.home.carousel_libraries'
const SETTING_KEY_CAROUSEL = 'home.carousel_libraries'

const TYPE_ICONS: Record<string, React.ReactNode> = {
  movie: <Film size={20} />,
  movies: <Film size={20} />,
  tv: <Tv size={20} />,
  series: <Tv size={20} />,
  anime: <Layers size={20} />,
  shows: <Tv size={20} />,
  variety: <Tv size={20} />,
  music: <Music size={20} />,
  adult: <HeartHandshake size={20} />,
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

export function LibrarySettingsPanel() {
  const [libraries, setLibraries] = useState<Library[]>([])
  const [libraryCards, setLibraryCards] = useState<Record<string, SeriesCard[]>>({})
  const [selectedIds, setSelectedIds] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [dirty, setDirty] = useState(false)

  useEffect(() => {
    async function load() {
      setLoading(true)
      try {
        const [libs, settings] = await Promise.all([
          libraryAPI.list({ includeHidden: true }).catch(() => [] as Library[]),
          adminAPI.listSettings().catch(() => [] as Setting[]),
        ])

        const libList = Array.isArray(libs) ? libs : []
        setLibraries(libList)

        // 异步拉取各个媒体库的前几个条目用于封面展示
        Promise.allSettled(
          libList.map(async (lib) => {
            const page = await libraryAPI.listMedia(lib.id, 1, 10)
            const items = Array.isArray(page?.items) ? page.items : []
            return { id: lib.id, cards: groupSeries(items) }
          }),
        ).then((results) => {
          const map: Record<string, SeriesCard[]> = {}
          for (const r of results) {
            if (r.status === 'fulfilled' && r.value) {
              map[r.value.id] = r.value.cards
            }
          }
          setLibraryCards(map)
        })

        // 优先读取系统配置，其次读取 localStorage，默认全选
        const settingItem = Array.isArray(settings)
          ? settings.find((s) => s.key === SETTING_KEY_CAROUSEL)
          : undefined

        let initialIds: string[] | null = null
        if (settingItem?.value) {
          try {
            initialIds = JSON.parse(settingItem.value)
          } catch {
            // ignore
          }
        }

        if (!initialIds) {
          try {
            const saved = localStorage.getItem(CAROUSEL_STORAGE_KEY)
            if (saved) initialIds = JSON.parse(saved)
          } catch {
            // ignore
          }
        }

        if (Array.isArray(initialIds)) {
          setSelectedIds(initialIds)
        } else {
          setSelectedIds(libList.map((l) => l.id))
        }
      } finally {
        setLoading(false)
      }
    }

    load()
  }, [])

  const toggleLibrary = (id: string) => {
    setSelectedIds((prev) => {
      const next = prev.includes(id) ? prev.filter((item) => item !== id) : [...prev, id]
      setDirty(true)
      return next
    })
  }

  const selectAll = () => {
    setSelectedIds(libraries.map((l) => l.id))
    setDirty(true)
  }

  const deselectAll = () => {
    setSelectedIds([])
    setDirty(true)
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      const jsonValue = JSON.stringify(selectedIds)
      await adminAPI.updateSetting(SETTING_KEY_CAROUSEL, jsonValue)
      try {
        localStorage.setItem(CAROUSEL_STORAGE_KEY, jsonValue)
      } catch {
        // ignore
      }
      toast.success('海报轮播设置已保存')
      setDirty(false)
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        '保存设置失败'
      toast.error(msg)
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return (
      <div className="flex justify-center py-12 text-ink-50">
        <Loader2 className="animate-spin" />
      </div>
    )
  }

  return (
    <div className="glass-panel space-y-6">
      {/* 头部说明与快捷操作 */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between border-b border-gray-200/80 pb-4">
        <div className="flex items-start gap-3">
          <div className="rounded-xl border border-primary-400/40 bg-primary-400/10 p-2 text-brand-500 mt-0.5">
            <SlidersHorizontal size={20} />
          </div>
          <div>
            <h3 className="font-display text-lg font-bold text-ink-600">首页海报轮播设置</h3>
            <p className="text-xs text-sand-500 mt-0.5">
              选择参与首页顶部大图海报轮播推荐的媒体库。勾选的媒体库内容将轮流展示在系统首页顶部。
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2 self-end sm:self-auto">
          <button
            type="button"
            onClick={selectAll}
            className="flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-xs font-semibold text-ink-100 transition hover:border-primary-400/50 hover:text-brand-500"
          >
            <CheckSquare size={14} />
            <span>全选</span>
          </button>
          <button
            type="button"
            onClick={deselectAll}
            className="flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-xs font-semibold text-ink-100 transition hover:border-primary-400/50 hover:text-brand-500"
          >
            <Square size={14} />
            <span>清空</span>
          </button>
        </div>
      </div>

      {/* 媒体库列表卡片 */}
      {libraries.length === 0 ? (
        <div className="py-8 text-center text-xs text-sand-500">
          暂无可用媒体库，请先添加媒体库后再配置海报轮播。
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {libraries.map((lib) => {
            const isSelected = selectedIds.includes(lib.id)
            const cards = libraryCards[lib.id] || []
            const artwork = getLibraryArtworks(lib, cards)

            return (
              <div
                key={lib.id}
                onClick={() => toggleLibrary(lib.id)}
                className={`flex cursor-pointer items-center justify-between rounded-2xl border p-4 transition-all duration-200 select-none ${
                  isSelected
                    ? 'border-brand-500/60 bg-primary-400/10 shadow-sm'
                    : 'border-gray-200 bg-white/70 hover:border-gray-300'
                }`}
              >
                <div className="flex items-center gap-3 min-w-0">
                  <div
                    className={`grid h-12 w-16 shrink-0 gap-0.5 overflow-hidden rounded-xl bg-gray-100 shadow-inner transition-colors ${
                      artwork.length > 1 ? 'grid-cols-2' : 'grid-cols-1'
                    } ${
                      isSelected ? 'ring-2 ring-brand-500/30' : ''
                    }`}
                  >
                    {artwork.length > 0 ? (
                      artwork.map(({ src, version }, index) => (
                        <img
                          key={`${src}-${index}`}
                          src={imageURL(src, version)}
                          alt=""
                          className="h-full w-full object-cover"
                          referrerPolicy="no-referrer"
                          onError={(e) => {
                            e.currentTarget.style.display = 'none'
                          }}
                        />
                      ))
                    ) : (
                      <div className="flex h-full w-full items-center justify-center text-ink-50">
                        {TYPE_ICONS[lib.type] || <FolderOpen size={20} />}
                      </div>
                    )}
                  </div>
                  <div className="min-w-0">
                    <div className="flex items-center gap-1.5">
                      <span className="truncate font-display text-sm font-bold text-ink-600">
                        {lib.name}
                      </span>
                      <span className="shrink-0 rounded px-1.5 py-0.5 text-[10px] font-bold border border-gray-200 bg-gray-50 text-ink-50">
                        {TYPE_LABELS[lib.type] || '自定义'}
                      </span>
                    </div>
                    <p className="text-xs text-sand-500 mt-0.5">
                      {isSelected ? '已启用轮播' : '未参与轮播'}
                    </p>
                  </div>
                </div>

                <div
                  className={`flex h-6 w-6 shrink-0 items-center justify-center rounded-lg border transition-colors ${
                    isSelected
                      ? 'border-brand-500 bg-brand-500 text-white'
                      : 'border-gray-300 bg-white text-transparent'
                  }`}
                >
                  <Check size={14} strokeWidth={3} />
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* 底部保存按钮 */}
      <div className="flex items-center justify-between pt-2 border-t border-gray-200/80">
        <span className="text-xs text-sand-500">
          已选择 {selectedIds.length} / {libraries.length} 个媒体库参与轮播
        </span>
        <button
          type="button"
          onClick={handleSave}
          disabled={saving || !dirty}
          className="neon-button disabled:opacity-50"
        >
          {saving ? <Loader2 size={16} className="animate-spin" /> : <Save size={16} />}
          保存设置
        </button>
      </div>
    </div>
  )
}
