import type { ReactNode } from 'react'

import { PageBackButton } from './PageBackButton'

type PageHeaderProps = {
  title: string
  description?: string
  backTo?: string
  backLabel?: string
  onBack?: () => void
  actions?: ReactNode
  className?: string
}

export function PageHeader({
  title,
  description,
  backTo,
  backLabel,
  onBack,
  actions,
  className,
}: PageHeaderProps) {
  const showBack = Boolean(backTo || onBack)

  return (
    <header className={className ?? 'space-y-3'}>
      {showBack && <PageBackButton to={backTo} label={backLabel} onClick={onBack} />}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div className="min-w-0">
          <h1 className="font-display text-2xl font-bold text-ink-600 sm:text-3xl">{title}</h1>
          {description ? <p className="mt-0.5 text-xs text-ink-50 sm:text-sm">{description}</p> : null}
        </div>
        {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
      </div>
    </header>
  )
}
