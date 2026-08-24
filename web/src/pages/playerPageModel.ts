import type { Media } from '../types'

export type PlayerMode = 'direct' | 'hls'

const directContainers = ['mp4', 'webm', 'm4v']
const directVideoCodecs = ['h264', 'avc', 'avc1']
const directAudioCodecs = ['aac', 'mp3', 'opus']

export function pickPlayerMode(media: Media): PlayerMode {
  return needsTranscodeForBrowser(media) ? 'hls' : 'direct'
}

export function needsTranscodeForBrowser(media: Media): boolean {
  const container = (media.container ?? '').toLowerCase()
  // .strm 媒体内容是远程直链（服务端 302 到播放 CDN 或反向代理），
  // 浏览器直接播放该远程流即可，转码无意义且必然失败（ffmpeg 无法读取
  // 文本 strm），恒走 direct play。
  if (container.includes('strm') || String(media.strm_url ?? '').trim() !== '') return false
  const videoCodec = (media.video_codec ?? '').toLowerCase()
  const audioCodec = (media.audio_codec ?? '').toLowerCase()
  const containerOK = directContainers.some((item) => container.includes(item))
  const videoOK = !videoCodec || directVideoCodecs.some((item) => videoCodec.includes(item))
  const audioOK = !audioCodec || directAudioCodecs.some((item) => audioCodec.includes(item))
  return !(containerOK && videoOK && audioOK)
}
