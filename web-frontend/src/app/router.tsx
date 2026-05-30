import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { useAuthStore } from '@/store/authStore'
import { AppShell } from '@/components/AppShell'
import { LoginPage } from '@/features/auth/LoginPage'
import { RegisterPage } from '@/features/auth/RegisterPage'
import { DashboardPage } from '@/features/dashboard/DashboardPage'
import { InstancesPage } from '@/features/instances/InstancesPage'
import { MarketPage } from '@/features/market/MarketPage'
import { TemplatesPage } from '@/features/templates/TemplatesPage'
import { AgentsPage } from '@/features/agents/AgentsPage'
import { EvolutionPage } from '@/features/evolution/EvolutionPage'
import { BacktestPage } from '@/features/backtesting/BacktestPage'
import { SettingsPage } from '@/features/settings/SettingsPage'

// Redirects to /login when unauthenticated, preserving the attempted path.
function AuthGate({ children }: { children: React.ReactNode }) {
  const token = useAuthStore((s) => s.token)
  const location = useLocation()
  if (!token) return <Navigate to="/login" replace state={{ from: location }} />
  return <>{children}</>
}

// Bounces an already-authenticated user away from the auth pages.
function PublicOnly({ children }: { children: React.ReactNode }) {
  const token = useAuthStore((s) => s.token)
  if (token) return <Navigate to="/" replace />
  return <>{children}</>
}

export function AppRouter() {
  return (
    <Routes>
      <Route path="/login" element={<PublicOnly><LoginPage /></PublicOnly>} />
      <Route path="/register" element={<PublicOnly><RegisterPage /></PublicOnly>} />
      <Route
        path="/"
        element={
          <AuthGate>
            <AppShell />
          </AuthGate>
        }
      >
        <Route index element={<DashboardPage />} />
        <Route path="market" element={<MarketPage />} />
        <Route path="templates" element={<TemplatesPage />} />
        <Route path="instances" element={<InstancesPage />} />
        <Route path="evolution" element={<EvolutionPage />} />
        <Route path="agents" element={<AgentsPage />} />
        <Route path="backtesting" element={<BacktestPage />} />
        <Route path="settings" element={<SettingsPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
