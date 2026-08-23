import type { LucideIcon } from 'lucide-react'
import {
  FolderOpen,
  Library,
  Settings,
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
  { to: '/admin', label: '用户管理', icon: Users, adminOnly: true },
  { to: '/files', label: '文件管理', icon: FolderOpen, adminOnly: true },
  { to: '/settings', label: '系统设置', icon: Settings, adminOnly: true },
]