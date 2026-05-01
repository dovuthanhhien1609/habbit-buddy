import React from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { LayoutDashboard, BarChart3, LogOut, Leaf } from 'lucide-react'
import { useStore } from '../store/useStore'
import { NotificationBell } from './NotificationBell'

interface Props {
  children: React.ReactNode
}

export function Layout({ children }: Props) {
  const location = useLocation()
  const navigate = useNavigate()
  const user = useStore((s) => s.user)
  const logout = useStore((s) => s.logout)

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  const navItems = [
    { to: '/', label: 'Dashboard', icon: LayoutDashboard },
    { to: '/analytics', label: 'Analytics', icon: BarChart3 },
  ]

  return (
    <div className="min-h-screen flex flex-col">
      {/* Top nav */}
      <header className="sticky top-0 z-30 bg-white border-b border-slate-100 shadow-sm">
        <div className="mx-auto max-w-2xl px-4 h-14 flex items-center justify-between">
          {/* Logo */}
          <Link to="/" className="flex items-center gap-2 font-bold text-brand-600">
            <Leaf size={20} />
            <span>habit-buddy</span>
          </Link>

          {/* Nav links */}
          <nav className="hidden sm:flex items-center gap-1">
            {navItems.map(({ to, label, icon: Icon }) => (
              <Link
                key={to}
                to={to}
                className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors
                  ${location.pathname === to
                    ? 'bg-brand-50 text-brand-600'
                    : 'text-slate-500 hover:text-slate-700 hover:bg-slate-100'
                  }`}
              >
                <Icon size={15} />
                {label}
              </Link>
            ))}
          </nav>

          {/* User */}
          <div className="flex items-center gap-3">
            <NotificationBell />
            <div className="hidden sm:flex items-center gap-2">
              <div className="h-7 w-7 rounded-full bg-brand-500 flex items-center justify-center text-white text-xs font-bold">
                {user?.username?.[0]?.toUpperCase() ?? 'U'}
              </div>
              <span className="text-sm text-slate-600">{user?.username}</span>
            </div>
            <button
              onClick={handleLogout}
              className="text-slate-400 hover:text-slate-600 transition-colors"
              title="Log out"
            >
              <LogOut size={16} />
            </button>
          </div>
        </div>
      </header>

      {/* Mobile nav */}
      <div className="sm:hidden fixed bottom-0 left-0 right-0 z-30 bg-white border-t border-slate-100 flex">
        {navItems.map(({ to, label, icon: Icon }) => (
          <Link
            key={to}
            to={to}
            className={`flex-1 flex flex-col items-center gap-1 py-3 text-xs font-medium transition-colors
              ${location.pathname === to ? 'text-brand-600' : 'text-slate-400'}`}
          >
            <Icon size={18} />
            {label}
          </Link>
        ))}
      </div>

      {/* Content */}
      <main className="flex-1 mx-auto w-full max-w-2xl px-4 py-6 pb-20 sm:pb-6">
        {children}
      </main>
    </div>
  )
}
