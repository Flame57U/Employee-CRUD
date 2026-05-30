import { useNavigate } from 'react-router-dom'
import { User, ShieldCheck, Boxes, LogOut, Mail, BadgeCheck } from 'lucide-react'
import { useAuthStore } from '@/store/authStore'

const cardClass = 'rounded-xl border border-white/[0.04] bg-white/[0.02] p-5 backdrop-blur-sm'

// Mirrors planQuota in handler_instance.go.
const PLAN_QUOTA: Record<string, number> = { free: 1, pro: 5, elite: 50 }

const PLAN_LABELS: Record<string, string> = {
  free: '免费版',
  pro: '专业版',
  elite: '旗舰版',
}

export function SettingsPage() {
  const user = useAuthStore((s) => s.user)
  const logout = useAuthStore((s) => s.logout)
  const navigate = useNavigate()

  const plan = user?.plan ?? 'free'
  const quota = PLAN_QUOTA[plan] ?? PLAN_QUOTA.free

  const onLogout = () => {
    logout()
    navigate('/login', { replace: true })
  }

  return (
    <div className="flex max-w-3xl flex-col gap-6">
      <header>
        <h1 className="text-lg font-semibold text-slate-200">账户设置</h1>
        <p className="text-sm text-slate-500">查看账户信息与当前套餐。</p>
      </header>

      {/* Account info */}
      <div className={cardClass}>
        <h2 className="flex items-center gap-2 text-sm font-semibold text-slate-200">
          <User className="h-4 w-4 text-[#2dd4bf]" /> 账户信息
        </h2>
        <dl className="mt-4 flex flex-col divide-y divide-white/[0.04]">
          <Row icon={Mail} label="邮箱" value={user?.email ?? '—'} mono />
          <Row icon={BadgeCheck} label="账户 ID" value={user?.user_id != null ? String(user.user_id) : '—'} mono />
          <Row icon={ShieldCheck} label="角色" value={user?.role ?? 'user'} mono />
        </dl>
      </div>

      {/* Subscription */}
      <div className={cardClass}>
        <h2 className="flex items-center gap-2 text-sm font-semibold text-slate-200">
          <Boxes className="h-4 w-4 text-[#2dd4bf]" /> 订阅套餐
        </h2>
        <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <span className="rounded-lg border border-[#2dd4bf]/30 bg-[#2dd4bf]/[0.06] px-3 py-1.5 text-sm font-semibold text-[#2dd4bf]">
              {PLAN_LABELS[plan] ?? plan}
            </span>
            <span className="text-sm text-slate-400">
              最多可创建 <span className="font-mono text-slate-200">{quota}</span> 个策略实例
            </span>
          </div>
        </div>
        <p className="mt-4 text-xs text-slate-500">
          套餐变更由平台统一管理,如需升级请联系管理员。旗舰版(elite)账户额外开放进化实验室与回测功能。
        </p>
      </div>

      {/* Session */}
      <div className={cardClass}>
        <h2 className="text-sm font-semibold text-slate-200">登录会话</h2>
        <p className="mt-1 text-sm text-slate-500">退出后需重新登录,角色与套餐变更也会在重新登录后生效。</p>
        <button
          onClick={onLogout}
          className="mt-4 flex items-center gap-2 rounded-lg border border-white/[0.08] px-4 py-2 text-sm text-slate-300 transition-colors hover:border-[#f87171]/40 hover:text-[#f87171]"
        >
          <LogOut className="h-4 w-4" /> 退出登录
        </button>
      </div>
    </div>
  )
}

function Row({
  icon: Icon,
  label,
  value,
  mono,
}: {
  icon: typeof User
  label: string
  value: string
  mono?: boolean
}) {
  return (
    <div className="flex items-center justify-between py-3">
      <dt className="flex items-center gap-2 text-sm text-slate-500">
        <Icon className="h-4 w-4" /> {label}
      </dt>
      <dd className={`text-sm text-slate-200 ${mono ? 'font-mono' : ''}`}>{value}</dd>
    </div>
  )
}
