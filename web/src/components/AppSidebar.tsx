import { NavLink } from "react-router-dom";
import { Home, Activity, User, LogOut } from "lucide-react";
import logoImage from "@/assets/white-logo-transparent.png";
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
    <Sidebar collapsible="icon" className="border-r-0 bg-black">
      {/* Logo Section */}
      <SidebarHeader className={`border-b border-gray-800 py-4 ${isCollapsed ? "px-2" : "px-4"}`}>
        <div className={`flex items-center ${isCollapsed ? "justify-center" : "gap-3"}`}>
          <img
            src={logoImage}
            alt="Rocketship"
            className="h-10 w-10 flex-shrink-0 object-contain"
          />
          {!isCollapsed && (
            <span className="font-semibold text-lg text-white whitespace-nowrap">
              Rocketship Cloud
            </span>
          )}
        </div>
      </SidebarHeader>

      {/* Navigation */}
      <SidebarContent className="pt-4">
        <SidebarGroup className={isCollapsed ? "px-0" : "px-3"}>
          <SidebarGroupContent>
            <SidebarMenu className={`gap-2 ${isCollapsed ? "items-center" : ""}`}>
              {navItems.map((item) => (
                <SidebarMenuItem key={item.title}>
                  <SidebarMenuButton asChild tooltip={item.title}>
                    <NavLink
                      to={item.url}
                      className={({ isActive }) =>
                        `flex items-center ${isCollapsed ? "justify-center" : "gap-4"} rounded-lg px-3 py-3 transition-colors ${
                          isActive
                            ? "bg-gray-800/80 text-white"
                            : "text-gray-400 hover:bg-gray-800/50 hover:text-white"
                        }`
                      }
                    >
                      <item.icon className="h-6 w-6 flex-shrink-0" />
                      {!isCollapsed && (
                        <span className="text-lg">{item.title}</span>
                      )}
                    </NavLink>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>

      {/* User Section */}
      <SidebarFooter className="border-t border-gray-800 p-3">
        <div className={`flex items-center ${isCollapsed ? "justify-center" : "gap-3"} mb-2`}>
          <div className="h-8 w-8 rounded-full bg-gray-800 flex items-center justify-center flex-shrink-0">
            <User className="h-5 w-5 text-gray-400" />
          </div>
          {!isCollapsed && (
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium text-white truncate">
                {userData?.user?.name || "User"}
              </p>
              <p className="text-xs text-gray-400 truncate">
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
            className="w-full justify-start text-gray-400 hover:text-white border-gray-700 bg-transparent hover:bg-gray-800"
          >
            <LogOut className="h-4 w-4 mr-2" />
            Log out
          </Button>
        ) : (
          <Button
            onClick={logout}
            variant="ghost"
            size="icon"
            className="w-full h-10 text-gray-400 hover:text-white hover:bg-gray-800"
            title="Log out"
          >
            <LogOut className="h-5 w-5" />
          </Button>
        )}
      </SidebarFooter>
    </Sidebar>
  );
}
