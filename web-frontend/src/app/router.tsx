import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { useAuthStore } from '@/store/authStore'
import { AppShell } from '@/components/AppShell'
import { LoginPage } from '@/features/auth/LoginPage'
import { RegisterPage } from '@/features/auth/RegisterPage'
import { DashboardPage } from '@/features/dashboard/DashboardPage'
import { ComingSoonPage } from '@/components/ComingSoonPage'

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
        <Route path="templates" element={<ComingSoonPage title="策略模板" />} />
        <Route path="instances" element={<ComingSoonPage title="实例管理" />} />
        <Route path="evolution" element={<ComingSoonPage title="进化实验室" />} />
        <Route path="agents" element={<ComingSoonPage title="Agent 管理" />} />
        <Route path="backtesting" element={<ComingSoonPage title="回测" />} />
        <Route path="settings" element={<ComingSoonPage title="账户设置" />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
