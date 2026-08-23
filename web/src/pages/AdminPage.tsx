import { Users } from 'lucide-react'

import { AdminUsersPanel } from './AdminUsersPanel'

export function AdminPage() {
  return (
    <div className="space-y-6">
      <header className="flex items-center gap-3">
        <Users className="h-6 w-6 text-brand-500" />
        <div>
          <h1 className="font-display text-3xl font-bold text-ink-600">用户管理</h1>
          <p className="text-sm text-ink-50">
            管理系统用户账号、角色权限以及密码重置与启禁用操作。
          </p>
        </div>
      </header>

      <AdminUsersPanel />
    </div>
  )
}
