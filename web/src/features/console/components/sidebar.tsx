import { LayoutDashboard, Heart, Activity, Lock, Folder, User, Rocket } from 'lucide-react';
import { useState } from 'react';

interface SidebarProps {
  activePage: string;
  onNavigate: (page: string) => void;
  userName?: string;
  orgName?: string;
}

export function Sidebar({ activePage, onNavigate, userName = 'User', orgName = 'Organization' }: SidebarProps) {
  const [isExpanded, setIsExpanded] = useState(false);

  const navStructure = [
    {
      type: 'item' as const,
      id: 'overview',
      label: 'Overview',
      icon: LayoutDashboard
    },
    {
      type: 'header' as const,
      label: 'MONITORING'
    },
    {
      type: 'item' as const,
      id: 'test-health',
      label: 'Test Health',
      icon: Heart
    },
    {
      type: 'item' as const,
      id: 'suite-activity',
      label: 'Suite Activity',
      icon: Activity
    },
    {
      type: 'header' as const,
      label: 'CONFIGURATION'
    },
    {
      type: 'item' as const,
      id: 'projects',
      label: 'Projects',
      icon: Folder
    },
    {
      type: 'item' as const,
      id: 'environments',
      label: 'Environments & Access',
      icon: Lock
    },
  ];

  return (
    <div
      className={`bg-black text-white text-sm flex flex-col fixed left-0 top-0 bottom-0 transition-all duration-300 z-50 ${
        isExpanded ? 'w-64' : 'w-16'
      }`}
      onMouseEnter={() => setIsExpanded(true)}
      onMouseLeave={() => setIsExpanded(false)}
    >
      {/* Logo */}
      <div className="p-6 border-b border-white/10 h-[73px] flex items-center">
        <div className="flex items-center gap-3 min-w-0">
          <Rocket className="w-6 h-6 flex-shrink-0" />
          <span className={`font-semibold whitespace-nowrap transition-opacity duration-300 ${
            isExpanded ? 'opacity-100' : 'opacity-0 w-0'
          }`}>
            Rocketship Cloud
          </span>
        </div>
      </div>

      {/* Navigation */}
      <nav className={`flex-1 ${isExpanded ? 'p-4' : 'px-2 py-4'}`}>
        {navStructure.map((item, index) => {
          if (item.type === 'header') {
            return (
              <div
                key={`header-${index}`}
                className={`text-white/40 text-xs font-semibold tracking-wider mb-2 transition-opacity duration-300 ${
                  isExpanded ? 'opacity-100 px-4 mt-6 first:mt-0' : 'opacity-0 h-0 overflow-hidden'
                }`}
              >
                {item.label}
              </div>
            );
          }

          const Icon = item.icon!;
          const isActive = activePage === item.id;

          return (
            <button
              key={item.id}
              onClick={() => onNavigate(item.id!)}
              className={`flex items-center rounded-md transition-colors mb-1 ${
                isActive
                  ? 'bg-white/10 text-white'
                  : 'text-white/70 hover:bg-white/5 hover:text-white'
              } ${
                isExpanded
                  ? 'w-full gap-3 px-4 py-3 justify-start'
                  : 'w-12 h-12 justify-center mx-auto'
              }`}
              title={!isExpanded ? item.label : undefined}
            >
              <Icon className="w-5 h-5 flex-shrink-0" />
              <span className={`whitespace-nowrap transition-opacity duration-300 ${
                isExpanded ? 'opacity-100' : 'opacity-0 w-0 absolute'
              }`}>
                {item.label}
              </span>
            </button>
          );
        })}
      </nav>

      {/* Profile - Anchored Bottom */}
      <div className={`border-t border-white/10 ${isExpanded ? 'p-4' : 'p-2'}`}>
        <button
          onClick={() => onNavigate('profile')}
          className={`flex items-center rounded-md transition-colors text-white/70 hover:bg-white/5 hover:text-white ${
            activePage === 'profile' ? 'bg-white/10 text-white' : ''
          } ${
            isExpanded
              ? 'w-full gap-3 px-4 py-3 justify-start'
              : 'w-12 h-12 justify-center mx-auto'
          }`}
          title={!isExpanded ? userName : undefined}
        >
          <User className="w-5 h-5 flex-shrink-0" />
          <div className={`flex flex-col items-start transition-opacity duration-300 ${
            isExpanded ? 'opacity-100' : 'opacity-0 w-0 absolute'
          }`}>
            <span className="whitespace-nowrap text-white">
              {userName}
            </span>
            <span className="whitespace-nowrap text-xs text-white/60">
              {orgName}
            </span>
          </div>
        </button>
      </div>
    </div>
  );
}
