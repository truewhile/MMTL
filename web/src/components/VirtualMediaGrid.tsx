import { forwardRef, useEffect, useState, type CSSProperties, type ReactNode } from 'react'
import { VirtuosoGrid } from 'react-virtuoso'

// 与 LibraryMediaSections 等处的海报网格保持同一套响应式列配置。
export const MEDIA_GRID_CLASS =
  'grid grid-cols-3 gap-4 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 xl:grid-cols-7 2xl:grid-cols-8'

// 首帧滚动容器尚未就绪时的占位渲染数量（约一屏）。
const FIRST_PAINT_COUNT = 28

const GridList = forwardRef<HTMLDivElement, { style?: CSSProperties; children?: ReactNode }>(
  function GridList({ children, style, ...rest }, ref) {
    return (
      <div ref={ref} style={style} {...rest} className={MEDIA_GRID_CLASS}>
        {children}
      </div>
    )
  },
)

const GridItem = forwardRef<HTMLDivElement, { style?: CSSProperties; children?: ReactNode }>(
  function GridItem({ children, style, ...rest }, ref) {
    return (
      <div ref={ref} style={style} {...rest}>
        {children}
      </div>
    )
  },
)

// VirtualMediaGrid 大库性能优化：无论库里有 1 千还是 5 万张卡片，
// 实际挂载的只有视口内及邻近一屏的节点。滚动容器是 Layout 的
// <main id="app-main-scroll">（非 window 滚动），通过 customScrollParent 接入；
// 元素尚未挂载时先渲染一屏卡片，挂载后切换为虚拟列表。
export function VirtualMediaGrid({
  totalCount,
  renderItem,
}: {
  totalCount: number
  renderItem: (index: number) => ReactNode
}) {
  const [scrollParent, setScrollParent] = useState<HTMLElement | null>(null)

  useEffect(() => {
    setScrollParent(document.getElementById('app-main-scroll'))
  }, [])

  if (!scrollParent) {
    return (
      <div className={MEDIA_GRID_CLASS}>
        {Array.from({ length: Math.min(totalCount, FIRST_PAINT_COUNT) }, (_, index) => (
          <div key={index}>{renderItem(index)}</div>
        ))}
      </div>
    )
  }

  return (
    <VirtuosoGrid
      customScrollParent={scrollParent}
      totalCount={totalCount}
      overscan={800}
      components={{ List: GridList, Item: GridItem }}
      itemContent={renderItem}
    />
  )
}
