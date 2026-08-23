// Danmaku parsing utilities. Normalizes Bilibili XML and custom JSON danmaku
// sources into a single Comment[] shape that the player renders.
//
// The Bilibili <d> element carries a `p` attribute:
//   p="time, mode, fontSize, color, timestamp, pool, senderHash, dmid"
// where mode: 1=scroll, 4=bottom, 5=top.
//
// Custom JSON sources may look like either `{ comments: [...] }` or a bare
// array; each comment may be an object `{ time, text, mode, color, size }` or
// the classic CCL tuple `[time, mode, size, color, text]`. The dandanplay
// protocol's own JSON shape `{ comments: [{ p: "time,mode,color,user", m:
// text, t: seconds }] }` is detected and parsed too.

export type DanmakuMode = 'scroll' | 'top' | 'bottom'

export interface Comment {
  /** Offset in seconds relative to the clip start. */
  time: number
  text: string
  mode: DanmakuMode
  /** CSS color, e.g. "#ffffff" or a hex int string. */
  color: string
  /** Font size ratio relative to the default (1 = default). */
  size: number
}

const BILI_MODES: Record<string, DanmakuMode> = { '1': 'scroll', '4': 'bottom', '5': 'top' }

function biliColorToCSS(color: string): string {
  if (color && color !== '16777215') {
    return `#${Number(color).toString(16).padStart(6, '0')}`
  }
  return '#ffffff'
}

/** Parse a Bilibili XML danmaku document into normalized comments. */
//
// Two flavors of `p` attribute exist:
//   Bilibili:   time, mode, fontSize, color, timestamp, pool, senderHash, dmid
//   dandanplay: time, mode, color, userId                      (4 fields only)
// Field count disambiguates: >=5 fields means Bilibili (color at index 3),
// 4 fields means dandanplay (color at index 2).
function parseBilibiliXml(xml: string): Comment[] {
  const comments: Comment[] = []
  const pattern = /<d\s+p="([^"]*)"[^>]*>([\s\S]*?)<\/d>/g
  let match: RegExpExecArray | null
  while ((match = pattern.exec(xml)) !== null) {
    const fields = match[1].split(',')
    const text = match[2].trim()
    const t = Number(fields[0])
    if (!text || !Number.isFinite(t) || t < 0) continue
    const modeStr = fields[1]
    const isDandanplay = fields.length >= 4 && fields.length < 5
    const sizeStr = isDandanplay ? '' : fields[2]
    const colorStr = isDandanplay ? fields[2] : fields[3]
    comments.push({
      time: t,
      text,
      mode: BILI_MODES[modeStr] ?? 'scroll',
      color: biliColorToCSS(colorStr),
      size: clampSize(Number(sizeStr)),
    })
  }
  return comments
}

interface JsonComment {
  time?: number
  text?: string
  mode?: number | string
  color?: string | number
  size?: number
  /** dandanplay JSON 弹幕（弹弹play 新接口 / 自建源）：p="time,mode,color,user" */
  p?: string
  m?: string
  t?: number
}

/** dandanplay JSON items look like { p, m, t, cid, like } — no time/text keys. */
function isDandanplayJson(list: JsonComment[]): boolean {
  const first = list[0]
  return (
    !!first &&
    typeof first === 'object' &&
    !Array.isArray(first) &&
    typeof first.p === 'string' &&
    typeof first.m === 'string'
  )
}

/** Parse the dandanplay JSON shape: { comments: [{ p, m, t, ... }] }. */
function parseDandanplayJson(list: JsonComment[]): Comment[] {
  const comments: Comment[] = []
  for (const item of list) {
    if (!item || typeof item !== 'object' || Array.isArray(item)) continue
    const text = String(item.m ?? '').trim()
    if (!text) continue
    const fields = String(item.p ?? '').split(',')
    const t = Number(fields[0])
    if (!Number.isFinite(t) || t < 0) continue
    comments.push({
      time: t,
      text,
      mode: BILI_MODES[fields[1]] ?? 'scroll',
      color: biliColorToCSS(fields[2]),
      size: 1,
    })
  }
  return comments
}

function clampSize(size: number): number {
  if (!Number.isFinite(size) || size <= 0) return 1
  // Bilibili font sizes are absolute 12-64; fold them onto a ~0.75-1.5 ratio.
  return Math.min(1.6, Math.max(0.6, size / 25))
}

/** Parse custom JSON danmaku data into normalized comments. */
function parseJson(raw: string): Comment[] {
  const data = JSON.parse(raw) as JsonComment[] | { comments?: JsonComment[] } | unknown
  const list = Array.isArray(data) ? data : (data as { comments?: JsonComment[] })?.comments
  if (!Array.isArray(list)) return []
  if (isDandanplayJson(list)) return parseDandanplayJson(list)
  const comments: Comment[] = []
  for (const rawComment of list) {
    let keyed: JsonComment
    if (Array.isArray(rawComment)) {
      // Classic CCL tuple: [time, mode, size, color, text]
      keyed = {
        time: Number(rawComment[0]),
        mode: rawComment[1],
        size: Number(rawComment[2]),
        color: rawComment[3],
        text: rawComment[4],
      }
    } else {
      keyed = rawComment as JsonComment
    }
    const t = Number(keyed.time)
    const text = String(keyed.text ?? '').trim()
    if (!text || !Number.isFinite(t) || t < 0) continue
    // Accept both Bilibili mode ints and explicit "scroll"/"top"/"bottom".
    let mode: DanmakuMode
    if (typeof keyed.mode === 'string') {
      mode = (['scroll', 'top', 'bottom'] as const).includes(keyed.mode as DanmakuMode)
        ? (keyed.mode as DanmakuMode)
        : 'scroll'
    } else {
      mode = BILI_MODES[String(keyed.mode)] ?? 'scroll'
    }
    let color = '#ffffff'
    if (typeof keyed.color === 'string') {
      if (/^#/.test(keyed.color)) {
        color = keyed.color
      } else {
        const n = Number(keyed.color)
        if (Number.isFinite(n)) color = `#${n.toString(16).padStart(6, '0')}`
      }
    } else if (typeof keyed.color === 'number' && Number.isFinite(keyed.color)) {
      color = `#${keyed.color.toString(16).padStart(6, '0')}`
    }
    comments.push({ time: t, text, mode, color, size: clampSize(Number(keyed.size)) })
  }
  return comments
}

/**
 * Parse danmaku payload into normalized comments. Auto-detects the format when
 * `sourceType` is "auto" (tries JSON first, falls back to XML).
 */
export function parseDanmaku(raw: string, sourceType: 'auto' | 'xml' | 'json'): Comment[] {
  if (sourceType === 'json' || (sourceType === 'auto' && looksLikeJson(raw))) {
    try {
      return parseJson(raw)
    } catch {
      if (sourceType === 'json') return []
    }
  }
  if (sourceType === 'xml') return parseBilibiliXml(raw)
  // auto: JSON parsing failed or didn't look like JSON — try XML.
  return parseBilibiliXml(raw)
}

function looksLikeJson(raw: string): boolean {
  const trimmed = raw.trimStart()
  return trimmed.startsWith('[') || trimmed.startsWith('{')
}