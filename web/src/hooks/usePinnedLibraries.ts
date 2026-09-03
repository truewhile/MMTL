import { useCallback, useEffect, useState } from 'react'

import {
  loadPinnedLibraryIds,
  savePinnedLibraryIds,
  togglePinnedLibraryId,
} from '../utils/pinnedLibraries'

export function usePinnedLibraries() {
  const [pinnedIds, setPinnedIds] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [syncing, setSyncing] = useState(false)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    loadPinnedLibraryIds()
      .then((ids) => {
        if (!cancelled) setPinnedIds(ids)
      })
      .catch(() => {
        if (!cancelled) setPinnedIds([])
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  const togglePin = useCallback(async (libraryId: string) => {
    let previous: string[] = []
    let optimistic: string[] = []
    setPinnedIds((current) => {
      previous = current
      optimistic = togglePinnedLibraryId(current, libraryId)
      return optimistic
    })
    setSyncing(true)
    try {
      const saved = await savePinnedLibraryIds(optimistic)
      setPinnedIds(saved)
    } catch {
      setPinnedIds(previous)
    } finally {
      setSyncing(false)
    }
  }, [])

  return { pinnedIds, loading, syncing, togglePin }
}
