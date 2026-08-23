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
            {subs.map((track, index) => (
              <track
                key={track.path}
                kind="subtitles"
                src={subtitlesAPI.url(media.id, track.path)}
                srcLang={track.lang}
                label={track.label || track.lang}
                default={index === 0}
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