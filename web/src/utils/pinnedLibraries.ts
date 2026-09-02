const STORAGE_KEY = 'mebox_pinned_libraries'

function parsePinnedIds(raw: string | null): string[] {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter((id): id is string => typeof id === 'string' && id.trim() !== '')
  } catch {
    return []
  }
}

export function readPinnedLibraryIds(): string[] {
  if (typeof window === 'undefined') return []
  return parsePinnedIds(window.localStorage.getItem(STORAGE_KEY))
}

export function writePinnedLibraryIds(ids: string[]): void {
  if (typeof window === 'undefined') return
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(ids))
}

export function togglePinnedLibraryId(id: string): string[] {
  const current = readPinnedLibraryIds()
  const index = current.indexOf(id)
  const next = index >= 0 ? current.filter((item) => item !== id) : [...current, id]
  writePinnedLibraryIds(next)
  return next
}

export function isLibraryPinned(id: string, pinnedIds: string[]): boolean {
  return pinnedIds.includes(id)
}

export function sortLibraryPreviews<T extends { library: { id: string } }>(items: T[], pinnedIds: string[]): T[] {
  if (pinnedIds.length === 0) return items
  const rank = new Map(pinnedIds.map((id, index) => [id, index]))
  return [...items].sort((a, b) => {
    const aRank = rank.get(a.library.id)
    const bRank = rank.get(b.library.id)
    const aPinned = aRank !== undefined
    const bPinned = bRank !== undefined
    if (aPinned !== bPinned) return aPinned ? -1 : 1
    if (aPinned && bPinned) return aRank - bRank
    return 0
  })
}
