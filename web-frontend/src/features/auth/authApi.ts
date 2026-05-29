import { apiRequest } from '@/lib/api'
import type { AuthUser } from '@/store/authStore'

interface AuthResponse {
  user_id: number
  email: string
  plan: string
  role?: string // present on login; register omits it
  token: string
}

// Mirrors backend planToRole: only the elite plan gets lab access.
function planToRole(plan: string): string {
  return plan === 'elite' ? 'lab' : 'user'
}

function toAuth(res: AuthResponse): { token: string; user: AuthUser } {
  return {
    token: res.token,
    user: {
      user_id: res.user_id,
      email: res.email,
      plan: res.plan,
      role: res.role ?? planToRole(res.plan),
    },
  }
}

export async function login(email: string, password: string) {
  const res = await apiRequest<AuthResponse>('/auth/login', {
    method: 'POST',
    body: { email, password },
  })
  return toAuth(res)
}

export async function register(email: string, password: string) {
  const res = await apiRequest<AuthResponse>('/auth/register', {
    method: 'POST',
    body: { email, password },
  })
  return toAuth(res)
}
