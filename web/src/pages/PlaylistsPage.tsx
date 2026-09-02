import { FormEvent, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { ChevronRight, ListMusic, ListPlus, Trash2 } from 'lucide-react'
import toast from 'react-hot-toast'

import { playbackAPI } from '../api/playback'
import { confirmAction } from '../components/confirmAction'
import { PageHeader } from '../components/PageHeader'
import type { Playlist } from '../types'

export function PlaylistsPage() {
  const [items, setItems] = useState<Playlist[]>([])
  const [name, setName] = useState('')
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)

  const refresh = () =>
    playbackAPI
      .listPlaylists()
      .then(setItems)
      .finally(() => setLoading(false))

  useEffect(() => {
    refresh().catch(() => undefined)
  }, [])

  const onCreate = async (e: FormEvent) => {
    e.preventDefault()
    const trimmed = name.trim()
    if (!trimmed) return

    setCreating(true)
    try {
      await playbackAPI.createPlaylist(trimmed)
      toast.success('已创建')
      setName('')
      await refresh()
    } catch {
      toast.error('创建失败')
    } finally {
      setCreating(false)
    }
  }

  const isEmpty = !loading && items.length === 0

  return (
    <div className="space-y-6">
      <PageHeader
        title="播放列表"
        description="整理你想稍后观看的影片，随时继续播放"
      />

      <form
        onSubmit={onCreate}
        className="glass-panel flex flex-col gap-3 sm:flex-row sm:items-center"
      >
        <input
          required
          className="input-base min-w-0 flex-1"
          placeholder="新播放列表名称"
          value={name}
          onChange={(e) => setName(e.target.value)}
          disabled={creating}
        />
        <button type="submit" className="neon-button shrink-0" disabled={creating}>
          <ListPlus size={16} />
          {creating ? '创建中…' : '新建'}
        </button>
      </form>

      {loading && (
        <div className="flex items-center gap-2 py-8 text-ink-50">
          <span className="inline-block h-4 w-4 animate-spin rounded-full border-2 border-primary-400 border-t-transparent" />
          加载中…
        </div>
      )}

      {isEmpty && (
        <div className="glass-panel flex flex-col items-center gap-4 p-10 text-center">
          <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-primary-400/10 text-brand-500">
            <ListMusic size={28} />
          </div>
          <div className="space-y-1">
            <p className="text-lg font-medium text-ink-100">还没有任何播放列表</p>
            <p className="text-sm text-sand-500">
              在上方输入名称创建列表，或在媒体详情页点击「加入播放列表」
            </p>
          </div>
        </div>
      )}

      {items.length > 0 && (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {items.map((p) => (
            <div
              key={p.id}
              className="glass-panel group flex items-center gap-3 !p-4 transition hover:border-primary-400/30"
            >
              <Link
                to={`/playlist/${p.id}`}
                className="flex min-w-0 flex-1 items-center gap-3"
              >
                <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-primary-400/10 text-brand-500 transition group-hover:bg-primary-400/20">
                  <ListMusic size={20} />
                </div>
                <div className="min-w-0 flex-1">
                  <p className="truncate font-medium text-ink-600 transition group-hover:text-brand-500">
                    {p.name}
                  </p>
                  {p.is_public && (
                    <span className="mt-0.5 inline-block rounded-md border border-primary-400/40 px-1.5 py-0.5 text-[10px] uppercase tracking-wide text-brand-500">
                      公开
                    </span>
                  )}
                </div>
                <ChevronRight
                  size={18}
                  className="shrink-0 text-sand-500 transition group-hover:text-brand-500"
                />
              </Link>
              <button
                type="button"
                onClick={async () => {
                  if (
                    !(await confirmAction({
                      title: '删除播放列表',
                      message: `删除「${p.name}」?`,
                      confirmText: '删除',
                    }))
                  ) {
                    return
                  }
                  await playbackAPI.deletePlaylist(p.id)
                  toast.success('已删除')
                  await refresh()
                }}
                className="shrink-0 rounded-lg border border-red-400/40 p-2 text-red-400 transition hover:bg-red-400/10"
                title="删除播放列表"
              >
                <Trash2 size={14} />
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
