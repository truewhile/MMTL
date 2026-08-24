import { useEffect } from 'react'
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
  // 点击视频切换播放/暂停；双击切换全屏（控制栏事件自行阻止冒泡）。
  const togglePlay = () => {
    const video = videoRef.current
    if (!video) return
    if (video.paused) void video.play()?.catch(() => undefined)
    else video.pause()
  }
  const toggleFullscreen = () => {
    const stage = videoRef.current?.parentElement
    if (!stage) return
    if (document.fullscreenElement) void document.exitFullscreen()
    else void stage.requestFullscreen?.()
  }

  // 把用户选择的字幕轨道应用到 <video> 的 textTracks（跨浏览器显式设置
  // mode；<track default> 只影响初始值，部分浏览器不会自动显示）。
  useEffect(() => {
    const video = videoRef.current
    if (!video || subs.length === 0) return
    const apply = () => {
      const tracks = video.textTracks
      for (let i = 0; i < tracks.length; i++) {
        tracks[i].mode = i === subtitleIndex ? 'showing' : 'disabled'
      }
    }
    apply()
    // 轨道元数据就绪后再应用一次，确保字幕真正可见
    video.addEventListener('loadedmetadata', apply)
    return () => video.removeEventListener('loadedmetadata', apply)
  }, [subtitleIndex, subs, videoRef])

  return (
    <div
      className="relative flex flex-1 items-center justify-center overflow-hidden bg-black"
      onClick={togglePlay}
      onDoubleClick={toggleFullscreen}
    >
      {media ? (
        <>
          <video
            ref={videoRef}
            autoPlay
            playsInline
            className="relative z-0 max-h-screen w-full max-w-[1600px] bg-black"
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