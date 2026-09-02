import { ArrowLeft } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import clsx from 'clsx'

type PageBackButtonProps = {
  to?: string
  label?: string
  onClick?: () => void
  className?: string
  compact?: boolean
}

export function PageBackButton({
  to,
  label = '返回',
  onClick,
  className,
  compact = false,
}: PageBackButtonProps) {
  const navigate = useNavigate()

  const handleClick = () => {
    if (onClick) {
      onClick()
      return
    }
    if (to) {
      navigate(to)
      return
    }
    navigate(-1)
  }

  return (
    <button
      type="button"
      onClick={handleClick}
      className={clsx(
        'inline-flex items-center gap-2 rounded-xl border border-[var(--app-border)] bg-[var(--app-panel)] px-3 py-2 text-sm font-semibold text-[var(--app-subtle)] shadow-sm transition-colors hover:bg-[var(--app-hover)] hover:text-[var(--app-text)]',
        compact && 'px-2.5 py-1.5 text-xs',
        className,
      )}
    >
      <ArrowLeft size={compact ? 15 : 16} />
      <span>{label}</span>
    </button>
  )
}
