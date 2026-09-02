import { ArrowLeft, RefreshCw, Sparkles } from 'lucide-react'

import type { PlayerMode } from './playerPageModel'

type PlayerTopBarProps = {
  directOnly: boolean
  isDirectStream?: boolean
  directStreamLabel?: string
  mode: PlayerMode
  onBack: () => void
  onToggleMode: () => void
}

export function PlayerTopBar({
  directOnly,
  isDirectStream,
  directStreamLabel,
  mode,
  onBack,
  onToggleMode,
}: PlayerTopBarProps) {
  return (
    <div className="pointer-events-none absolute inset-x-0 top-0 z-20 flex items-center justify-between p-4 sm:p-6">
      <button
        onClick={onBack}
        className="pointer-events-auto flex items-center gap-2 rounded-full border border-white/15 bg-black/70 px-4 py-2 text-sm font-medium text-white shadow-xl backdrop-blur transition hover:bg-black/85"
      >
        <ArrowLeft size={16} /> 返回
      </button>

      <div className="pointer-events-auto flex items-center gap-2">
        {isDirectStream ? (
          <span
            className="flex items-center gap-2 rounded-full border border-white/15 bg-black/70 px-4 py-2 text-sm font-medium text-white shadow-xl backdrop-blur"
            title={directStreamLabel ? `${directStreamLabel}，默认直连播放，不进行转码` : '远程直连播放，不进行转码'}
          >
            <Sparkles size={14} /> {directStreamLabel || '直连播放'}
          </span>
        ) : directOnly ? (
          <span
            className="flex items-center gap-2 rounded-full border border-white/15 bg-black/70 px-4 py-2 text-sm font-medium text-white shadow-xl backdrop-blur"
            title="宿主机不转码，由客户端本地解码直连"
          >
            <Sparkles size={14} /> 客户端直连解码
          </span>
        ) : (
          <button
            onClick={onToggleMode}
            className="flex items-center gap-2 rounded-full border border-white/15 bg-black/70 px-4 py-2 text-sm font-medium text-white shadow-xl backdrop-blur transition hover:bg-black/85"
            title="切换播放模式"
          >
            {mode === 'hls' ? (
              <>
                <RefreshCw size={14} /> HLS 转码中
              </>
            ) : (
              <>
                <Sparkles size={14} /> 直接播放
              </>
            )}
          </button>
        )}
      </div>
    </div>
  )
}