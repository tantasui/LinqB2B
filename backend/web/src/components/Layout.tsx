import React, { useState, useEffect } from 'react'
import { NavLink, useNavigate, useLocation } from 'react-router-dom'
import { useStore } from '../store/useStore'
import { 
  LayoutDashboard, 
  Wallet, 
  ArrowLeftRight, 
  Link as LinkIcon, 
  Settings, 
  LogOut, 
  Bell, 
  Menu, 
  X,
  User
} from 'lucide-react'
import { Toaster } from 'sonner'

interface LayoutProps {
  children: React.ReactNode
}

const Layout: React.FC<LayoutProps> = ({ children }) => {
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false)
  const { merchant, logout } = useStore()
  const navigate = useNavigate()
  const location = useLocation()

  // Session timeout logic (15 mins)
  useEffect(() => {
    let timeout: any
    const resetTimeout = () => {
      if (timeout) clearTimeout(timeout)
      timeout = setTimeout(() => {
        logout()
        navigate('/login')
      }, 15 * 60 * 1000)
    }

    window.addEventListener('mousemove', resetTimeout)
    window.addEventListener('keypress', resetTimeout)
    resetTimeout()

    return () => {
      window.removeEventListener('mousemove', resetTimeout)
      window.removeEventListener('keypress', resetTimeout)
      if (timeout) clearTimeout(timeout)
    }
  }, [logout, navigate])

  const navItems = [
    { name: 'Home', path: '/', icon: 'dashboard', lucide: LayoutDashboard },
    { name: 'Deposits', path: '/deposits', icon: 'account_balance_wallet', lucide: Wallet },
    { name: 'Settlements', path: '/settlements', icon: 'payments', lucide: ArrowLeftRight },
    { name: 'Payment Links', path: '/payment-links', icon: 'link', lucide: LinkIcon },
    { name: 'Settings', path: '/settings', icon: 'settings', lucide: Settings },
  ]

  const activeClass = "bg-violet-50 dark:bg-violet-900/20 text-violet-600 dark:text-violet-400 border-l-4 border-violet-600 font-bold"
  const inactiveClass = "text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900/50 hover:text-violet-600"

  return (
    <div className="bg-background text-on-background min-h-screen flex flex-col md:flex-row overflow-x-hidden font-sans">
      <Toaster position="top-right" />
      
      {/* Mobile Header */}
      <header className="md:hidden flex justify-between items-center w-full px-6 py-3 bg-slate-50/80 dark:bg-slate-950/80 backdrop-blur-xl sticky top-0 z-50 border-b border-slate-200/50 dark:border-slate-800/50 shadow-sm">
        <div className="flex items-center gap-4">
          <button onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)} className="text-slate-600 dark:text-slate-400">
            {isMobileMenuOpen ? <X size={24} /> : <Menu size={24} />}
          </button>
          <span className="text-xl font-black tracking-tighter text-violet-600 dark:text-violet-400">Vibrant Merchant</span>
        </div>
        <div className="flex items-center gap-4">
          <Bell className="text-violet-600 dark:text-violet-400 cursor-pointer" size={24} />
          <div className="w-8 h-8 rounded-full border-2 border-violet-200 bg-violet-100 flex items-center justify-center text-violet-600 font-bold">
            {merchant?.name?.charAt(0) || <User size={16} />}
          </div>
        </div>
      </header>

      {/* Desktop Sidebar & Mobile Drawer */}
      <nav className={`bg-slate-50 dark:bg-slate-950 h-screen w-64 border-r border-slate-200 dark:border-slate-800 fixed left-0 top-0 flex flex-col p-4 gap-2 z-[60] transition-transform duration-300 ease-in-out shadow-2xl md:shadow-none ${isMobileMenuOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0'}`}>
        <div className="flex items-center gap-3 mb-8 px-4 mt-4">
          <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-violet-500 to-primary-container flex items-center justify-center text-white font-bold">L</div>
          <div>
            <div className="text-lg font-bold text-violet-600 dark:text-violet-400 leading-tight">Merchant Hub</div>
            <div className="text-xs text-slate-500">Institutional Grade</div>
          </div>
        </div>
        
        <div className="flex-1 space-y-2">
          {navItems.map((item) => (
            <NavLink
              key={item.path}
              to={item.path}
              onClick={() => setIsMobileMenuOpen(false)}
              className={({ isActive }) => `flex items-center gap-3 px-4 py-3 rounded-lg transition-all duration-150 ${isActive ? activeClass : inactiveClass}`}
            >
              <span className="material-symbols-outlined">{item.icon}</span>
              {item.name}
            </NavLink>
          ))}
        </div>

        <button 
          onClick={logout}
          className="flex items-center gap-3 text-slate-600 dark:text-slate-400 px-4 py-3 hover:bg-error-container/20 hover:text-error transition-all rounded-lg mt-auto"
        >
          <LogOut size={20} />
          Logout
        </button>
      </nav>

      {/* Main Content Area */}
      <main className="flex-1 md:ml-64 p-6 lg:p-10 flex flex-col gap-6 max-w-7xl mx-auto w-full pb-32 md:pb-10">
        {/* Desktop Header */}
        <div className="hidden md:flex justify-between items-center mb-4">
          <div>
            <h1 className="text-3xl font-bold text-on-background">
              {location.pathname === '/' ? `Hi ${merchant?.name || 'Ben'},` : navItems.find(i => i.path === location.pathname)?.name}
            </h1>
            <p className="text-on-surface-variant">
              {location.pathname === '/' ? "Here's what's happening with your accounts today." : `Manage your ${navItems.find(i => i.path === location.pathname)?.name.toLowerCase()}.`}
            </p>
          </div>
          <div className="flex items-center gap-4">
            <button className="w-10 h-10 rounded-full bg-surface-container flex items-center justify-center text-primary hover:bg-surface-container-high transition-colors relative">
              <Bell size={20} />
              <span className="absolute top-2 right-2 w-2 h-2 bg-error rounded-full"></span>
            </button>
            <div className="flex items-center gap-3 bg-surface-container-lowest border border-outline-variant rounded-full py-1 pr-4 pl-1 shadow-sm">
              <div className="w-8 h-8 rounded-full bg-violet-100 flex items-center justify-center text-violet-600 font-bold">
                {merchant?.name?.charAt(0) || 'B'}
              </div>
              <span className="text-sm font-semibold text-on-surface">{merchant?.name || 'Ben Admin'}</span>
            </div>
            <button 
              onClick={logout}
              className="flex items-center gap-2 text-on-surface-variant hover:text-error transition-colors px-3 py-2 rounded-full hover:bg-error-container/20 group"
            >
              <LogOut size={18} />
              <span className="text-xs font-semibold uppercase tracking-wider">Logout</span>
            </button>
          </div>
        </div>

        {children}
      </main>

      {/* Mobile Bottom Navigation */}
      <nav className="md:hidden fixed bottom-6 left-1/2 -translate-x-1/2 w-[90%] z-50 flex justify-around items-center bg-white/70 dark:bg-slate-900/70 backdrop-blur-2xl text-violet-600 dark:text-violet-400 rounded-2xl border border-slate-200/50 dark:border-slate-700/50 shadow-[0px_10px_30px_rgba(124,58,237,0.15)] py-1">
        {navItems.map((item) => (
          <NavLink
            key={item.path}
            to={item.path}
            className={({ isActive }) => `flex flex-col items-center justify-center py-2 px-4 rounded-xl transition-all duration-300 ${isActive ? 'bg-violet-600 text-white active:scale-90' : 'text-slate-500 dark:text-slate-400 active:scale-90'}`}
          >
            <span className="material-symbols-outlined text-[20px]">{item.icon}</span>
            <span className="text-[10px] font-bold">{item.name.split(' ')[0]}</span>
          </NavLink>
        ))}
      </nav>
    </div>
  )
}

export default Layout
