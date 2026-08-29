import { useCallback, useEffect, useRef, useState } from 'react'
import toast from 'react-hot-toast'
import { CheckCircle2, Download, Loader2, XCircle } from 'lucide-react'

import { toolsAPI, type FFmpegToolInfo, type FFmpegToolsStatus } from '../api/tools'

// FFToolsPanel 展示 ffmpeg/ffprobe 的安装状态，并提供「一键下载安装」：
// 后端自动匹配当前运行环境（OS + 架构），下载到 data 目录并写入设置，
// 无需手动填写路径。
export function FFToolsPanel({ onInstalled }: { onInstalled?: () => void }) {
  const [status, setStatus] = useState<FFmpegToolsStatus | null>(null)
  const [working, setWorking] = useState(false)
  const pollingRef = useRef(false)

  const load = useCallback(async () => {
    try {
      setStatus(await toolsAPI.ffToolsStatus())
    } catch {
      // 网络波动时保持旧状态，不打断页面
    }
  }, [])

  useEffect(() => {
    load().catch(() => undefined)
  }, [load])

  const installing = Boolean(status?.installing)

  // 安装进行中：轮询状态直到结束，完成后刷新设置页（路径字段自动填入）。
  useEffect(() => {
    if (!installing || pollingRef.current) return
    pollingRef.current = true
    const timer = window.setInterval(async () => {
      try {
        const next = await toolsAPI.ffToolsStatus()
        setStatus(next)
        if (!next.installing) {
          window.clearInterval(timer)
          pollingRef.current = false
          if (next.error) {
            toast.error(`FFmpeg 下载安装失败：${next.error}`)
          } else {
            toast.success('FFmpeg / FFprobe 下载安装完成，系统已自动使用')
            onInstalled?.()
          }
        }
      } catch {
        // 轮询失败继续等下一轮
      }
    }, 1500)
    return () => {
      window.clearInterval(timer)
      pollingRef.current = false
    }
  }, [installing, onInstalled])

  const install = async () => {
    setWorking(true)
    try {
      const next = await toolsAPI.installFFTools()
      setStatus(next)
      if (next.error && !next.installing) {
        toast.error(next.error)
      } else if (next.installing) {
        toast('开始下载，完成后自动生效')
      } else {
        toast.success('已检测到可用工具')
        onInstalled?.()
      }
    } catch (err) {
      toast.error(
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
          (err as { message?: string })?.message ??
          '启动下载失败',
      )
    } finally {
      setWorking(false)
    }
  }

  const busy = working || installing
  const targetLabel = status?.target?.label ?? '当前平台'
  const ffmpegInstalled = Boolean(status?.ffmpeg?.installed)
  const ffprobeInstalled = Boolean(status?.ffprobe?.installed)

  return (
    <div className="glass-panel space-y-3">
      <div className="flex items-center justify-between gap-3">
        <div>
          <div className="font-display text-lg font-bold text-ink-600">FFmpeg / FFprobe 工具</div>
          <div className="text-xs text-sand-500">
            当前平台：{targetLabel} · 安装目录：
            <span className="font-mono">{status?.install_dir ?? '…'}</span>
          </div>
        </div>
        <button
          type="button"
          onClick={install}
          disabled={busy}
          className="neon-button shrink-0 disabled:opacity-50"
        >
          {busy ? <Loader2 size={16} className="animate-spin" /> : <Download size={16} />}
          {busy ? '下载安装中…' : '下载并安装'}
        </button>
      </div>

      <div className="grid gap-2 md:grid-cols-2">
        <ToolRow name="FFmpeg" info={status?.ffmpeg} />
        <ToolRow name="FFprobe" info={status?.ffprobe} />
      </div>

      {(status?.message || status?.error) && (
        <div
          className={
            status?.error
              ? 'rounded-xl bg-red-50 px-3 py-2 text-xs text-red-600'
              : 'rounded-xl bg-sand-100/70 px-3 py-2 text-xs text-sand-500'
          }
        >
          {status?.error || status?.message}
        </div>
      )}

      {!ffmpegInstalled || !ffprobeInstalled ? (
        <div className="text-xs text-sand-500">
          未检测到可用工具。点击「下载并安装」后，服务端会自动匹配当前系统下载对应版本，安装完成后无需手动填写路径。
        </div>
      ) : null}
    </div>
  )
}

function ToolRow({ name, info }: { name: string; info?: FFmpegToolInfo }) {
  return (
    <div className="flex items-center gap-2 rounded-xl border border-gray-200 bg-white/70 px-3 py-2">
      {info?.installed ? (
        <CheckCircle2 size={16} className="shrink-0 text-green-500" />
      ) : (
        <XCircle size={16} className="shrink-0 text-red-400" />
      )}
      <div className="min-w-0">
        <div className="text-sm font-medium text-ink-600">
          {name} {info?.installed ? '' : '（未安装）'}
        </div>
        {info?.installed && (
          <div className="truncate font-mono text-[11px] text-ink-50" title={info.path}>
            {info.path}
          </div>
        )}
        {info?.version && <div className="truncate text-[11px] text-sand-500">{info.version}</div>}
      </div>
    </div>
  )
}