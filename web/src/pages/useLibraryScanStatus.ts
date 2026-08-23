import { useCallback, useState } from 'react'
import toast from 'react-hot-toast'

import { libraryAPI } from '../api/library'
import { useWebSocket } from '../hooks/useWebSocket'
import { formatDuration } from './libraryPageModel'

type UseLibraryScanStatusInput = {
  libraryID: string
  isAdmin: boolean
  onLibraryChanged: () => void
}

export function useLibraryScanStatus({
  libraryID,
  isAdmin,
  onLibraryChanged,
}: UseLibraryScanStatusInput) {
  const [scanning, setScanning] = useState(false)
  const [scanProgress, setScanProgress] = useState('')

  const onRealtimeEvent = useCallback((topic: string, payload: unknown) => {
    if (!isAdmin) return
    if (topic !== 'scan' || !payload || typeof payload !== 'object') return
    const event = payload as Record<string, unknown>
    if (event.library_id !== libraryID) return
    if (event.error) {
      setScanning(false)
      setScanProgress(`扫描失败：${String(event.error)}`)
      return
    }
    if (event.finished) {
      setScanning(false)
      setScanProgress(finishedScanMessage(event))
      onLibraryChanged()
      return
    }
    if (event.queued) {
      setScanning(true)
      setScanProgress(String(event.message ?? '扫描已排队，后台会自动入库'))
      return
    }
  }, [isAdmin, libraryID, onLibraryChanged])

  useWebSocket(onRealtimeEvent)

  const handleScan = useCallback(async () => {
    setScanning(true)
    setScanProgress('正在提交扫描任务…')
    try {
      const result = await libraryAPI.scan(libraryID)
      toast.success(`扫描完成:新增 ${result.added} 项，更新 ${result.updated ?? 0} 项`)
      setScanProgress(`扫描完成：新增 ${result.added} · 更新 ${result.updated ?? 0}`)
      onLibraryChanged()
    } catch {
      toast.error('扫描失败')
      setScanProgress('扫描失败，请查看日志或稍后重试')
    } finally {
      setScanning(false)
    }
  }, [libraryID, onLibraryChanged])

  return {
    scanning,
    scanProgress,
    handleScan,
  }
}

function finishedScanMessage(event: Record<string, unknown>) {
  const elapsed = Number(event.elapsed_seconds ?? event.elapsed ?? 0)
  const elapsedText = elapsed > 0 ? ` · 耗时 ${formatDuration(elapsed)}` : ''
  return `扫描完成：发现 ${event.discovered ?? event.visited ?? 0} · 新增 ${event.added ?? 0} · 更新 ${event.updated ?? 0} · 跳过 ${event.skipped ?? 0}${elapsedText}`
}