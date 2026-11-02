import {
  ArrowRight,
  ChevronDown,
  CircleHelp,
  Command,
  MoreHorizontal,
  LogOut,
  Search,
  Settings2,
  X,
  Zap,
} from 'lucide-react'
import type { ComponentType, Dispatch, SetStateAction } from 'react'
import type { LucideIcon } from 'lucide-react'

interface NavItem {
  label: string
  icon: LucideIcon
  count?: string
}

interface NavGroup {
  label: string
  items: NavItem[]
}

interface SidebarProps {
  navGroups: NavGroup[]
  activeNav: string
  setActiveNav: Dispatch<SetStateAction<string>>
  setActiveFilter: Dispatch<SetStateAction<string>>
  mobileSidebar: boolean
  setMobileSidebar: Dispatch<SetStateAction<boolean>>
  AvatarComponent: ComponentType<{ initials: string; index: number }>
  onSignOut: () => void
  userName?: string
  userEmail?: string
  userRole?: string
}

export default function Sidebar({
  navGroups,
  activeNav,
  setActiveNav,
  setActiveFilter,
  mobileSidebar,
  setMobileSidebar,
  AvatarComponent,
  onSignOut,
  userName = 'Workspace admin',
  userEmail = 'Administrator',
  userRole = 'Administrator',
}: SidebarProps) {
  const userInitials = userName.split(/\s+/).map(part => part[0]).slice(0, 2).join('').toUpperCase()
  const handleNavigation = (label: string) => {
    setActiveNav(label)
    setActiveFilter('All tasks')
    setMobileSidebar(false)
  }

  return (
    <>
      {mobileSidebar && (
        <button
          className="mobile-backdrop"
          aria-label="Close navigation"
          onClick={() => setMobileSidebar(false)}
        />
      )}

      <aside className={`sidebar ${mobileSidebar ? 'sidebar-open' : ''}`}>
        <div className="brand">
          <div className="brand-mark"><Command size={21} strokeWidth={2.5} /></div>
          <span>workboard<span className="brand-period">.</span></span>
          <button className="icon-button sidebar-close" onClick={() => setMobileSidebar(false)} aria-label="Close menu">
            <X size={18} />
          </button>
        </div>

        <button className="workspace-switcher">
          <div className="workspace-avatar">A</div>
          <div className="workspace-info">
            <strong>Office Workspace</strong>
            <span>Task management</span>
          </div>
          <ChevronDown size={16} className="muted-icon" />
        </button>

        <button className="quick-search" onClick={() => document.getElementById('task-search')?.focus()}>
          <Search size={16} />
          <span>Search anything...</span>
          <kbd>⌘ K</kbd>
        </button>

        <nav className="sidebar-nav" aria-label="Main navigation">
          {navGroups.map((group) => (
            <div className="nav-group" key={group.label}>
              <p className="nav-label">{group.label}</p>
              {group.items.map(({ label, icon: Icon, count }) => (
                <button
                  key={label}
                  className={`nav-item ${activeNav === label ? 'nav-item-active' : ''}`}
                  onClick={() => handleNavigation(label)}
                >
                  <Icon size={18} strokeWidth={1.8} />
                  <span>{label}</span>
                  {count && <span className={`nav-count ${label === 'Approvals' ? 'nav-count-alert' : ''}`}>{count}</span>}
                </button>
              ))}
            </div>
          ))}
        </nav>

        <div className="sidebar-bottom">
          <div className="upgrade-card">
            <div className="upgrade-icon"><Zap size={16} /></div>
            <strong>Make work flow.</strong>
            <p>Everything your team needs, all in one place.</p>
            <button onClick={() => handleNavigation('Projects')}>Explore workspace <ArrowRight size={14} /></button>
          </div>

          <button className={`nav-item ${activeNav === 'Settings' ? 'nav-item-active' : ''}`} onClick={() => handleNavigation('Settings')}>
            <Settings2 size={18} strokeWidth={1.8} />
            <span>Settings</span>
          </button>
          <button className={`nav-item ${activeNav === 'Help & support' ? 'nav-item-active' : ''}`} onClick={() => handleNavigation('Help & support')}>
            <CircleHelp size={18} strokeWidth={1.8} />
            <span>Help &amp; support</span>
          </button>

          <div className="sidebar-profile">
            <AvatarComponent initials={userInitials || 'U'} index={0} />
            <div className="profile-copy">
              <strong>{userName}</strong>
              <span title={userEmail}>{userRole}</span>
            </div>
            <button className="icon-button" aria-label="Profile options"><MoreHorizontal size={18} /></button>
          </div>
          <button className="nav-item sidebar-signout" onClick={onSignOut}>
            <LogOut size={18} strokeWidth={1.8} />
            <span>Sign out</span>
          </button>
        </div>
      </aside>
    </>
  )
}
