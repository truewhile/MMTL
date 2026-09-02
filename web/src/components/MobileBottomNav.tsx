import { Link, useLocation } from 'react-router-dom'
import clsx from 'clsx'
import { Menu } from 'lucide-react'

import { MOBILE_BOTTOM_NAV_ITEMS } from './layoutNavigation'

type MobileBottomNavProps = {
  onOpenMenu: () => void
}

export function MobileBottomNav({ onOpenMenu }: MobileBottomNavProps) {
  const location = useLocation()

  return (
    <nav
      className="fixed inset-x-0 bottom-0 z-40 border-t border-[var(--app-border)] bg-[var(--app-panel)]/95 backdrop-blur-md lg:hidden"
      style={{ paddingBottom: 'env(safe-area-inset-bottom, 0px)' }}
      aria-label="主导航"
    >
      <div className="mx-auto flex h-14 max-w-lg items-stretch justify-around px-1">
        {MOBILE_BOTTOM_NAV_ITEMS.map((item) => {
          const Icon = item.icon
          const active = item.match
            ? item.match(location.pathname)
            : item.end
              ? location.pathname === item.to
              : location.pathname === item.to || location.pathname.startsWith(`${item.to}/`)

          return (
            <Link
              key={item.to}
              to={item.to}
              className={clsx(
                'flex min-w-0 flex-1 flex-col items-center justify-center gap-0.5 rounded-xl px-1 py-1 text-[10px] font-semibold transition-colors',
                active ? 'text-brand-600' : 'text-[var(--app-muted)] hover:text-[var(--app-text)]',
              )}
            >
              <Icon size={18} strokeWidth={active ? 2.4 : 2} />
              <span className="truncate">{item.label}</span>
            </Link>
          )
        })}
        <button
          type="button"
          onClick={onOpenMenu}
          className="flex min-w-0 flex-1 flex-col items-center justify-center gap-0.5 rounded-xl px-1 py-1 text-[10px] font-semibold text-[var(--app-muted)] transition-colors hover:text-[var(--app-text)]"
          aria-label="打开更多菜单"
        >
          <Menu size={18} />
          <span>更多</span>
        </button>
      </div>
    </nav>
  )
}
