import { useState } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { LineChart, Loader2, Play, CheckCircle2, XCircle, Clock } from 'lucide-react'
import { relativeTime } from '@/lib/format'
import { getTemplates } from '../templates/templatesApi'
import { createBacktest, getBacktest } from './backtestApi'

const cardClass = 'rounded-xl border border-white/[0.04] bg-white/[0.02] p-5 backdrop-blur-sm'

// Minimal placeholder; the async worker expects a chromosome + spawn point.
const PARAM_PACK_SAMPLE = `{
  "chromosome": {},
  "spawn_point": {}
}`

const STATUS_META: Record<string, { label: string; cls: string; icon: typeof Clock }> = {
  pending: { label: '排队中', cls: 'text-slate-400', icon: Clock },
  running: { label: '运行中', cls: 'text-[#2dd4bf]', icon: Loader2 },
  done: { label: '已完成', cls: 'text-[#34d399]', icon: CheckCircle2 },
  failed: { label: '失败', cls: 'text-[#f87171]', icon: XCircle },
}

export function BacktestPage() {
  const [strategyId, setStrategyId] = useState<number | null>(null)
  const [symbol, setSymbol] = useState('')
  const [paramPack, setParamPack] = useState(PARAM_PACK_SAMPLE)
  const [jsonError, setJsonError] = useState<string | null>(null)
  const [activeId, setActiveId] = useState<number | null>(null)

  const templatesQ = useQuery({ queryKey: ['strategies'], queryFn: getTemplates })
  const templates = templatesQ.data ?? []
  const effectiveId = strategyId ?? templates[0]?.ID ?? null

  const create = useMutation({
    mutationFn: (body: { template_id: number; symbol: string; param_pack: unknown }) =>
      createBacktest(body),
    onSuccess: (row) => setActiveId(row.ID),
  })

  const onSubmit = () => {
    setJsonError(null)
    let parsed: unknown
    try {
      parsed = JSON.parse(paramPack)
    } catch {
      setJsonError('param_pack 不是合法的 JSON')
      return
    }
    if (effectiveId == null) return
    create.mutate({ template_id: effectiveId, symbol: symbol.trim(), param_pack: parsed })
  }

  return (
    <div className="flex flex-col gap-6">
      <header>
        <h1 className="text-lg font-semibold text-slate-200">回测</h1>
        <p className="text-sm text-slate-500">
          针对策略模板与参数组合提交一次回测,异步运行完成后查看结果。
        </p>
      </header>

      <div className={cardClass}>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <label className="flex flex-col gap-1.5">
            <span className="text-xs uppercase tracking-wider text-slate-500">策略模板</span>
            {templatesQ.isLoading ? (
              <span className="flex items-center gap-2 text-sm text-slate-500">
                <Loader2 className="h-4 w-4 animate-spin" /> 加载模板…
              </span>
            ) : templates.length === 0 ? (
              <span className="text-sm text-slate-500">暂无可用策略模板。</span>
            ) : (
              <select
                value={effectiveId ?? ''}
                onChange={(e) => setStrategyId(Number(e.target.value))}
                className="rounded-lg border border-white/[0.08] bg-slate-900/60 px-3 py-2 text-sm text-slate-200 outline-none focus:border-[#2dd4bf]/50"
              >
                {templates.map((t) => (
                  <option key={t.ID} value={t.ID}>
                    {t.Name} · {t.Version}
                  </option>
                ))}
              </select>
            )}
          </label>
          <label className="flex flex-col gap-1.5">
            <span className="text-xs uppercase tracking-wider text-slate-500">标的代码 (symbol)</span>
            <input
              value={symbol}
              onChange={(e) => setSymbol(e.target.value)}
              placeholder="例如 510300.SH"
              className="rounded-lg border border-white/[0.08] bg-slate-900/60 px-3 py-2 text-sm text-slate-200 outline-none focus:border-[#2dd4bf]/50"
            />
          </label>
        </div>

        <label className="mt-4 flex flex-col gap-1.5">
          <span className="text-xs uppercase tracking-wider text-slate-500">参数包 (param_pack, JSON)</span>
          <textarea
            value={paramPack}
            onChange={(e) => setParamPack(e.target.value)}
            rows={8}
            spellCheck={false}
            className="custom-scrollbar rounded-lg border border-white/[0.08] bg-[#020617]/60 px-3 py-2 font-mono text-xs text-slate-200 outline-none focus:border-[#2dd4bf]/50"
          />
        </label>
        {jsonError && <p className="mt-2 text-sm text-[#f87171]">{jsonError}</p>}

        <div className="mt-4 flex items-center gap-3">
          <button
            onClick={onSubmit}
            disabled={create.isPending || effectiveId == null || symbol.trim() === ''}
            className="flex items-center gap-2 rounded-lg bg-[#2dd4bf] px-4 py-2 text-sm font-semibold text-slate-900 transition-opacity hover:opacity-90 disabled:opacity-50"
          >
            {create.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Play className="h-4 w-4" />}
            提交回测
          </button>
          {create.isError && (
            <span className="text-sm text-[#f87171]">{(create.error as Error).message}</span>
          )}
        </div>
      </div>

      {activeId != null && <BacktestResult id={activeId} />}
    </div>
  )
}

function BacktestResult({ id }: { id: number }) {
  const q = useQuery({
    queryKey: ['backtest', id],
    queryFn: () => getBacktest(id),
    // Poll while pending/running; stop once terminal.
    refetchInterval: (query) => {
      const s = query.state.data?.Status
      return s === 'done' || s === 'failed' ? false : 3_000
    },
  })

  if (q.isLoading) {
    return (
      <div className={`${cardClass} flex items-center gap-2 text-sm text-slate-500`}>
        <Loader2 className="h-4 w-4 animate-spin" /> 加载回测 #{id}…
      </div>
    )
  }
  if (q.isError) {
    return <div className={`${cardClass} text-sm text-[#f87171]`}>加载失败:{(q.error as Error).message}</div>
  }

  const bt = q.data!
  const meta = STATUS_META[bt.Status] ?? STATUS_META.pending
  const Icon = meta.icon
  const running = bt.Status === 'pending' || bt.Status === 'running'

  return (
    <div className={cardClass}>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="flex items-center gap-2 text-sm font-semibold text-slate-200">
          <LineChart className="h-4 w-4 text-[#2dd4bf]" /> 回测 #{bt.ID} · {bt.Symbol}
        </h2>
        <span className={`flex items-center gap-1.5 text-sm ${meta.cls}`}>
          <Icon className={`h-4 w-4 ${bt.Status === 'running' ? 'animate-spin' : ''}`} />
          {meta.label}
        </span>
      </div>
      <p className="mt-1 text-xs text-slate-500">提交于 {relativeTime(bt.CreatedAt)}</p>

      {running && (
        <p className="mt-4 text-sm text-slate-500">回测正在异步运行,完成后将自动刷新结果…</p>
      )}

      {bt.Status === 'failed' && bt.ErrorMessage && (
        <p className="mt-4 rounded-lg border border-[#f87171]/20 bg-[#f87171]/[0.05] px-3 py-2 text-sm text-[#f87171]">
          {bt.ErrorMessage}
        </p>
      )}

      {bt.Status === 'done' && bt.Result != null && (
        <pre className="custom-scrollbar mt-4 max-h-96 overflow-auto rounded-lg border border-white/[0.06] bg-[#020617]/60 p-4 font-mono text-xs leading-relaxed text-slate-300">
          {JSON.stringify(bt.Result, null, 2)}
        </pre>
      )}
    </div>
  )
}
