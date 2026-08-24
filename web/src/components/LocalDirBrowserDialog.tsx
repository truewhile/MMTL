import { useEffect, useState } from 'react'
import { ChevronRight, Folder, FolderPlus, Loader2, X } from 'lucide-react'
import toast from 'react-hot-toast'

import { strmAPI } from '../api/strm'

function apiErrorMessage(err: unknown): string {
  if (typeof err === 'object' && err !== null && 'response' in err) {
    const res = (err as { response?: { data?: { error?: string; message?: string } } }).response
    if (res?.data?.error) return res.data.error
    if (res?.data?.message) return res.data.message
  }
  if (err instanceof Error) return err.message
  return '请求失败'
}

export function LocalDirBrowserDialog({
  initialDir,
  title = '选择本地目录',
  onSelect,
  onClose,
}: {
  initialDir?: string
  title?: string
  onSelect: (path: string) => void
  onClose: () => void
}) {
  const [list, setList] = useState<{
    roots: boolean
    parent?: string
    current?: string
    children: { name: string; path: string }[]
  } | null>(null)
  const [loading, setLoading] = useState(true)

  const load = async (target: string) => {
    setLoading(true)
    try {
      const data = await strmAPI.listLocalDirs(target)
      setList(data)
    } catch (err) {
      toast.error(apiErrorMessage(err))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load(initialDir ?? '').catch(() => undefined)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const selectable = list && !list.roots && list.current

  return (
    <div
      className="fixed inset-0 z-[110] flex items-center justify-center bg-black/35 p-4 backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        className="flex max-h-[80vh] w-full max-w-2xl flex-col overflow-hidden rounded-3xl border border-white/70 bg-white shadow-2xl"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex items-center justify-between border-b border-gray-100 px-6 py-4">
          <h3 className="font-display text-lg font-bold text-ink-600">{title}</h3>
          <button
            type="button"
            onClick={onClose}
            className="rounded-xl p-1.5 text-ink-50 transition hover:bg-gray-100 hover:text-ink-600"
            title="关闭"
          >
            <X size={20} />
          </button>
        </div>
        <div className="flex items-center gap-2 border-b border-gray-100 px-6 py-2.5 text-xs text-sand-500">
          {list?.roots ? (
            <span className="text-ink-50">选择盘符 / 根目录</span>
          ) : (
            <>
              <button
                type="button"
                className="hover:text-brand-500"
                onClick={() => load('')}
              >
                盘符 / 根
              </button>
              <ChevronRight size={12} />
              <span className="truncate text-ink-50" title={list?.current}>
                {list?.current}
              </span>
            </>
          )}
        </div>
        <div className="min-h-[260px] flex-1 overflow-y-auto p-3">
          {loading ? (
            <div className="flex justify-center py-10 text-ink-50">
              <Loader2 className="animate-spin" />
            </div>
          ) : !list || list.children.length === 0 ? (
            <p className="py-10 text-center text-sm text-sand-500">该目录下没有子目录</p>
          ) : (
            <div className="space-y-1">
              {(list.parent ?? '') !== '' && (
                <button
                  type="button"
                  className="flex w-full items-center gap-3 rounded-xl px-3 py-2 text-left text-sm text-ink-50 transition hover:bg-gray-50"
                  onClick={() => load(list.parent ?? '')}
                >
                  <FolderPlus size={16} className="text-sand-400" />
                  <span>..（上级目录）</span>
                </button>
              )}
              {list.children.map((entry) => (
                <button
                  key={entry.path}
                  type="button"
                  className="flex w-full items-center gap-3 rounded-xl px-3 py-2 text-left text-sm transition hover:bg-gray-50"
                  onClick={() => load(entry.path)}
                >
                  <Folder size={16} className="text-brand-400" />
                  <span className="flex-1 truncate text-ink-600">{entry.name}</span>
                </button>
              ))}
            </div>
          )}
        </div>
        <div className="flex items-center justify-between border-t border-gray-100 px-6 py-3">
          <span className="text-xs text-sand-500">单击目录进入下一级，点击「选择当前目录」完成选择</span>
          <button
            type="button"
            className="neon-button"
            disabled={!selectable || loading}
            onClick={() => {
              if (list?.current) onSelect(list.current)
            }}
          >
            选择当前目录
          </button>
        </div>
      </div>
    </div>
  )
}
