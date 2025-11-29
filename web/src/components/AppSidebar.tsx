import { NavLink } from "react-router-dom";
import { Home, Activity, User, LogOut } from "lucide-react";
import logoImage from "@/assets/black-logo-transparent.png";
import { useAuth } from "@/contexts/AuthContext";
import { Button } from "@/components/ui/button";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from "@/components/ui/sidebar";

const navItems = [
  {
    title: "Dashboard",
    icon: Home,
    url: "/dashboard",
  },
  {
    title: "Test Runs",
    icon: Activity,
    url: "/runs",
  },
];

export function AppSidebar() {
  const { logout, userData } = useAuth();
  const { state } = useSidebar();
  const isCollapsed = state === "collapsed";

  return (
    <Sidebar collapsible="icon" className="border-r border-gray-200">
      {/* Logo Section */}
      <SidebarHeader className="border-b border-gray-100 px-4 py-4">
        <div className="flex items-center gap-3">
          <img
            src={logoImage}
            alt="Rocketship"
            className="h-8 w-8 flex-shrink-0"
          />
          {!isCollapsed && (
            <span className="font-semibold text-base text-gray-900">
              Rocketship
            </span>
          )}
        </div>
      </SidebarHeader>

      {/* Navigation */}
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              {navItems.map((item) => (
                <SidebarMenuItem key={item.title}>
                  <SidebarMenuButton asChild tooltip={item.title}>
                    <NavLink
                      to={item.url}
                      className={({ isActive }) =>
                        isActive
                          ? "bg-gray-100 text-gray-900"
                          : "text-gray-600 hover:bg-gray-50 hover:text-gray-900"
                      }
                    >
                      <item.icon className="h-4 w-4" />
                      <span>{item.title}</span>
                    </NavLink>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>

      {/* User Section */}
      <SidebarFooter className="border-t border-gray-100 p-3">
        <div className="flex items-center gap-3 mb-2">
          <div className="h-8 w-8 rounded-full bg-gray-100 flex items-center justify-center flex-shrink-0">
            <User className="h-4 w-4 text-gray-500" />
          </div>
          {!isCollapsed && (
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium text-gray-900 truncate">
                {userData?.user?.name || "User"}
              </p>
              <p className="text-xs text-gray-500 truncate">
                {userData?.user?.email || "user@example.com"}
              </p>
            </div>
          )}
        </div>
        {!isCollapsed ? (
          <Button
            onClick={logout}
            variant="outline"
            size="sm"
            className="w-full justify-start text-gray-600 hover:text-gray-900 border-gray-200"
          >
            <LogOut className="h-4 w-4 mr-2" />
            Log out
          </Button>
        ) : (
          <Button
            onClick={logout}
            variant="ghost"
            size="icon"
            className="w-full text-gray-600 hover:text-gray-900"
            title="Log out"
          >
            <LogOut className="h-4 w-4" />
          </Button>
        )}
      </SidebarFooter>
    </Sidebar>
  );
}
