import type { LucideIcon } from 'lucide-react'
import {
  Download,
  FileOutput,
  FolderOpen,
  Library,
  Settings,
  Upload,
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
  // STRM 管理（设置区）：网盘目录生成 strm 与元数据下载/上传队列
  { to: '/strm', label: 'STRM 管理', icon: FileOutput, adminOnly: true },
  { to: '/strm/downloads', label: '下载队列', icon: Download, adminOnly: true },
  { to: '/strm/uploads', label: '上传队列', icon: Upload, adminOnly: true },
]