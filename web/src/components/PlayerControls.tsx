import { useEffect, useRef, useState } from 'react'
import {
  Captions,
  CaptionsOff,
  Maximize,
  MessageSquareText,
  Minimize,
  Pause,
  PictureInPicture,
  Play,
  Volume2,
  VolumeX,
} from 'lucide-react'
import type { SubtitleTrack } from '../api/subtitles'

// PlayerControls — custom bottom control bar replacing the native <video
// controls> (which cannot host custom buttons). The danmaku toggle sits right
// next to the volume control. The bar auto-hides while playing and reappears
// on mouse movement; it stays visible while paused.
//
// Native keyboard shortcuts (space / arrows) still work because they are
// element-level defaults on <video>. Subtitles from <track> elements keep
// rendering; the CC button opens a track picker (关闭 / 各轨道).

function formatTime(s: number): string {
  if (!Number.isFinite(s) || s < 0) s = 0
  const m = Math.floor(s / 60)
  const sec = Math.floor(s % 60)
  return `${m}:${String(sec).padStart(2, '0')}`
}

type PlayerControlsProps = {
  videoRef: React.RefObject<HTMLVideoElement>
  subs: SubtitleTrack[]
  /** 当前激活字幕轨道：-1=关闭，0..n-1=对应轨道。 */
  subtitleIndex: number
  onSelectSubtitle: (index: number) => void
  danmakuOpen: boolean
  danmakuEnabled: boolean
  onToggleDanmaku: () => void
}

export function PlayerControls({
  videoRef,
  subs,
  subtitleIndex,
  onSelectSubtitle,
  danmakuOpen,
  danmakuEnabled,
  onToggleDanmaku,
}: PlayerControlsProps) {
  const video = () => videoRef.current
  const container = () => videoRef.current?.parentElement ?? null

  const [playing, setPlaying] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [volume, setVolume] = useState(1)
  const [muted, setMuted] = useState(false)
  const [fullscreen, setFullscreen] = useState(false)
  const [pip, setPip] = useState(false)
  const [uiVisible, setUiVisible] = useState(true)
  const [subtitleMenuOpen, setSubtitleMenuOpen] = useState(false)
  const subtitleMenuRef = useRef<HTMLDivElement | null>(null)
  const hideTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // 点击控制栏外部时关闭字幕菜单
  useEffect(() => {
    if (!subtitleMenuOpen) return
    const onDocClick = (e: MouseEvent) => {
      if (subtitleMenuRef.current && !subtitleMenuRef.current.contains(e.target as Node)) {
        setSubtitleMenuOpen(false)
      }
    }
    document.addEventListener('mousedown', onDocClick)
    return () => document.removeEventListener('mousedown', onDocClick)
  }, [subtitleMenuOpen])

  // 播放时 3 秒无操作自动隐藏控制栏；暂停时保持显示。监听挂在视频容器上，
  // 控制栏隐藏（pointer-events-none）后移动鼠标仍能重新唤起。
  useEffect(() => {
    const el = video()
    if (!el) return
    const parent = el.parentElement
    const onMove = () => {
      setUiVisible(true)
      if (hideTimerRef.current) clearTimeout(hideTimerRef.current)
      if (!el.paused) {
        hideTimerRef.current = setTimeout(() => setUiVisible(false), 3000)
      }
    }
    const onLeave = () => {
      if (el.paused) return
      if (hideTimerRef.current) clearTimeout(hideTimerRef.current)
      setUiVisible(false)
    }
    const syncPlay = () => {
      setPlaying(!el.paused)
      onMove()
    }
    const syncTime = () => setCurrentTime(el.currentTime)
    const syncMeta = () => {
      setDuration(el.duration || 0)
      setCurrentTime(el.currentTime)
    }
    const syncVolume = () => {
      setVolume(el.volume)
      setMuted(el.muted)
    }
    const syncFullscreen = () => setFullscreen(document.fullscreenElement === parent)
    const syncPip = () => setPip(document.pictureInPictureElement === el)

    parent?.addEventListener('mousemove', onMove)
    parent?.addEventListener('mouseleave', onLeave)

    el.addEventListener('play', syncPlay)
    el.addEventListener('playing', syncPlay)
    el.addEventListener('pause', syncPlay)
    el.addEventListener('timeupdate', syncTime)
    el.addEventListener('durationchange', syncMeta)
    el.addEventListener('loadedmetadata', syncMeta)
    el.addEventListener('volumechange', syncVolume)
    document.addEventListener('fullscreenchange', syncFullscreen)
    el.addEventListener('enterpictureinpicture', syncPip)
    el.addEventListener('leavepictureinpicture', syncPip)

    syncMeta()
    syncVolume()
    syncFullscreen()
    return () => {
      parent?.removeEventListener('mousemove', onMove)
      parent?.removeEventListener('mouseleave', onLeave)
      el.removeEventListener('play', syncPlay)
      el.removeEventListener('playing', syncPlay)
      el.removeEventListener('pause', syncPlay)
      el.removeEventListener('timeupdate', syncTime)
      el.removeEventListener('durationchange', syncMeta)
      el.removeEventListener('loadedmetadata', syncMeta)
      el.removeEventListener('volumechange', syncVolume)
      document.removeEventListener('fullscreenchange', syncFullscreen)
      el.removeEventListener('enterpictureinpicture', syncPip)
      el.removeEventListener('leavepictureinpicture', syncPip)
      if (hideTimerRef.current) clearTimeout(hideTimerRef.current)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [videoRef])

  const togglePlay = () => {
    const el = video()
    if (!el) return
    if (el.paused) void el.play()?.catch(() => undefined)
    else el.pause()
  }

  const seek = (v: number) => {
    const el = video()
    if (!el) return
    el.currentTime = v
    setCurrentTime(v)
  }

  const changeVolume = (v: number) => {
    const el = video()
    if (!el) return
    el.volume = v
    el.muted = v === 0
    setVolume(v)
    setMuted(v === 0)
  }

  const toggleMute = () => {
    const el = video()
    if (!el) return
    el.muted = !el.muted
  }

  const toggleFullscreen = () => {
    const el = container()
    if (!el) return
    if (document.fullscreenElement) {
      void document.exitFullscreen()
    } else {
      void el.requestFullscreen?.()
    }
  }

  const togglePip = () => {
    const el = video()
    if (!el) return
    if (document.pictureInPictureElement) {
      void document.exitPictureInPicture()
    } else {
      void el.requestPictureInPicture?.()
    }
  }

  const pipSupported =
    typeof document !== 'undefined' &&
    'pictureInPictureEnabled' in document &&
    document.pictureInPictureEnabled

  return (
    <div
      className={`pointer-events-auto absolute inset-x-0 bottom-0 z-20 px-3 pb-2 pt-12 transition-opacity duration-300 ${
        uiVisible ? 'opacity-100' : 'pointer-events-none opacity-0'
      }`}
      onClick={(e) => e.stopPropagation()}
    >
      <div className="flex items-center gap-2 text-white">
        <button
          onClick={togglePlay}
          className="rounded-full p-1.5 transition hover:bg-white/15"
          title={playing ? '暂停' : '播放'}
        >
          {playing ? <Pause size={20} /> : <Play size={20} />}
        </button>

        <input
          type="range"
          min={0}
          max={duration || 0}
          step={0.1}
          value={currentTime}
          onChange={(e) => seek(Number(e.target.value))}
          className="min-w-0 flex-1 accent-rose-500"
          aria-label="播放进度"
        />

        <span className="shrink-0 font-mono text-xs tabular-nums text-white/85">
          {formatTime(currentTime)} / {formatTime(duration)}
        </span>

        {pipSupported && (
          <button
            onClick={togglePip}
            className="rounded-full p-1.5 transition hover:bg-white/15"
            title={pip ? '退出画中画' : '画中画'}
          >
            <PictureInPicture size={18} className={pip ? 'text-rose-400' : ''} />
          </button>
        )}

        {subs.length > 0 && (
          <div className="relative" ref={subtitleMenuRef}>
            <button
              onClick={() => setSubtitleMenuOpen((v) => !v)}
              className="rounded-full p-1.5 transition hover:bg-white/15"
              title="字幕"
            >
              {subtitleIndex >= 0 ? (
                <Captions size={18} className="text-rose-400" />
              ) : (
                <CaptionsOff size={18} className="text-white/70" />
              )}
            </button>
            {subtitleMenuOpen && (
              <div className="absolute bottom-11 right-0 z-30 min-w-36 rounded-xl border border-white/15 bg-black/85 p-1 shadow-2xl backdrop-blur">
                <button
                  type="button"
                  onClick={() => {
                    onSelectSubtitle(-1)
                    setSubtitleMenuOpen(false)
                  }}
                  className={`flex w-full items-center gap-2 rounded-lg px-3 py-1.5 text-left text-xs transition hover:bg-white/10 ${
                    subtitleIndex < 0 ? 'text-rose-400' : 'text-white/85'
                  }`}
                >
                  关闭字幕
                </button>
                {subs.map((track, index) => (
                  <button
                    key={track.path}
                    type="button"
                    onClick={() => {
                      onSelectSubtitle(index)
                      setSubtitleMenuOpen(false)
                    }}
                    className={`flex w-full items-center gap-2 truncate rounded-lg px-3 py-1.5 text-left text-xs transition hover:bg-white/10 ${
                      subtitleIndex === index ? 'text-rose-400' : 'text-white/85'
                    }`}
                    title={track.label || track.lang}
                  >
                    <span className="truncate">{track.label || track.lang || `字幕 ${index + 1}`}</span>
                    {subtitleIndex === index && <span className="ml-auto text-rose-400">●</span>}
                  </button>
                ))}
              </div>
            )}
          </div>
        )}

        <button
          onClick={onToggleDanmaku}
          className={
            'flex items-center gap-1.5 rounded-full px-2.5 py-1.5 text-xs font-medium transition ' +
            (danmakuOpen || danmakuEnabled
              ? 'bg-rose-500/90 text-white hover:bg-rose-500'
              : 'bg-white/10 text-white/80 hover:bg-white/20')
          }
          title="弹幕设置"
        >
          <MessageSquareText size={15} />
          弹幕
          {danmakuEnabled && <span className="h-1.5 w-1.5 rounded-full bg-lime-400" />}
        </button>

        <button
          onClick={toggleMute}
          className="rounded-full p-1.5 transition hover:bg-white/15"
          title={muted || volume === 0 ? '取消静音' : '静音'}
        >
          {muted || volume === 0 ? <VolumeX size={18} /> : <Volume2 size={18} />}
        </button>
        <input
          type="range"
          min={0}
          max={1}
          step={0.05}
          value={muted ? 0 : volume}
          onChange={(e) => changeVolume(Number(e.target.value))}
          className="w-16 accent-rose-500"
          aria-label="音量"
        />

        <button
          onClick={toggleFullscreen}
          className="rounded-full p-1.5 transition hover:bg-white/15"
          title={fullscreen ? '退出全屏' : '全屏'}
        >
          {fullscreen ? <Minimize size={18} /> : <Maximize size={18} />}
        </button>
      </div>
    </div>
  )
}