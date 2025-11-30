import { NavLink } from "react-router-dom";
import { Home, Activity, User, LogOut } from "lucide-react";
import logoImage from "@/assets/black-logo-transparent.png";
import { useAuth } from "@/contexts/AuthContext";
import { Button } from "@/components/ui/button";

export function Sidebar() {
  const { logout } = useAuth();

  return (
    <div className="flex h-screen w-64 flex-col border-r bg-white">
      {/* Logo Section */}
      <div className="flex h-16 items-center border-b px-6">
        <div className="flex items-center gap-3">
          <img
            src={logoImage}
            alt="Rocketship"
            className="h-8 w-auto flex-shrink-0"
          />
          <span className="font-semibold text-lg">Rocketship</span>
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex-1 space-y-1 px-3 py-4">
        <NavLink
          to="/dashboard"
          className={({ isActive }) =>
            `flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors ${
              isActive
                ? "bg-secondary text-foreground"
                : "text-muted-foreground hover:bg-secondary hover:text-foreground"
            }`
          }
        >
          <Home className="h-4 w-4" />
          Dashboard
        </NavLink>

        <NavLink
          to="/runs"
          className={({ isActive }) =>
            `flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors ${
              isActive
                ? "bg-secondary text-foreground"
                : "text-muted-foreground hover:bg-secondary hover:text-foreground"
            }`
          }
        >
          <Activity className="h-4 w-4" />
          Test Runs
        </NavLink>
      </nav>

      {/* User Section */}
      <div className="border-t p-4 space-y-3">
        <div className="flex items-center gap-3">
          <div className="h-8 w-8 rounded-full bg-secondary flex items-center justify-center">
            <User className="h-4 w-4 text-muted-foreground" />
          </div>
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium truncate">User</p>
            <p className="text-xs text-muted-foreground truncate">
              user@example.com
            </p>
          </div>
        </div>
        <Button
          onClick={logout}
          variant="outline"
          size="sm"
          className="w-full justify-start"
        >
          <LogOut className="h-4 w-4 mr-2" />
          Log out
        </Button>
      </div>
    </div>
  );
}
