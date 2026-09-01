import type { LucideIcon } from 'lucide-react'
import {
  FileOutput,
  FolderOpen,
  Library,
  ListChecks,
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

export const LAYOUT_NAV_ITEMS: LayoutNavItem[] = [
  { to: '/profile', label: '个人资料', icon: User },
  { to: '/libraries?from=admin', label: '媒体库', icon: Library },
  { to: '/queue', label: '任务队列', icon: ListChecks, adminOnly: true },
  { to: '/admin', label: '用户管理', icon: Users, adminOnly: true },
  { to: '/files', label: '文件管理', icon: FolderOpen, adminOnly: true },
  { to: '/settings', label: '系统设置', icon: Settings, adminOnly: true },
  // Emby 挂载（设置区）：远程 Emby 媒体库挂载
  { to: '/emby-mount', label: 'Emby 挂载', icon: Tv, adminOnly: true },
  // STRM 管理（设置区）：网盘目录生成 strm 与元数据同步
  { to: '/strm', label: 'STRM 管理', icon: FileOutput, adminOnly: true },
]