import { useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Loader2 } from 'lucide-react'
import { AuthScaffold } from '@/components/AuthScaffold'
import { useAuthStore } from '@/store/authStore'
import { ApiRequestError } from '@/lib/api'
import { login } from './authApi'
import { fieldClass, primaryButtonClass } from './styles'

export function LoginPage() {
  const setAuth = useAuthStore((s) => s.login)
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    setBusy(true)
    try {
      const { token, user } = await login(email.trim(), password)
      setAuth(token, user)
      navigate('/', { replace: true })
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : '登录失败，请重试')
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthScaffold slogan="纯粹策略 · 进化计算 · 量化终端">
      <form onSubmit={onSubmit} className="flex flex-col gap-4">
        <div className="flex flex-col gap-1.5">
          <label className="text-xs uppercase tracking-wider text-slate-500">邮箱</label>
          <input
            type="email"
            autoComplete="email"
            required
            disabled={busy}
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className={fieldClass}
            placeholder="you@example.com"
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <label className="text-xs uppercase tracking-wider text-slate-500">密码</label>
          <input
            type="password"
            autoComplete="current-password"
            required
            minLength={8}
            disabled={busy}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className={fieldClass}
            placeholder="••••••••"
          />
        </div>

        <button type="submit" disabled={busy} className={primaryButtonClass}>
          {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
          {busy ? '登录中' : '登录'}
        </button>

        {error && <p className="text-center text-sm text-[#f87171]">{error}</p>}

        <p className="text-center text-sm text-slate-500">
          没有账号？{' '}
          <Link to="/register" className="text-[#2dd4bf] hover:underline">
            注册
          </Link>
        </p>
      </form>
    </AuthScaffold>
  )
}
