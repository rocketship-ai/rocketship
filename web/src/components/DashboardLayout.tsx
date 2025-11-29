import { type ReactNode, useState, useRef } from 'react'
import { AppSidebar } from './AppSidebar'
import { SidebarProvider, SidebarInset } from '@/components/ui/sidebar'

interface DashboardLayoutProps {
  children: ReactNode
}

export function DashboardLayout({ children }: DashboardLayoutProps) {
  const [isOpen, setIsOpen] = useState(false)
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const handleMouseEnter = () => {
    // Clear any pending close timeout
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current)
      timeoutRef.current = null
    }
    setIsOpen(true)
  }

  const handleMouseLeave = () => {
    // Add a small delay before closing to prevent flicker
    timeoutRef.current = setTimeout(() => {
      setIsOpen(false)
    }, 150)
  }

  return (
    <SidebarProvider open={isOpen} onOpenChange={setIsOpen}>
      <div
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
        className="h-full"
      >
        <AppSidebar />
      </div>
      <SidebarInset className="bg-[#fafafa]">
        {children}
      </SidebarInset>
    </SidebarProvider>
  )
}
