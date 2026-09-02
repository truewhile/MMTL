import type { LucideIcon } from 'lucide-react'
import {
  Clock,
  FileOutput,
  FolderOpen,
  Heart,
  Home,
  Library,
  ListChecks,
  ListMusic,
  Settings,
  Tv,
  User,
  Users,
} from 'lucide-react'

export type LayoutNavItem = {
  to: string
  label: string
  icon: LucideIcon
  end?: boolean
  permission?: string
  adminOnly?: boolean
}

export type HeaderBackTarget = {
  to: string
  label: string
}

export type MobileBottomNavItem = {
  to: string
  label: string
  icon: LucideIcon
  end?: boolean
  match?: (pathname: string) => boolean
}

/** Desktop admin sidebar and admin mobile drawer. */
export const LAYOUT_NAV_ITEMS: LayoutNavItem[] = [
  { to: '/profile', label: '个人资料', icon: User },
  { to: '/libraries?from=admin', label: '媒体库', icon: Library },
  { to: '/queue', label: '任务队列', icon: ListChecks, adminOnly: true },
  { to: '/admin', label: '用户管理', icon: Users, adminOnly: true },
  { to: '/files', label: '文件管理', icon: FolderOpen, adminOnly: true },
  { to: '/settings', label: '系统设置', icon: Settings, adminOnly: true },
  { to: '/emby-mount', label: 'Emby 挂载', icon: Tv, adminOnly: true },
  { to: '/strm', label: 'STRM 管理', icon: FileOutput, adminOnly: true },
]

/** Media browsing links for the mobile drawer when browsing libraries. */
export const MEDIA_NAV_ITEMS: LayoutNavItem[] = [
  { to: '/', label: '首页', icon: Home, end: true },
  { to: '/libraries', label: '媒体库', icon: Library },
  { to: '/favourites', label: '我的收藏', icon: Heart },
  { to: '/playlists', label: '播放列表', icon: ListMusic },
  { to: '/history', label: '观看历史', icon: Clock },
]

export const MOBILE_BOTTOM_NAV_ITEMS: MobileBottomNavItem[] = [
  { to: '/', label: '首页', icon: Home, end: true },
  {
    to: '/libraries',
    label: '媒体库',
    icon: Library,
    match: (p) => p === '/libraries' || p.startsWith('/library'),
  },
  { to: '/favourites', label: '收藏', icon: Heart },
  {
    to: '/playlists',
    label: '列表',
    icon: ListMusic,
    match: (p) => p === '/playlists' || p.startsWith('/playlist'),
  },
]

const ROOT_MEDIA_PATHS = new Set(['/', '/libraries', '/favourites', '/playlists', '/history'])

/** True only for the fullscreen player route (`/play` or `/play/:id`), not `/playlists`. */
export function isPlayerRoute(pathname: string): boolean {
  return pathname === '/play' || pathname.startsWith('/play/')
}

export function isRootMediaPath(pathname: string): boolean {
  return ROOT_MEDIA_PATHS.has(pathname)
}

export function resolveHeaderBack(pathname: string): HeaderBackTarget | null {
  if (pathname.startsWith('/library/')) {
    return { to: '/libraries', label: '媒体库' }
  }
  if (pathname === '/playlists' || pathname === '/favourites' || pathname === '/history') {
    return { to: '/', label: '首页' }
  }
  if (pathname.startsWith('/playlist/')) {
    return { to: '/playlists', label: '播放列表' }
  }
  if (pathname.startsWith('/media/')) {
    return null
  }
  if (pathname === '/scraper/queue') {
    return { to: '/queue', label: '任务队列' }
  }
  if (pathname === '/strm/downloads' || pathname === '/strm/uploads') {
    return { to: '/strm', label: 'STRM 管理' }
  }
  if (
    pathname === '/profile' ||
    pathname === '/dlna' ||
    pathname === '/play-profiles' ||
    pathname === '/poster-wall'
  ) {
    return { to: '/', label: '首页' }
  }
  if (
    pathname === '/settings' ||
    pathname === '/admin' ||
    pathname === '/files' ||
    pathname === '/queue' ||
    pathname === '/emby-mount' ||
    pathname === '/strm'
  ) {
    return { to: '/', label: '首页' }
  }
  return null
}

export function shouldShowMobileBottomNav(pathname: string): boolean {
  if (isPlayerRoute(pathname)) return false
  return (
    pathname === '/' ||
    pathname === '/libraries' ||
    pathname.startsWith('/library') ||
    pathname.startsWith('/media') ||
    pathname === '/favourites' ||
    pathname === '/playlists' ||
    pathname.startsWith('/playlist') ||
    pathname === '/history' ||
    pathname === '/poster-wall'
  )
}
