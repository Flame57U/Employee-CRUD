import { useEffect, useState, type ReactNode } from 'react'
import { configureApi } from '@/lib/api'
import { getAuthToken, useAuthStore } from '@/store/authStore'

// Wires the api layer to the auth store exactly once, before children render.
// There is no /auth/me endpoint, so token validity is verified lazily: the
// first protected request that returns 401 triggers logout via the api hook.
export function AuthProvider({ children }: { children: ReactNode }) {
  const logout = useAuthStore((s) => s.logout)
  const [ready, setReady] = useState(false)

  useEffect(() => {
    configureApi({ getToken: getAuthToken, onUnauthorized: logout })
    setReady(true)
  }, [logout])

  if (!ready) return null
  return <>{children}</>
}
