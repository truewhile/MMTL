import { useLocation, useNavigate } from 'react-router-dom'

import { useAuthStore } from '../stores/auth'
import { usePlayProfileStore } from '../stores/playProfile'
import {
  LayoutHeader,
  LayoutSidebars,
  LayoutWorkspace,
} from './LayoutSections'
import { useLayoutPermissions } from './useLayoutPermissions'
import { useLayoutProfiles } from './useLayoutProfiles'
import { useLayoutSidebar } from './useLayoutSidebar'
import { useThemeMode } from './useThemeMode'

function isMediaView(pathname: string, search: string): boolean {
  const params = new URLSearchParams(search)
  // 从设置/管理后台菜单进入（携带 from=admin 或 from=settings 或 manage=1）时展示左侧栏
  if (params.get('from') === 'admin' || params.get('from') === 'settings' || params.get('manage') === '1') {
    return false
  }

  return (
    pathname === '/' ||
    pathname === '/libraries' ||
    pathname.startsWith('/library') ||
    pathname.startsWith('/media') ||
    pathname.startsWith('/play') ||
    pathname === '/favourites' ||
    pathname === '/playlists' ||
    pathname.startsWith('/playlist') ||
    pathname === '/history' ||
    pathname === '/poster-wall'
  )
}

export function Layout() {
  const navigate = useNavigate()
  const location = useLocation()
  const user = useAuthStore((s) => s.user)
  const logout = useAuthStore((s) => s.logout)
  const activeProfileId = usePlayProfileStore((s) => s.activeProfileId)
  const setActiveProfile = usePlayProfileStore((s) => s.setActiveProfile)
  const theme = useThemeMode()
  const permissions = useLayoutPermissions(user)
  const sidebar = useLayoutSidebar(location.pathname)
  const profile = useLayoutProfiles({ activeProfileId, setActiveProfile, user })

  const handleLogout = () => { logout(); navigate('/login') }
  const closeProfileAndLogout = () => { profile.setIsProfileOpen(false); handleLogout() }

  const showSidebar = !isMediaView(location.pathname, location.search)
  const hideSearch = location.pathname.startsWith('/settings')

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-[var(--app-bg)] text-[var(--app-text)] font-body select-none">
      <LayoutSidebars
        sidebar={sidebar}
        isAdmin={permissions.isAdmin}
        can={permissions.can}
        showSidebar={showSidebar}
      />
      <div className="flex flex-1 flex-col min-w-0 overflow-hidden">
        <LayoutHeader
          permissions={permissions}
          theme={theme}
          onOpenMobileDrawer={() => sidebar.setIsMobileDrawerOpen(true)}
          user={user}
          activeProfileId={activeProfileId}
          profile={profile}
          onLogout={closeProfileAndLogout}
          showSidebar={showSidebar}
          hideSearch={hideSearch}
        />
        <LayoutWorkspace routeKey={location.pathname} />
      </div>
    </div>
  )
}
