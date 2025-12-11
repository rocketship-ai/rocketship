import { type ReactNode, useRef, useCallback, useSyncExternalStore } from 'react'
import { AppSidebar } from './AppSidebar'
import { SidebarProvider, SidebarInset, SidebarTrigger } from '@/components/ui/sidebar'

interface DashboardLayoutProps {
  children: ReactNode
}

// Global sidebar state that persists across React re-renders and navigation
const sidebarState = {
  isOpen: false,
  isHovering: false,
  listeners: new Set<() => void>(),

  setOpen(open: boolean) {
    if (this.isOpen !== open) {
      this.isOpen = open
      this.listeners.forEach(listener => listener())
    }
  },

  subscribe(listener: () => void) {
    this.listeners.add(listener)
    return () => this.listeners.delete(listener)
  },

  getSnapshot() {
    return sidebarState.isOpen
  }
}

// Track hover state globally
let hoverTimeout: ReturnType<typeof setTimeout> | null = null

export function DashboardLayout({ children }: DashboardLayoutProps) {
  const sidebarRef = useRef<HTMLDivElement>(null)

  // Use external store so state persists across navigation
  const isOpen = useSyncExternalStore(
    sidebarState.subscribe.bind(sidebarState),
    sidebarState.getSnapshot
  )

  const handleMouseEnter = useCallback(() => {
    if (hoverTimeout) {
      clearTimeout(hoverTimeout)
      hoverTimeout = null
    }
    sidebarState.isHovering = true
    sidebarState.setOpen(true)
  }, [])

  const handleMouseLeave = useCallback(() => {
    sidebarState.isHovering = false
    hoverTimeout = setTimeout(() => {
      if (!sidebarState.isHovering) {
        sidebarState.setOpen(false)
      }
    }, 150)
  }, [])

  const handleOpenChange = useCallback((open: boolean) => {
    // Prevent closing if mouse is still hovering
    if (!open && sidebarState.isHovering) {
      return
    }
    sidebarState.setOpen(open)
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
