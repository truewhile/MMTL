import { useEffect, useRef, useState } from 'react'
import type { ReactNode, RefObject } from 'react'

import { subtitlesAPI, type SubtitleTrack } from '../api/subtitles'
import { type DanmakuAnime } from '../api/danmaku'
import type { Media } from '../types'
import { DanmakuStage } from '../components/DanmakuStage'
import { PlayerControls } from '../components/PlayerControls'

type PlayerVideoStageProps = {
  media: Media | null
  playerError: string
  subs: SubtitleTrack[]
  /** 当前激活字幕轨道：-1=关闭，0..n-1=对应轨道。 */
  subtitleIndex: number
  onSelectSubtitle: (index: number) => void
  videoRef: RefObject<HTMLVideoElement>
  onVideoError: () => void
  danmakuEnabled: boolean
  danmakuOpacity: number
  danmakuFontSize: number
  danmakuArea: number
  danmakuSearch: string | null
  danmakuEpisodeId: number | string | null
  danmakuOpen: boolean
  onToggleDanmaku: () => void
  onDanmakuLoaded: () => void
  onDanmakuCandidates: (candidates: DanmakuAnime[]) => void
  /** Danmaku settings panel; rendered inside the stage so it stays visible in fullscreen. */
  danmakuPanel: ReactNode
}

export function PlayerVideoStage({
  media,
  playerError,
  subs,
  subtitleIndex,
  onSelectSubtitle,
  videoRef,
  onVideoError,
  danmakuEnabled,
  danmakuOpacity,
  danmakuFontSize,
  danmakuArea,
  danmakuSearch,
  danmakuEpisodeId,
  danmakuOpen,
  onToggleDanmaku,
  onDanmakuLoaded,
  onDanmakuCandidates,
  danmakuPanel,
}: PlayerVideoStageProps) {
  const stageRef = useRef<HTMLDivElement>(null)
  const [videoRatio, setVideoRatio] = useState<number | null>(null)
  const [stageRect, setStageRect] = useState<{ width: number; height: number } | null>(null)

  // 监听舞台容器的真实尺寸（响应窗口大小调整和全屏切换）
  useEffect(() => {
    const stage = stageRef.current
    if (!stage) return
    const ro = new ResizeObserver((entries) => {
      const entry = entries[0]
      if (entry) {
        setStageRect({
          width: entry.contentRect.width,
          height: entry.contentRect.height,
        })
      }
    })
    ro.observe(stage)
    return () => ro.disconnect()
  }, [])

  // 监听视频元数据加载，获取真实画面宽高比
  useEffect(() => {
    const video = videoRef.current
    if (!video) return
    const updateRatio = () => {
      if (video.videoWidth && video.videoHeight) {
        setVideoRatio(video.videoWidth / video.videoHeight)
      }
    }
    updateRatio()
    video.addEventListener('loadedmetadata', updateRatio)
    video.addEventListener('resize', updateRatio)
    return () => {
      video.removeEventListener('loadedmetadata', updateRatio)
      video.removeEventListener('resize', updateRatio)
    }
  }, [videoRef, media])

  // 点击视频切换播放/暂停；双击切换全屏（控制栏事件自行阻止冒泡）。
  const togglePlay = () => {
    const video = videoRef.current
    if (!video) return
    if (video.paused) void video.play()?.catch(() => undefined)
    else video.pause()
  }
  const toggleFullscreen = () => {
    const stage = stageRef.current
    if (!stage) return
    if (document.fullscreenElement) void document.exitFullscreen()
    else void stage.requestFullscreen?.()
  }

  // 把用户选择的字幕轨道应用到 <video> 的 textTracks（跨浏览器显式设置
  // mode；<track default> 只影响初始值，部分浏览器不会自动显示）。
  //
  // 通过 React 渲染的 <track> 元素定位而不是 textTracks 索引：hls.js 等
  // 可能向 textTracks 注入内部轨道（如 CEA-608 captions），索引会错位。
  useEffect(() => {
    const video = videoRef.current
    if (!video || subs.length === 0) return
    const timers = new Set<ReturnType<typeof setTimeout>>()
    const delay = (fn: () => void, ms: number) => {
      const id = setTimeout(fn, ms)
      timers.add(id)
    }

    const apply = () => {
      const trackEls = Array.from(video.querySelectorAll('track'))
      if (trackEls.length === 0) return
      trackEls.forEach((el, i) => {
        const tt = el.track
        if (tt) tt.mode = i === subtitleIndex ? 'showing' : 'disabled'
      })
      // Chrome 把轨道从 disabled 切回 showing 时是异步重新拉取字幕；
      // 若拉取因竞态没有发生（例如切换过程中视频重载），cues 会一直为空，
      // 表现为「字幕已选中但不显示」。检测到空轨道后强制重载一次兜底。
      const selected = trackEls[subtitleIndex]
      if (!selected) return
      delay(() => {
        const tt = selected.track
        if (tt && tt.mode === 'showing' && (!tt.cues || tt.cues.length === 0)) {
          const src = selected.getAttribute('src')
          if (src) {
            selected.setAttribute('src', '')
            selected.setAttribute('src', src)
            tt.mode = 'showing'
          }
        }
      }, 1200)
    }

    apply()
    // 轨道元数据就绪后再应用一次，确保字幕真正可见
    video.addEventListener('loadedmetadata', apply)
    return () => {
      video.removeEventListener('loadedmetadata', apply)
      timers.forEach(clearTimeout)
    }
  }, [subtitleIndex, subs, videoRef])

  // 根据视频画面宽高比与舞台宽高比，确定视频在哪个轴向撑满 100%
  const isWiderThanStage =
    videoRatio && stageRect && stageRect.height > 0
      ? videoRatio > stageRect.width / stageRect.height
      : true

  const wrapperStyle = videoRatio
    ? {
        aspectRatio: `${videoRatio}`,
        width: isWiderThanStage ? '100%' : 'auto',
        height: isWiderThanStage ? 'auto' : '100%',
        maxWidth: '100%',
        maxHeight: '100%',
      }
    : {
        width: '100%',
        height: '100%',
      }

  return (
    <div
      ref={stageRef}
      data-player-stage
      className="relative flex h-full w-full flex-1 items-center justify-center overflow-hidden bg-black"
      onClick={togglePlay}
      onDoubleClick={toggleFullscreen}
    >
      {media ? (
        <>
          <div
            className="relative flex items-center justify-center overflow-hidden"
            style={wrapperStyle}
          >
            <video
              ref={videoRef}
              autoPlay
              playsInline
              className="h-full w-full object-contain bg-black"
              onError={onVideoError}
            >
              {subs.map((track) => (
                <track
                  key={track.path}
                  kind="subtitles"
                  src={subtitlesAPI.url(media.id, track.path)}
                  srcLang={track.lang}
                  label={track.label || track.lang}
                />
              ))}
            </video>
            <DanmakuStage
              media={media}
              videoRef={videoRef}
              enabled={danmakuEnabled}
              opacity={danmakuOpacity}
              fontSize={danmakuFontSize}
              area={danmakuArea}
              search={danmakuSearch}
              episodeId={danmakuEpisodeId}
              onLoaded={onDanmakuLoaded}
              onCandidates={onDanmakuCandidates}
            />
          </div>
          <PlayerControls
            videoRef={videoRef}
            subs={subs}
            subtitleIndex={subtitleIndex}
            onSelectSubtitle={onSelectSubtitle}
            danmakuOpen={danmakuOpen}
            danmakuEnabled={danmakuEnabled}
            onToggleDanmaku={onToggleDanmaku}
          />
          {danmakuPanel}
        </>
      ) : (
        <p className="text-sand-500">加载中…</p>
      )}
      {playerError ? (
        <div className="absolute bottom-20 left-1/2 w-[min(92vw,720px)] -translate-x-1/2 rounded-2xl border border-white/15 bg-black/75 px-5 py-4 text-sm text-white shadow-2xl backdrop-blur">
          {playerError}
        </div>
      ) : null}
    </div>
  )
}
