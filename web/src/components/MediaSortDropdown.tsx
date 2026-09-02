import { useEffect, useRef, useState } from 'react'
import { ArrowDown, ArrowUp, ArrowUpDown, Check, Shuffle } from 'lucide-react'

import {
  SORT_OPTIONS,
  type SortField,
  type SortOption,
  type SortOrder,
} from '../utils/mediaSort'

type MediaSortDropdownProps = {
  value: SortField
  order: SortOrder
  onChange: (field: SortField, order: SortOrder) => void
  onReshuffle?: () => void
  className?: string
}

export function MediaSortDropdown({
  value,
  order,
  onChange,
  onReshuffle,
  className = '',
}: MediaSortDropdownProps) {
  const [isOpen, setIsOpen] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)

  const currentOption = SORT_OPTIONS.find((opt) => opt.id === value) ?? SORT_OPTIONS[0]

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false)
      }
    }
    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside)
    }
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [isOpen])

  const handleSelect = (option: SortOption) => {
    if (option.id === 'random') {
      if (value === 'random') {
        onReshuffle?.()
      } else {
        onChange('random', 'desc')
      }
      setIsOpen(false)
      return
    }

    if (value === option.id) {
      // 再次点击相同项：切换升降序
      const nextOrder: SortOrder = order === 'asc' ? 'desc' : 'asc'
      onChange(option.id, nextOrder)
    } else {
      // 点击新项：应用默认排序方向
      onChange(option.id, option.defaultOrder)
    }
    setIsOpen(false)
  }

  return (
    <div className={`relative inline-block text-left ${className}`} ref={dropdownRef}>
      <button
        type="button"
        onClick={() => setIsOpen((prev) => !prev)}
        className="inline-flex h-9 items-center gap-1.5 rounded-xl border border-sand-200 bg-white/90 px-3 py-1.5 text-xs font-semibold text-ink-600 shadow-sm transition-all hover:border-brand-300 hover:bg-brand-50/50 hover:text-brand-700"
        title="更改排序方式"
      >
        <ArrowUpDown size={14} className="text-sand-500" />
        <span>排序: {currentOption.label}</span>
        {value === 'random' ? (
          <Shuffle size={12} className="text-brand-600" />
        ) : order === 'asc' ? (
          <ArrowUp size={13} className="text-brand-600 font-bold" />
        ) : (
          <ArrowDown size={13} className="text-brand-600 font-bold" />
        )}
      </button>

      {isOpen && (
        <div className="absolute left-0 z-50 mt-1.5 w-52 max-w-[calc(100vw-2rem)] origin-top-left rounded-2xl border border-sand-200/80 bg-white/95 p-1.5 shadow-xl backdrop-blur-md transition-all animate-in fade-in-0 zoom-in-95 sm:left-auto sm:right-0 sm:origin-top-right">
          <div className="px-2.5 py-1.5 text-[11px] font-bold text-sand-500 border-b border-sand-100">
            排序方式
          </div>
          <div className="max-h-80 overflow-y-auto py-1 space-y-0.5">
            {SORT_OPTIONS.map((option) => {
              const isSelected = value === option.id
              return (
                <button
                  key={option.id}
                  type="button"
                  onClick={() => handleSelect(option)}
                  className={`flex w-full items-center justify-between rounded-xl px-3 py-2 text-xs font-medium transition-colors ${
                    isSelected
                      ? 'bg-brand-50 text-brand-700 font-semibold'
                      : 'text-ink-600 hover:bg-sand-50 hover:text-brand-600'
                  }`}
                >
                  <div className="flex items-center gap-2">
                    {isSelected && <Check size={13} className="text-brand-600" />}
                    <span className={isSelected ? '' : 'pl-5'}>{option.label}</span>
                  </div>
                  {isSelected && (
                    <div className="flex items-center text-brand-600">
                      {option.id === 'random' ? (
                        <Shuffle size={13} />
                      ) : order === 'asc' ? (
                        <ArrowUp size={14} className="stroke-[2.5]" />
                      ) : (
                        <ArrowDown size={14} className="stroke-[2.5]" />
                      )}
                    </div>
                  )}
                </button>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
