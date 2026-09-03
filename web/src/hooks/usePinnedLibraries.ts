import { useCallback, useEffect, useRef, useState } from 'react'

import {
  loadPinnedLibraryIds,
  savePinnedLibraryIds,
  togglePinnedLibraryId,
} from '../utils/pinnedLibraries'

export function usePinnedLibraries() {
  const [pinnedIds, setPinnedIds] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [syncing, setSyncing] = useState(false)
  const [loadError, setLoadError] = useState(false)
  const loadedRef = useRef(false)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setLoadError(false)
    loadedRef.current = false
    loadPinnedLibraryIds()
      .then((ids) => {
        if (cancelled) return
        loadedRef.current = true
        setPinnedIds(ids)
        setLoadError(false)
      })
      .catch(() => {
        if (cancelled) return
        loadedRef.current = false
        setLoadError(true)
        // Keep any existing pins in memory; never replace a known server list with [].
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  const togglePin = useCallback(async (libraryId: string) => {
    // Avoid saving from an unloaded/failed state — that would overwrite the
    // server list with only the clicked id and can wipe existing local pins
    // when mounted Emby ids used to be filtered out server-side.
    if (!loadedRef.current || loading || loadError) return

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
  }, [loading, loadError])

  return { pinnedIds, loading, syncing, loadError, togglePin }
}
