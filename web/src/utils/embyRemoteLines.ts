export type EmbyRemoteLine = {
  name: string
  url: string
}

export function normalizeEmbyRemoteLines(lines: EmbyRemoteLine[]): EmbyRemoteLine[] {
  return lines
    .map((line) => ({
      name: line.name.trim(),
      url: line.url.trim().replace(/\/+$/, ''),
    }))
    .filter((line) => line.url.startsWith('http://') || line.url.startsWith('https://'))
}

export function encodeEmbyRemoteConfigLines(lines: EmbyRemoteLine[]): Record<string, string> {
  const normalized = normalizeEmbyRemoteLines(lines)
  if (normalized.length === 0) {
    return {}
  }
  return {
    urls: JSON.stringify(normalized),
    url: normalized[0].url,
  }
}

export function defaultEmbyRemoteLines(existing?: EmbyRemoteLine[]): EmbyRemoteLine[] {
  if (existing && existing.length > 0) {
    return existing.map((line) => ({ name: line.name ?? '', url: line.url ?? '' }))
  }
  return [{ name: '线路 1', url: '' }]
}
