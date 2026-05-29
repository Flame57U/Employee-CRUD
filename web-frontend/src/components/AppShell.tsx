import { useState } from 'react'
import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import {
  Activity,
  LayoutDashboard,
  LayoutTemplate,
  Boxes,
  Dna,
  Cpu,
  LineChart,
  Settings,
  LogOut,
  ChevronDown,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useAuthStore } from '@/store/authStore'
import { apiRequest } from '@/lib/api'
import { AppBackground } from './AppBackground'

type Feature = 'dashboard' | 'strategies' | 'instances' | 'evolution' | 'agents' | 'backtesting' | 'settings'

interface NavItem {
  to: string
  label: string
  icon: LucideIcon
  feature: Feature
  end?: boolean
  placement?: 'main' | 'footer'
}

const NAV_ITEMS: NavItem[] = [
  { to: '/', label: '总览', icon: LayoutDashboard, feature: 'dashboard', end: true },
  { to: '/templates', label: '策略模板', icon: LayoutTemplate, feature: 'strategies' },
  { to: '/instances', label: '实例管理', icon: Boxes, feature: 'instances' },
  { to: '/evolution', label: '进化实验室', icon: Dna, feature: 'evolution' },
  { to: '/agents', label: 'Agent', icon: Cpu, feature: 'agents' },
  { to: '/backtesting', label: '回测', icon: LineChart, feature: 'backtesting' },
  { to: '/settings', label: '账户设置', icon: Settings, feature: 'settings', placement: 'footer' },
]

// Lab/dev-only features are gated by JWT role; everything else is universal.
function useHasFeature() {
  const role = useAuthStore((s) => s.user?.role ?? 'user')
  return (f: Feature) => {
    if (f === 'evolution' || f === 'backtesting') return role === 'lab' || role === 'dev'
    return true
  }
}

function NavRow({ item }: { item: NavItem }) {
  const Icon = item.icon
  return (
    <NavLink
      to={item.to}
      end={item.end}
      className={({ isActive }) =>
        [
          'flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors duration-150',
          isActive
            ? 'border border-[#2dd4bf]/20 bg-[#2dd4bf]/[0.06] text-[#2dd4bf]'
            : 'border border-transparent text-slate-500 hover:text-slate-300',
        ].join(' ')
      }
    >
      <Icon className="h-4 w-4 shrink-0" />
      <span className="hidden lg:inline">{item.label}</span>
    </NavLink>
  )
}

function AgentStatusDot() {
  const { data } = useQuery({
    queryKey: ['agent-status'],
    queryFn: () => apiRequest<{ connected: boolean }>('/agents/status'),
    refetchInterval: 30_000,
  })
  const online = data?.connected ?? false
  return (
    <div className="flex items-center gap-2 text-xs text-slate-400">
      <span
        className={`h-2 w-2 rounded-full ${online ? 'bg-[#34d399]' : 'bg-slate-600'}`}
        style={online ? { boxShadow: '0 0 8px #34d399' } : undefined}
      />
      <span className="hidden sm:inline">{online ? 'Agent 在线' : 'Agent 离线'}</span>
    </div>
  )
}

function UserMenu() {
  const email = useAuthStore((s) => s.user?.email ?? '')
  const logout = useAuthStore((s) => s.logout)
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)

  const onLogout = () => {
    logout()
    navigate('/login', { replace: true })
  }

  return (
    <div className="relative">
      <button
        onClick={() => setOpen((v) => !v)}
        className="flex items-center gap-2 rounded-lg border border-white/[0.06] bg-white/[0.02] px-3 py-1.5 text-sm text-slate-300 transition-colors hover:border-white/10"
      >
        <span className="max-w-[12rem] truncate font-mono text-xs">{email}</span>
        <ChevronDown className="h-3.5 w-3.5 text-slate-500" />
      </button>
      {open && (
        <>
          <div className="fixed inset-0 z-10" onClick={() => setOpen(false)} />
          <div className="absolute right-0 z-20 mt-2 w-40 rounded-lg border border-white/10 bg-slate-900/90 p-1 backdrop-blur-xl">
            <button
              onClick={onLogout}
              className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm text-slate-300 transition-colors hover:bg-white/[0.04] hover:text-[#f87171]"
            >
              <LogOut className="h-4 w-4" />
              退出登录
            </button>
          </div>
        </>
      )}
    </div>
  )
}

export function AppShell() {
  const hasFeature = useHasFeature()
  const mainItems = NAV_ITEMS.filter((i) => i.placement !== 'footer' && hasFeature(i.feature))
  const footerItems = NAV_ITEMS.filter((i) => i.placement === 'footer' && hasFeature(i.feature))

  return (
    <div className="relative flex h-screen overflow-hidden">
      <AppBackground />

      {/* Sidebar */}
      <aside className="flex h-screen w-16 shrink-0 flex-col border-r-2 border-[#0a0f1c] bg-[#020617]/40 backdrop-blur-xl lg:w-64">
        <div className="flex h-16 items-center gap-3 px-4">
          <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-[#ff8c6b]/10 shadow-[0_0_20px_rgba(255,140,107,0.35)]">
            <Activity className="h-5 w-5 text-[#ff8c6b]" />
          </div>
          <span className="hidden text-sm font-semibold tracking-wider text-slate-200 lg:inline">
            QuantSaaS
          </span>
        </div>

        <nav className="flex flex-1 flex-col gap-1 px-2 py-4">
          {mainItems.map((item) => (
            <NavRow key={item.to} item={item} />
          ))}
        </nav>

        <div className="flex flex-col gap-1 border-t border-white/[0.04] px-2 py-4">
          {footerItems.map((item) => (
            <NavRow key={item.to} item={item} />
          ))}
        </div>
      </aside>

      {/* Main column */}
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-16 shrink-0 items-center justify-end gap-4 border-b border-white/[0.04] px-4 lg:px-6">
          <AgentStatusDot />
          <UserMenu />
        </header>
        <main className="custom-scrollbar min-h-0 flex-1 overflow-y-auto p-4 lg:p-6">
          <div className="mx-auto max-w-[1800px]">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  )
}
