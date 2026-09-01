import type { Media } from '../types'
import { isRemoteEmbyID } from '../utils/remoteEmby'

export type PlayerMode = 'direct' | 'hls'

const directContainers = ['mp4', 'webm', 'm4v']
const directVideoCodecs = ['h264', 'avc', 'avc1']
const directAudioCodecs = ['aac', 'mp3', 'opus']

/**
 * 判断媒体是否为远程直链或挂载直连流媒体（STRM 或 Emby 远程挂载）。
 * 这类媒体服务端 302 重定向到直链或进行原生流中继，本地不具备原始文件，
 * 无法也不应该进行 ffmpeg 转码，恒走直连播放。
 */
export function isDirectStreamMedia(media?: Media | null): boolean {
  if (!media) return false
  if (isRemoteEmbyID(media.id)) return true
  const container = (media.container ?? '').toLowerCase()
  return container.includes('strm') || String(media.strm_url ?? '').trim() !== ''
}

export function pickPlayerMode(media: Media): PlayerMode {
  return needsTranscodeForBrowser(media) ? 'hls' : 'direct'
}

export function needsTranscodeForBrowser(media: Media): boolean {
  // Emby 远程挂载与 .strm 媒体一样，均为直连流，无法进行本地转码，恒走 direct play。
  if (isDirectStreamMedia(media)) return false

  const container = (media.container ?? '').toLowerCase()
  const videoCodec = (media.video_codec ?? '').toLowerCase()
  const audioCodec = (media.audio_codec ?? '').toLowerCase()
  const containerOK = directContainers.some((item) => container.includes(item))
  const videoOK = !videoCodec || directVideoCodecs.some((item) => videoCodec.includes(item))
  const audioOK = !audioCodec || directAudioCodecs.some((item) => audioCodec.includes(item))
  return !(containerOK && videoOK && audioOK)
}

