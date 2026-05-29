import { useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Loader2 } from 'lucide-react'
import { AuthScaffold } from '@/components/AuthScaffold'
import { useAuthStore } from '@/store/authStore'
import { ApiRequestError } from '@/lib/api'
import { register } from './authApi'
import { fieldClass, primaryButtonClass } from './styles'

export function RegisterPage() {
  const setAuth = useAuthStore((s) => s.login)
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    if (password !== confirm) {
      setError('两次输入的密码不一致')
      return
    }
    setBusy(true)
    try {
      const { token, user } = await register(email.trim(), password)
      setAuth(token, user)
      navigate('/', { replace: true })
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : '注册失败，请重试')
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthScaffold slogan="创建账户，开启量化之旅">
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
            autoComplete="new-password"
            required
            minLength={8}
            disabled={busy}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className={fieldClass}
            placeholder="至少 8 位"
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <label className="text-xs uppercase tracking-wider text-slate-500">确认密码</label>
          <input
            type="password"
            autoComplete="new-password"
            required
            minLength={8}
            disabled={busy}
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            className={fieldClass}
            placeholder="再次输入密码"
          />
        </div>

        <button type="submit" disabled={busy} className={primaryButtonClass}>
          {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
          {busy ? '注册中' : '注册'}
        </button>

        {error && <p className="text-center text-sm text-[#f87171]">{error}</p>}

        <p className="text-center text-sm text-slate-500">
          已有账号？{' '}
          <Link to="/login" className="text-[#2dd4bf] hover:underline">
            登录
          </Link>
        </p>
      </form>
    </AuthScaffold>
  )
}
