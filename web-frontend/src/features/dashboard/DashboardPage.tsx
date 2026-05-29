import { useMemo } from 'react'
import { useSearchParams, useNavigate } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Play, Pause, Plus, Boxes, Loader2, TrendingUp } from 'lucide-react'
import { formatCNY, formatNumber, relativeTime } from '@/lib/format'
import {
  getDashboard,
  getInstances,
  getTrades,
  startInstance,
  stopInstance,
  type InstanceRow,
} from './dashboardApi'

const STATUS_META: Record<string, { label: string; dot: string; text: string }> = {
  RUNNING: { label: '运行中', dot: 'bg-[#34d399]', text: 'text-[#34d399]' },
  STOPPED: { label: '已暂停', dot: 'bg-slate-500', text: 'text-slate-400' },
  ERROR: { label: '异常', dot: 'bg-[#f87171]', text: 'text-[#f87171]' },
}

function statusMeta(s: string) {
  return STATUS_META[s] ?? STATUS_META.STOPPED
}

function instanceName(row: InstanceRow | undefined, id: number) {
  if (row?.Template?.Name) return `${row.Template.Name} #${id}`
  return `实例 #${id}`
}

const cardClass =
  'rounded-xl border border-white/[0.04] bg-white/[0.02] p-4 backdrop-blur-sm'

function StatCard({ label, value, accent }: { label: string; value: string; accent?: string }) {
  return (
    <div className={cardClass}>
      <p className="text-xs uppercase tracking-wider text-slate-500">{label}</p>
      <p className={`mt-2 font-mono text-xl ${accent ?? 'text-slate-200'}`}>{value}</p>
    </div>
  )
}

export function DashboardPage() {
  const [params, setParams] = useSearchParams()
  const navigate = useNavigate()
  const qc = useQueryClient()

  const dashboardQ = useQuery({
    queryKey: ['dashboard'],
    queryFn: getDashboard,
    refetchInterval: 60_000,
  })
  const instancesQ = useQuery({
    queryKey: ['instances'],
    queryFn: getInstances,
    refetchInterval: 60_000,
  })

  const instances = dashboardQ.data?.instances ?? []
  const rowsById = useMemo(() => {
    const m = new Map<number, InstanceRow>()
    for (const r of instancesQ.data ?? []) m.set(r.ID, r)
    return m
  }, [instancesQ.data])

  const urlId = Number(params.get('instance')) || null
  const selectedId =
    urlId && instances.some((i) => i.id === urlId) ? urlId : instances[0]?.id ?? null
  const selected = instances.find((i) => i.id === selectedId) ?? null
  const selectedRow = selectedId ? rowsById.get(selectedId) : undefined

  const tradesQ = useQuery({
    queryKey: ['trades', selectedId],
    queryFn: () => getTrades(selectedId!),
    enabled: selectedId != null,
    refetchInterval: 60_000,
  })

  const toggle = useMutation({
    mutationFn: (inst: { id: number; status: string }) =>
      inst.status === 'RUNNING' ? stopInstance(inst.id) : startInstance(inst.id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['dashboard'] })
      qc.invalidateQueries({ queryKey: ['instances'] })
    },
  })

  const selectInstance = (id: number) => {
    const next = new URLSearchParams(params)
    next.set('instance', String(id))
    setParams(next, { replace: true })
  }

  const d = dashboardQ.data

  if (dashboardQ.isLoading) {
    return (
      <div className="flex min-h-[50vh] items-center justify-center text-slate-500">
        <Loader2 className="mr-2 h-5 w-5 animate-spin" /> 加载总览数据…
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-6">
      <header>
        <h1 className="text-lg font-semibold text-slate-200">账户总览</h1>
        <p className="text-sm text-slate-500">您的全部策略实例运行情况</p>
      </header>

      {/* Aggregate strip */}
      <div className="qs-bento-grid">
        <StatCard label="实例总数" value={String(d?.total_instances ?? 0)} />
        <StatCard label="运行中" value={String(d?.running_count ?? 0)} accent="text-[#34d399]" />
        <StatCard label="总资产" value={formatCNY(d?.total_equity)} accent="text-[#2dd4bf]" />
        <StatCard label="可用资金" value={formatCNY(d?.total_cny)} />
      </div>

      {instances.length === 0 ? (
        <EmptyInstances onCreate={() => navigate('/instances')} />
      ) : (
        <div className="flex flex-col gap-6 lg:flex-row">
          {/* Left: instance selector */}
          <div className="flex w-full flex-col gap-3 lg:w-1/4">
            <div className="flex items-center justify-between">
              <span className="text-xs uppercase tracking-wider text-slate-500">策略实例</span>
              <button
                onClick={() => navigate('/instances')}
                className="flex items-center gap-1 text-xs text-[#2dd4bf] hover:underline"
              >
                <Plus className="h-3.5 w-3.5" /> 新建
              </button>
            </div>

            {instances.map((inst) => {
              const meta = statusMeta(inst.status)
              const isSel = inst.id === selectedId
              const busy = toggle.isPending && toggle.variables?.id === inst.id
              return (
                <button
                  key={inst.id}
                  onClick={() => selectInstance(inst.id)}
                  className={[
                    'rounded-xl border bg-white/[0.02] p-3 text-left backdrop-blur-sm transition-colors',
                    isSel
                      ? 'border-l-2 border-l-[#2dd4bf] border-y-white/[0.04] border-r-white/[0.04]'
                      : 'border-white/[0.04] hover:border-white/10',
                  ].join(' ')}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="truncate text-sm text-slate-200">
                      {instanceName(rowsById.get(inst.id), inst.id)}
                    </span>
                    <span className={`flex shrink-0 items-center gap-1.5 text-xs ${meta.text}`}>
                      <span className={`h-1.5 w-1.5 rounded-full ${meta.dot}`} />
                      {meta.label}
                    </span>
                  </div>
                  <div className="mt-2 flex items-center justify-between">
                    <span className="font-mono text-xs text-slate-400">
                      {formatCNY(inst.total_equity)}
                    </span>
                    <span
                      role="button"
                      tabIndex={0}
                      onClick={(e) => {
                        e.stopPropagation()
                        if (!busy) toggle.mutate({ id: inst.id, status: inst.status })
                      }}
                      className="flex items-center gap-1 rounded-md border border-white/[0.06] px-2 py-1 text-xs text-slate-300 transition-colors hover:border-[#2dd4bf]/40 hover:text-[#2dd4bf]"
                    >
                      {busy ? (
                        <Loader2 className="h-3 w-3 animate-spin" />
                      ) : inst.status === 'RUNNING' ? (
                        <Pause className="h-3 w-3" />
                      ) : (
                        <Play className="h-3 w-3" />
                      )}
                      {inst.status === 'RUNNING' ? '暂停' : '启动'}
                    </span>
                  </div>
                </button>
              )
            })}
          </div>

          {/* Right: detail */}
          <div className="flex w-full flex-col gap-6 lg:w-3/4">
            {selected && (
              <>
                <StrategyOverviewCard
                  name={instanceName(selectedRow, selected.id)}
                  version={selectedRow?.Template?.Version}
                  status={selected.status}
                  totalEquity={selected.total_equity}
                  cnyBalance={selected.cny_balance}
                />
                <NavChartPlaceholder />
                <StrategyJourneyCard
                  createdAt={selectedRow?.CreatedAt}
                  tradeCount={tradesQ.data?.length}
                  loadingTrades={tradesQ.isLoading}
                  version={selectedRow?.Template?.Version}
                />
              </>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

function StrategyOverviewCard({
  name,
  version,
  status,
  totalEquity,
  cnyBalance,
}: {
  name: string
  version?: string
  status: string
  totalEquity: number
  cnyBalance: number
}) {
  const meta = statusMeta(status)
  const holdings = Math.max(totalEquity - cnyBalance, 0)
  return (
    <div className={cardClass}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h2 className="text-base font-semibold text-slate-200">{name}</h2>
          {version && <p className="text-xs text-slate-500">策略版本 {version}</p>}
        </div>
        <span className={`flex items-center gap-1.5 text-sm ${meta.text}`}>
          <span className={`h-2 w-2 rounded-full ${meta.dot}`} />
          {meta.label}
        </span>
      </div>
      <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-3">
        <Metric label="总资产" value={formatCNY(totalEquity)} accent="text-[#2dd4bf]" />
        <Metric label="持仓市值" value={formatCNY(holdings)} />
        <Metric label="可用资金" value={formatCNY(cnyBalance)} />
      </div>
    </div>
  )
}

function Metric({ label, value, accent }: { label: string; value: string; accent?: string }) {
  return (
    <div>
      <p className="text-xs uppercase tracking-wider text-slate-500">{label}</p>
      <p className={`mt-1 font-mono text-lg ${accent ?? 'text-slate-200'}`}>{value}</p>
    </div>
  )
}

function NavChartPlaceholder() {
  return (
    <div className={`${cardClass} flex min-h-[14rem] flex-col`}>
      <div className="flex items-center gap-2 text-sm text-slate-300">
        <TrendingUp className="h-4 w-4 text-[#2dd4bf]" /> 净值曲线
      </div>
      <div className="flex flex-1 flex-col items-center justify-center gap-1 text-center text-slate-600">
        <p className="text-sm">暂无历史净值数据</p>
        <p className="text-xs">策略运行并完成首次结算后，净值曲线将在此展示。</p>
      </div>
    </div>
  )
}

function StrategyJourneyCard({
  createdAt,
  tradeCount,
  loadingTrades,
  version,
}: {
  createdAt?: string
  tradeCount?: number
  loadingTrades: boolean
  version?: string
}) {
  return (
    <div className={cardClass}>
      <h3 className="text-sm font-semibold text-slate-200">策略旅程</h3>
      <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-3">
        <Metric label="创建时间" value={relativeTime(createdAt)} />
        <Metric
          label="累计成交次数"
          value={loadingTrades ? '…' : formatNumber(tradeCount ?? 0, 0)}
        />
        <Metric label="策略版本" value={version ?? '—'} />
      </div>
    </div>
  )
}

function EmptyInstances({ onCreate }: { onCreate: () => void }) {
  return (
    <div className={`${cardClass} flex flex-col items-center justify-center gap-4 py-16 text-center`}>
      <div className="flex h-14 w-14 items-center justify-center rounded-2xl border border-white/[0.06] bg-white/[0.02]">
        <Boxes className="h-6 w-6 text-[#2dd4bf]" />
      </div>
      <div>
        <p className="text-base text-slate-200">还没有策略实例</p>
        <p className="mt-1 text-sm text-slate-500">创建第一个实例，开始运行您的量化策略。</p>
      </div>
      <button
        onClick={onCreate}
        className="flex items-center gap-2 rounded-lg bg-[#2dd4bf] px-4 py-2 text-sm font-semibold text-slate-900 transition-opacity hover:opacity-90"
      >
        <Plus className="h-4 w-4" /> 新建实例
      </button>
    </div>
  )
}
