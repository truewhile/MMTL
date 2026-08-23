import { Link } from 'react-router-dom'
import { motion } from 'framer-motion'
import { ArrowLeft, Menu, X } from 'lucide-react'
import clsx from 'clsx'
import { LAYOUT_NAV_ITEMS, type LayoutNavItem } from './layoutNavigation'
import { SidebarLink } from './LayoutSidebarNav'

export type LayoutSidebarContentProps = {
  isSidebarOpen: boolean
  isMobileDrawerOpen: boolean
  isAdmin: boolean
  can: (key: string) => boolean
  onToggleSidebar: () => void
  onCloseMobileDrawer: () => void
}

export function LayoutSidebarContent({
  isSidebarOpen,
  isMobileDrawerOpen,
  isAdmin,
  can,
  onToggleSidebar,
  onCloseMobileDrawer,
}: LayoutSidebarContentProps) {
  const sidebarExpanded = isSidebarOpen || isMobileDrawerOpen
  const visibleItems = visibleSidebarItems({ isAdmin, can })

  return (
    <div className="flex h-full flex-col border-r border-[var(--app-border)] bg-[var(--app-panel)]">
      <LayoutSidebarHeader
        sidebarExpanded={sidebarExpanded}
        onToggleSidebar={onToggleSidebar}
        onCloseMobileDrawer={onCloseMobileDrawer}
      />
      <LayoutSidebarNav items={visibleItems} sidebarExpanded={sidebarExpanded} />
      <LayoutSidebarHomeBack sidebarExpanded={sidebarExpanded} />
    </div>
  )
}

function visibleSidebarItems({
  isAdmin,
  can,
}: Pick<LayoutSidebarContentProps, 'isAdmin' | 'can'>): LayoutNavItem[] {
  return LAYOUT_NAV_ITEMS.filter(
    (item) => (!item.adminOnly || isAdmin) && (!item.permission || can(item.permission)),
  )
}

function LayoutSidebarHeader({
  sidebarExpanded,
  onToggleSidebar,
  onCloseMobileDrawer,
}: {
  sidebarExpanded: boolean
  onToggleSidebar: () => void
  onCloseMobileDrawer: () => void
}) {
  return (
    <div className="flex h-20 items-center justify-between border-b border-[var(--app-border)] px-6">
      <Link to="/" className="flex items-center gap-3">
        <img
          src="/brand/logo-192.png"
          alt="MMTL"
          className="h-10 w-10 shrink-0 rounded-xl object-contain shadow-sm"
        />
        {sidebarExpanded && (
          <motion.span
            initial={{ opacity: 0, x: -10 }}
            animate={{ opacity: 1, x: 0 }}
            className="font-display text-lg font-extrabold tracking-tight text-[var(--app-text)]"
          >
            MMTL
          </motion.span>
        )}
      </Link>
      <SidebarIconButton className="hidden lg:block" onClick={onToggleSidebar}>
        <Menu size={18} />
      </SidebarIconButton>
      <SidebarIconButton className="block lg:hidden" onClick={onCloseMobileDrawer}>
        <X size={18} />
      </SidebarIconButton>
    </div>
  )
}

function LayoutSidebarNav({
  items,
  sidebarExpanded,
}: {
  items: LayoutNavItem[]
  sidebarExpanded: boolean
}) {
  return (
    <nav className="flex-1 overflow-y-auto px-4 py-5 space-y-1 scrollbar-hide">
      {items.map((item) => {
        const ItemIcon = item.icon
        return (
          <SidebarLink
            key={item.to}
            to={item.to}
            icon={<ItemIcon size={18} />}
            label={item.label}
            end={item.end}
            collapsed={!sidebarExpanded}
          />
        )
      })}
    </nav>
  )
}

function LayoutSidebarHomeBack({ sidebarExpanded }: { sidebarExpanded: boolean }) {
  return (
    <div className="border-t border-[var(--app-border)] bg-[var(--app-panel-soft)] p-4">
      <Link
        to="/"
        className={clsx(
          'flex items-center gap-3.5 rounded-xl px-4 py-3 text-sm font-semibold transition-all duration-300 w-full group',
          sidebarExpanded
            ? 'justify-start text-[var(--app-muted)] hover:bg-[var(--app-hover)] hover:text-[var(--app-text)]'
            : 'justify-center text-[var(--app-muted)] hover:text-[var(--app-text)]',
        )}
        title="返回系统首页"
      >
        <ArrowLeft size={18} className="transition-transform group-hover:-translate-x-0.5" />
        {sidebarExpanded && <span>返回系统首页</span>}
      </Link>
    </div>
  )
}

function SidebarIconButton({
  children,
  className,
  onClick,
}: {
  children: React.ReactNode
  className: string
  onClick: () => void
}) {
  return (
    <button
      onClick={onClick}
      className={clsx(
        'rounded-xl p-1.5 text-[var(--app-muted)] hover:bg-[var(--app-hover)] hover:text-[var(--app-text)] transition-colors',
        className,
      )}
    >
      {children}
    </button>
  )
}