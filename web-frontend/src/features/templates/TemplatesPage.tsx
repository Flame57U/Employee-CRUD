import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  LayoutTemplate,
  Loader2,
  Plus,
  X,
  Clock,
  CandlestickChart,
  Coins,
  Cpu,
  ShieldCheck,
} from 'lucide-react'
import { formatCNY } from '@/lib/format'
import { ApiRequestError } from '@/lib/api'
import { createInstance } from '../dashboard/dashboardApi'
import {
  getTemplates,
  assetClassLabel,
  type StrategyTemplate,
  type TemplatePolicy,
} from './templatesApi'

const cardClass = 'rounded-xl border border-white/[0.04] bg-white/[0.02] p-4 backdrop-blur-sm'

export function TemplatesPage() {
  const [selected, setSelected] = useState<StrategyTemplate | null>(null)

  const templatesQ = useQuery({
    queryKey: ['strategies'],
    queryFn: getTemplates,
  })

  const templates = templatesQ.data ?? []

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-lg font-semibold text-slate-200">策略模板</h1>
          <p className="text-sm text-slate-500">
            预置的只读策略蓝图，用于创建运行实例。模板由平台统一维护，不可编辑。
          </p>
        </div>
        {templates.length > 0 && (
          <span className="rounded-full border border-white/[0.06] bg-white/[0.02] px-3 py-1 text-xs text-slate-400">
            共 {templates.length} 个模板
          </span>
        )}
      </header>

      {templatesQ.isLoading ? (
        <div className="flex min-h-[40vh] items-center justify-center text-slate-500">
          <Loader2 className="mr-2 h-5 w-5 animate-spin" /> 加载策略模板…
        </div>
      ) : templatesQ.isError ? (
        <div className={`${cardClass} text-sm text-[#f87171]`}>
          加载失败：{(templatesQ.error as Error).message}
        </div>
      ) : templates.length === 0 ? (
        <EmptyTemplates />
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {templates.map((t) => (
            <TemplateCard key={t.ID} template={t} onOpen={() => setSelected(t)} />
          ))}
        </div>
      )}

      {selected && (
        <TemplateDetailPanel template={selected} onClose={() => setSelected(null)} />
      )}
    </div>
  )
}

function Badge({ children, tone = 'slate' }: { children: React.ReactNode; tone?: 'slate' | 'teal' | 'amber' }) {
  const tones = {
    slate: 'border-white/[0.08] text-slate-400',
    teal: 'border-[#2dd4bf]/30 text-[#2dd4bf]',
    amber: 'border-[#fbbf24]/30 text-[#fbbf24]',
  }
  return (
    <span className={`rounded-md border px-2 py-0.5 text-xs ${tones[tone]}`}>{children}</span>
  )
}

function TemplateCard({
  template,
  onOpen,
}: {
  template: StrategyTemplate
  onOpen: () => void
}) {
  const mf = template.Manifest
  const policy = mf?.spawn_point?.Policy
  const asset = assetClassLabel(policy?.AssetClass)
  const symbol = policy?.Symbol ?? mf?.symbol

  return (
    <button
      onClick={onOpen}
      className={`${cardClass} flex flex-col gap-3 text-left transition-colors hover:border-[#2dd4bf]/30`}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border border-white/[0.06] bg-[#2dd4bf]/[0.06]">
            <LayoutTemplate className="h-5 w-5 text-[#2dd4bf]" />
          </div>
          <div>
            <p className="font-medium text-slate-200">{template.Name}</p>
            <p className="font-mono text-xs text-slate-500">{template.Version}</p>
          </div>
        </div>
        <Badge tone={template.IsSpot ? 'teal' : 'amber'}>{template.IsSpot ? '现货' : '合约'}</Badge>
      </div>

      {mf?.desc && <p className="line-clamp-2 text-sm text-slate-400">{mf.desc}</p>}

      <div className="flex flex-wrap items-center gap-2">
        {asset && <Badge>{asset}</Badge>}
        {mf?.engine && (
          <span className="flex items-center gap-1 text-xs text-slate-400">
            <Cpu className="h-3 w-3" /> {mf.engine}
          </span>
        )}
        {symbol && (
          <span className="flex items-center gap-1 text-xs text-slate-400">
            <CandlestickChart className="h-3 w-3" /> {symbol}
          </span>
        )}
        {mf?.interval && (
          <span className="flex items-center gap-1 text-xs text-slate-400">
            <Clock className="h-3 w-3" /> {mf.interval}
          </span>
        )}
      </div>
    </button>
  )
}

function TemplateDetailPanel({
  template,
  onClose,
}: {
  template: StrategyTemplate
  onClose: () => void
}) {
  const qc = useQueryClient()
  const navigate = useNavigate()

  const create = useMutation({
    mutationFn: () => createInstance(template.ID),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['instances'] })
      qc.invalidateQueries({ queryKey: ['dashboard'] })
      navigate('/instances')
    },
  })

  const mf = template.Manifest
  const policy = mf?.spawn_point?.Policy
  const asset = assetClassLabel(policy?.AssetClass)
  const symbol = policy?.Symbol ?? mf?.symbol

  return (
    <>
      <div className="fixed inset-0 z-30 bg-black/50 backdrop-blur-sm" onClick={onClose} />
      <aside className="custom-scrollbar fixed right-0 top-0 z-40 flex h-screen w-full max-w-md flex-col overflow-y-auto border-l border-white/[0.06] bg-[#020617]/95 p-6 backdrop-blur-xl">
        <div className="flex items-start justify-between gap-3">
          <div className="flex items-center gap-3">
            <div className="flex h-11 w-11 items-center justify-center rounded-lg border border-white/[0.06] bg-[#2dd4bf]/[0.06]">
              <LayoutTemplate className="h-5 w-5 text-[#2dd4bf]" />
            </div>
            <div>
              <h2 className="font-semibold text-slate-200">{template.Name}</h2>
              <p className="font-mono text-xs text-slate-500">{template.Version}</p>
            </div>
          </div>
          <button onClick={onClose} className="text-slate-500 hover:text-slate-300">
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="mt-4 flex flex-wrap gap-2">
          <Badge tone={template.IsSpot ? 'teal' : 'amber'}>
            {template.IsSpot ? '现货' : '合约'}
          </Badge>
          {asset && <Badge>{asset}</Badge>}
          {mf?.engine && <Badge>{mf.engine}</Badge>}
        </div>

        {mf?.desc && <p className="mt-4 text-sm leading-relaxed text-slate-400">{mf.desc}</p>}

        {(symbol || mf?.interval) && (
          <section className="mt-6 flex flex-col gap-3">
            <h3 className="text-xs uppercase tracking-wider text-slate-500">标的与周期</h3>
            <dl className="grid grid-cols-2 gap-3">
              <DetailItem icon={CandlestickChart} label="标的" value={symbol ?? '—'} />
              <DetailItem icon={Clock} label="K线周期" value={mf?.interval ?? '—'} />
            </dl>
          </section>
        )}

        {policy && <PolicySection policy={policy} />}

        <div className="mt-auto pt-8">
          {create.isError && (
            <p className="mb-3 text-sm text-[#f87171]">{createError(create.error)}</p>
          )}
          <button
            onClick={() => create.mutate()}
            disabled={create.isPending}
            className="flex w-full items-center justify-center gap-2 rounded-lg bg-[#2dd4bf] px-4 py-2.5 text-sm font-semibold text-slate-900 transition-opacity hover:opacity-90 disabled:opacity-50"
          >
            {create.isPending ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Plus className="h-4 w-4" />
            )}
            用此模板创建实例
          </button>
        </div>
      </aside>
    </>
  )
}

function PolicySection({ policy }: { policy: TemplatePolicy }) {
  const rows: Array<{ label: string; value: string }> = []
  if (policy.TotalCapitalCNY != null)
    rows.push({ label: '总资金', value: formatCNY(policy.TotalCapitalCNY) })
  if (policy.MonthlyInjectCNY != null)
    rows.push({ label: '每月注入', value: formatCNY(policy.MonthlyInjectCNY) })
  if (policy.MacroMinOrderCNY != null)
    rows.push({ label: '最小下单额', value: formatCNY(policy.MacroMinOrderCNY) })
  if (policy.DeadlineRatio != null)
    rows.push({ label: '死钱期限比', value: String(policy.DeadlineRatio) })
  if (policy.MaxLotsPerTick != null)
    rows.push({ label: '每 Tick 最大批次', value: String(policy.MaxLotsPerTick) })

  if (rows.length === 0) return null

  return (
    <section className="mt-6 flex flex-col gap-3">
      <h3 className="flex items-center gap-1.5 text-xs uppercase tracking-wider text-slate-500">
        <ShieldCheck className="h-3.5 w-3.5" /> 资金策略
      </h3>
      <dl className="flex flex-col divide-y divide-white/[0.04] rounded-lg border border-white/[0.04]">
        {rows.map((r) => (
          <div key={r.label} className="flex items-center justify-between px-3 py-2 text-sm">
            <dt className="text-slate-500">{r.label}</dt>
            <dd className="font-mono text-slate-300">{r.value}</dd>
          </div>
        ))}
      </dl>
    </section>
  )
}

function DetailItem({
  icon: Icon,
  label,
  value,
}: {
  icon: typeof Coins
  label: string
  value: string
}) {
  return (
    <div className="rounded-lg border border-white/[0.04] bg-white/[0.02] p-3">
      <p className="flex items-center gap-1.5 text-xs text-slate-500">
        <Icon className="h-3.5 w-3.5" /> {label}
      </p>
      <p className="mt-1 font-mono text-sm text-slate-200">{value}</p>
    </div>
  )
}

// Surface the backend's quota message specifically; fall back to the raw message.
function createError(err: unknown): string {
  if (err instanceof ApiRequestError && err.status === 403) {
    return '已达到当前套餐的实例数量上限，请升级套餐或删除已有实例。'
  }
  return `创建失败：${(err as Error).message}`
}

function EmptyTemplates() {
  return (
    <div className={`${cardClass} flex flex-col items-center justify-center gap-4 py-16 text-center`}>
      <div className="flex h-14 w-14 items-center justify-center rounded-2xl border border-white/[0.06] bg-white/[0.02]">
        <LayoutTemplate className="h-6 w-6 text-[#2dd4bf]" />
      </div>
      <div>
        <p className="text-base text-slate-200">暂无策略模板</p>
        <p className="mt-1 text-sm text-slate-500">模板由平台统一预置，请稍后再来查看。</p>
      </div>
    </div>
  )
}
