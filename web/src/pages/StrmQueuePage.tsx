import { useCallback, useEffect, useState } from 'react'
import toast from 'react-hot-toast'
import { Ban, Download, Loader2, RefreshCw, Trash2, Upload } from 'lucide-react'

import { strmAPI } from '../api/strm'
import type { StrmQueueSnapshot, StrmTask, StrmTaskStatus } from '../types/strm'
import { STRM_PROVIDER_LABELS } from '../types/strm'
import { apiErrorMessage, formatBytes, formatTime, taskStatusMeta } from './StrmManagePage'

const FILTERS: { key: 'all' | StrmTaskStatus; label: string }[] = [
  { key: 'all', label: '全部' },
  { key: 'pending', label: '排队中' },
  { key: 'running', label: '进行中' },
  { key: 'done', label: '已完成' },
  { key: 'failed', label: '失败' },
  { key: 'canceled', label: '已取消' },
]

const PAGE_SIZE = 50

function StrmQueuePanel({ kind }: { kind: 'download' | 'upload' }) {
  const [snapshot, setSnapshot] = useState<StrmQueueSnapshot | null>(null)
  const [filter, setFilter] = useState<'all' | StrmTaskStatus>('all')
  const [page, setPage] = useState(1)
  const [totalPages, setTotalPages] = useState(1)
  const [loading, setLoading] = useState(true)
  const [batchBusy, setBatchBusy] = useState(false)

  const refresh = useCallback(async () => {
    try {
      const status = filter === 'all' ? undefined : filter
      const data =
        kind === 'download'
          ? await strmAPI.downloads(status, page, PAGE_SIZE)
          : await strmAPI.uploads(status, page, PAGE_SIZE)
      const tp = Math.max(1, Math.ceil((data.total ?? data.tasks.length) / PAGE_SIZE))
      if (page > tp) {
        // 当前页超出范围（数据被批量删除），回退到最后一页再刷新
        setPage(tp)
        return
      }
      setTotalPages(tp)
      setSnapshot(data)
    } catch {
      /* keep last data */
    } finally {
      setLoading(false)
    }
  }, [kind, filter, page])

  useEffect(() => {
    refresh().catch(() => undefined)
    const timer = setInterval(() => refresh().catch(() => undefined), 3000)
    return () => clearInterval(timer)
  }, [refresh])

  const cancelTask = async (task: StrmTask) => {
    try {
      if (kind === 'download') await strmAPI.cancelDownload(task.id)
      else await strmAPI.cancelUpload(task.id)
      toast.success('已取消任务')
      await refresh()
    } catch (err) {
      toast.error(apiErrorMessage(err))
    }
  }

  const retryTask = async (task: StrmTask) => {
    try {
      if (kind === 'download') await strmAPI.retryDownload(task.id)
      else await strmAPI.retryUpload(task.id)
      toast.success('已重新入队')
      await refresh()
    } catch (err) {
      toast.error(apiErrorMessage(err))
    }
  }

  // 批量操作：可选确认弹窗，操作中锁定按钮，结束后刷新。
  const runBatch = async (
    action: () => Promise<{ deleted?: number; retried?: number; canceled?: number }>,
    confirmMsg?: string,
  ) => {
    if (confirmMsg && !window.confirm(confirmMsg)) return
    setBatchBusy(true)
    try {
      const res = await action()
      if (res.deleted !== undefined) toast.success(`已清空 ${res.deleted} 条记录`)
      else if (res.retried !== undefined) toast.success(`已重新入队 ${res.retried} 个任务`)
      else if (res.canceled !== undefined) toast.success(`已取消 ${res.canceled} 个任务`)
      await refresh()
    } catch (err) {
      toast.error(apiErrorMessage(err))
    } finally {
      setBatchBusy(false)
    }
  }

  const batchBtn = (title: string, cls: string, onClick: () => void) => (
    <button
      type="button"
      disabled={batchBusy}
      onClick={onClick}
      className={
        'ml-auto inline-flex items-center gap-1 rounded-lg border px-2 py-1 text-xs font-semibold transition disabled:opacity-50 ' + cls
      }
    >
      <Trash2 size={12} />
      {title}
    </button>
  )

  const batchActionByFilter = () => {
    if (!isDownload) return null
    if (filter === 'done')
      return batchBtn(
        '清空成功记录',
        'border-gray-200 text-rose-500 hover:bg-rose-50',
        () => runBatch(() => strmAPI.clearDoneDownloads(), '确定清空所有已完成下载记录？'),
      )
    if (filter === 'failed')
      return batchBtn('批量重试', 'border-gray-200 text-brand-500 hover:bg-brand-50', () =>
        runBatch(() => strmAPI.retryFailedDownloads(), '确定重新入队所有失败下载任务？'),
      )
    if (filter === 'pending')
      return batchBtn(
        '批量取消',
        'border-gray-200 text-amber-600 hover:bg-amber-50',
        () => runBatch(() => strmAPI.cancelPendingDownloads(), '确定取消所有排队中的下载任务？'),
      )
    return null
  }

  const counts = snapshot?.counts
  const tasks = snapshot?.tasks.filter((t) => filter === 'all' || t.status === filter) ?? []
  const isDownload = kind === 'download'
  const Icon = isDownload ? Download : Upload

  return (
    <div className="space-y-5">
      <header className="flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-sand-300/40 text-ink-100">
          <Icon size={20} />
        </div>
        <div>
          <h1 className="font-display text-3xl font-bold text-ink-600">
            {isDownload ? '下载队列' : '上传队列'}
          </h1>
          <p className="text-sm text-ink-50">
            {isDownload
              ? 'STRM 元数据下载情况（远端网盘 → 本地输出目录，3 秒自动刷新）'
              : 'STRM 元数据上传情况（本地 → 远端网盘，3 秒自动刷新）'}
          </p>
        </div>
        {isDownload && (
          <button
            type="button"
            disabled={batchBusy}
            onClick={() =>
              runBatch(() => strmAPI.clearFinishedDownloads(), '确定清空所有失败和成功的下载记录？')
            }
            className="inline-flex items-center gap-1.5 rounded-xl border border-rose-200 px-3 py-2 text-sm font-semibold text-rose-500 transition hover:bg-rose-50 disabled:opacity-50"
          >
            <Trash2 size={14} />
            清空失败与完成记录
          </button>
        )}
        <button type="button" onClick={refresh} className="ml-auto inline-flex items-center gap-1.5 rounded-xl border border-gray-200 px-3 py-2 text-sm font-semibold text-ink-100 hover:bg-gray-50">
          <RefreshCw size={14} />
          刷新
        </button>
      </header>

      <div className="flex flex-wrap items-center gap-2">
        <StatPill label="排队中" value={counts?.pending ?? 0} cls="text-gray-600" />
        <StatPill label="进行中" value={counts?.running ?? 0} cls="text-brand-500" />
        <StatPill label="已完成" value={counts?.done ?? 0} cls="text-emerald-600" />
        <StatPill label="失败" value={counts?.failed ?? 0} cls="text-rose-500" />
        <StatPill label="已取消" value={counts?.canceled ?? 0} cls="text-amber-600" />
      </div>

      <div className="flex gap-1 overflow-x-auto border-b border-gray-200">
        {FILTERS.map((item) => (
          <button
            key={item.key}
            type="button"
            onClick={() => {
              setFilter(item.key)
              setPage(1)
            }}
            className={
              'border-b-2 px-3 py-2 text-sm whitespace-nowrap transition ' +
              (filter === item.key ? 'border-primary-400 text-brand-500' : 'border-transparent text-ink-50 hover:text-white')
            }
          >
            {item.label}
          </button>
        ))}
        {batchActionByFilter()}
      </div>

      <div className="glass-panel overflow-hidden">
        {loading ? (
          <div className="flex justify-center py-12 text-ink-50">
            <Loader2 className="animate-spin" />
          </div>
        ) : tasks.length === 0 ? (
          <p className="py-12 text-center text-sm text-sand-500">
            {filter === 'all' ? (isDownload ? '暂无元数据下载任务' : '暂无元数据上传任务') : '该状态下暂无任务'}
          </p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-gray-200 text-xs uppercase tracking-wider text-sand-500">
                <tr>
                  <th className="px-4 py-3">文件</th>
                  <th className="px-4 py-3">提供方</th>
                  <th className="px-4 py-3">{isDownload ? '本地目标' : '本地来源'}</th>
                  <th className="px-4 py-3">远端路径</th>
                  <th className="px-4 py-3 text-right">大小</th>
                  <th className="px-4 py-3">状态</th>
                  <th className="px-4 py-3">创建时间</th>
                  <th className="px-4 py-3 text-right">操作</th>
                </tr>
              </thead>
              <tbody>
                {tasks.map((task) => {
                  const status = taskStatusMeta(task.status)
                  return (
                    <tr key={task.id} className="border-t border-gray-200 transition hover:bg-gray-50">
                      <td className="max-w-[220px] truncate px-4 py-2.5 font-medium text-ink-600">{task.file_name}</td>
                      <td className="px-4 py-2.5 text-xs">
                        {STRM_PROVIDER_LABELS[task.provider] ?? task.provider}
                        {task.retry_count > 0 && (
                          <span className="ml-1 text-[10px] text-gray-400">重试 {task.retry_count}</span>
                        )}
                      </td>
                      <td className="max-w-[200px] truncate px-4 py-2.5 font-mono text-xs text-ink-50">{task.local_path}</td>
                      <td className="max-w-[200px] truncate px-4 py-2.5 font-mono text-xs text-ink-50">{task.remote_path}</td>
                      <td className="px-4 py-2.5 text-right text-xs text-ink-50">{formatBytes(task.size)}</td>
                      <td className="px-4 py-2.5">
                        <span className={'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-semibold ' + status.cls}>
                          {task.status === 'running' && <Loader2 size={10} className="animate-spin" />}
                          {status.label}
                        </span>
                        {task.error && (
                          <p className="mt-0.5 max-w-[220px] truncate text-[11px] text-rose-500" title={task.error}>
                            {task.error}
                          </p>
                        )}
                      </td>
                      <td className="whitespace-nowrap px-4 py-2.5 text-xs text-ink-50">{formatTime(task.created_at)}</td>
                      <td className="whitespace-nowrap px-4 py-2.5 text-right">
                        {(task.status === 'pending' || task.status === 'running') && (
                          <button
                            type="button"
                            onClick={() => cancelTask(task)}
                            className="inline-flex items-center gap-1 rounded-lg border border-gray-200 px-2 py-1 text-xs font-semibold text-amber-600 transition hover:bg-amber-50"
                          >
                            <Ban size={12} />
                            取消
                          </button>
                        )}
                        {(task.status === 'failed' || task.status === 'canceled') && (
                          <button
                            type="button"
                            onClick={() => retryTask(task)}
                            className="ml-1 inline-flex items-center gap-1 rounded-lg border border-gray-200 px-2 py-1 text-xs font-semibold text-brand-500 transition hover:bg-brand-50"
                          >
                            <RefreshCw size={12} />
                            重试
                          </button>
                        )}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
        {(snapshot?.total ?? 0) > 0 && (
          <div className="flex items-center justify-between border-t border-gray-200 px-4 py-3">
            <span className="text-xs text-ink-50">
              共 {snapshot?.total ?? 0} 条 · 第 {page} / {totalPages} 页
            </span>
            <div className="flex items-center gap-2">
              <button
                type="button"
                disabled={page <= 1 || loading}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                className="inline-flex items-center rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-semibold text-ink-100 transition hover:bg-gray-50 disabled:opacity-40"
              >
                上一页
              </button>
              <button
                type="button"
                disabled={page >= totalPages || loading}
                onClick={() => setPage((p) => p + 1)}
                className="inline-flex items-center rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-semibold text-ink-100 transition hover:bg-gray-50 disabled:opacity-40"
              >
                下一页
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

function StatPill({ label, value, cls }: { label: string; value: number; cls: string }) {
  return (
    <div className="flex items-center gap-2 rounded-xl border border-gray-100 bg-white px-3 py-2 shadow-sm">
      <span className="text-xs text-sand-500">{label}</span>
      <span className={'font-display text-lg font-bold ' + cls}>{value}</span>
    </div>
  )
}

export function StrmDownloadQueuePage() {
  return <StrmQueuePanel kind="download" />
}

export function StrmUploadQueuePage() {
  return <StrmQueuePanel kind="upload" />
}