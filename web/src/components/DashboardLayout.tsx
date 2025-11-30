import { type ReactNode, useState, useRef, useCallback, useEffect } from 'react'
import { AppSidebar } from './AppSidebar'
import { SidebarProvider, SidebarInset, SidebarTrigger } from '@/components/ui/sidebar'

interface DashboardLayoutProps {
  children: ReactNode
}

// Track mouse position globally to persist across re-renders
let globalMouseX = 0
document.addEventListener('mousemove', (e) => {
  globalMouseX = e.clientX
})

export function DashboardLayout({ children }: DashboardLayoutProps) {
  const sidebarRef = useRef<HTMLDivElement>(null)
  const [isOpen, setIsOpen] = useState(() => {
    // Initialize based on current mouse position
    // Sidebar collapsed width is ~56px, expanded is ~256px
    return globalMouseX < 256
  })
  const isHoveringRef = useRef(globalMouseX < 256)
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Check if mouse is over sidebar on mount (handles navigation case)
  useEffect(() => {
    const checkMousePosition = () => {
      if (sidebarRef.current) {
        const rect = sidebarRef.current.getBoundingClientRect()
        const isOver = globalMouseX >= rect.left && globalMouseX <= rect.right
        if (isOver && !isOpen) {
          isHoveringRef.current = true
          setIsOpen(true)
        }
      }
    }
    // Small delay to let the DOM settle after navigation
    const timer = setTimeout(checkMousePosition, 50)
    return () => clearTimeout(timer)
  }, [children]) // Re-check when children change (navigation)

  const handleMouseEnter = useCallback(() => {
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current)
      timeoutRef.current = null
    }
    isHoveringRef.current = true
    setIsOpen(true)
  }, [])

  const handleMouseLeave = useCallback(() => {
    isHoveringRef.current = false
    timeoutRef.current = setTimeout(() => {
      if (!isHoveringRef.current) {
        setIsOpen(false)
      }
    }, 150)
  }, [])

  const handleOpenChange = useCallback((open: boolean) => {
    if (!open && isHoveringRef.current) {
      return
    }
    setIsOpen(open)
  }, [])

  return (
    <SidebarProvider open={isOpen} onOpenChange={handleOpenChange}>
      <div
        ref={sidebarRef}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
        className="h-full"
      >
        <AppSidebar />
      </div>
      <SidebarInset className="bg-[#fafafa]">
        {/* Mobile header with menu trigger */}
        <div className="flex items-center gap-2 p-4 md:hidden border-b border-gray-200">
          <SidebarTrigger className="h-8 w-8" />
          <span className="font-semibold text-gray-900">Rocketship Cloud</span>
        </div>
        {children}
      </SidebarInset>
    </SidebarProvider>
  )
}
