import { useCallback, useEffect, useState } from 'react'

export function useLayoutSidebar(pathname: string) {
  const [isSidebarOpen, setIsSidebarOpen] = useState(true)
  const [isMobileDrawerOpen, setIsMobileDrawerOpen] = useState(false)

  useEffect(() => {
    const handleResize = () => {
      setIsSidebarOpen(window.innerWidth >= 1024)
    }
    handleResize()
    window.addEventListener('resize', handleResize)
    return () => window.removeEventListener('resize', handleResize)
  }, [])

  useEffect(() => {
    setIsMobileDrawerOpen(false)
  }, [pathname])

  const toggleSidebar = useCallback(() => {
    setIsSidebarOpen((current) => !current)
  }, [])

  return {
    isMobileDrawerOpen,
    isSidebarOpen,
    setIsMobileDrawerOpen,
    setIsSidebarOpen,
    toggleSidebar,
  }
}