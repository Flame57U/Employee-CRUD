import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Loader2, RefreshCw, Search, TrendingUp, TrendingDown } from 'lucide-react'
import { formatNumber } from '@/lib/format'
import { ApiRequestError } from '@/lib/api'
import { getQuote, type Asset, type Quote } from './marketApi'

const cardClass = 'rounded-xl border border-white/[0.04] bg-white/[0.02] p-4 backdrop-blur-sm'

interface Preset {
  label: string
  asset: Asset
  region: string
  code: string
}

// A few ready-made symbols so the page shows live data on first load.
const PRESETS: Preset[] = [
  { label: 'GBP/USD', asset: 'forex', region: 'gb', code: 'GBPUSD' },
  { label: 'EUR/USD', asset: 'forex', region: 'gb', code: 'EURUSD' },
  { label: 'BTC/USDT', asset: 'crypto', region: 'ba', code: 'BTCUSDT' },
  { label: 'ETH/USDT', asset: 'crypto', region: 'ba', code: 'ETHUSDT' },
  { label: 'AAPL', asset: 'stock', region: 'us', code: 'AAPL' },
]

const ASSET_LABEL: Record<Asset, string> = { forex: '外汇', crypto: '加密货币', stock: '股票' }

export function MarketPage() {
  const [form, setForm] = useState<Preset>(PRESETS[0])
  // The "active" query is committed only on submit/preset click, not per keystroke.
  const [active, setActive] = useState<Preset>(PRESETS[0])

  const quoteQ = useQuery({
    queryKey: ['quote', active.asset, active.region, active.code],
    queryFn: () => getQuote(active.asset, active.region, active.code),
    refetchInterval: 5_000,
    enabled: Boolean(active.region && active.code),
  })

  const submit = (e: React.FormEvent) => {
    e.preventDefault()
    setActive({ ...form })
  }

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-lg font-semibold text-slate-200">实时行情</h1>
          <p className="text-sm text-slate-500">数据来源 iTick · 每 5 秒自动刷新</p>
        </div>
        <button
          onClick={() => quoteQ.refetch()}
          disabled={quoteQ.isFetching}
          className="flex items-center gap-2 rounded-lg border border-white/[0.06] px-3 py-2 text-sm text-slate-300 transition-colors hover:border-[#2dd4bf]/40 hover:text-[#2dd4bf] disabled:opacity-50"
        >
          <RefreshCw className={`h-4 w-4 ${quoteQ.isFetching ? 'animate-spin' : ''}`} /> 刷新
        </button>
      </header>

      {/* Presets */}
      <div className="flex flex-wrap gap-2">
        {PRESETS.map((p) => {
          const on = p.code === active.code && p.region === active.region && p.asset === active.asset
          return (
            <button
              key={p.label}
              onClick={() => {
                setForm(p)
                setActive(p)
              }}
              className={[
                'rounded-lg border px-3 py-1.5 text-xs transition-colors',
                on
                  ? 'border-[#2dd4bf]/40 bg-[#2dd4bf]/[0.08] text-[#2dd4bf]'
                  : 'border-white/[0.06] text-slate-400 hover:border-white/15 hover:text-slate-200',
              ].join(' ')}
            >
              {p.label}
            </button>
          )
        })}
      </div>

      {/* Manual query */}
      <form onSubmit={submit} className={`${cardClass} flex flex-wrap items-end gap-3`}>
        <Field label="资产类别">
          <select
            value={form.asset}
            onChange={(e) => setForm({ ...form, asset: e.target.value as Asset })}
            className="rounded-lg border border-white/[0.08] bg-slate-900/60 px-3 py-2 text-sm text-slate-200 outline-none focus:border-[#2dd4bf]/50"
          >
            {(['forex', 'crypto', 'stock'] as Asset[]).map((a) => (
              <option key={a} value={a}>
                {ASSET_LABEL[a]}
              </option>
            ))}
          </select>
        </Field>
        <Field label="区域 region">
          <input
            value={form.region}
            onChange={(e) => setForm({ ...form, region: e.target.value })}
            placeholder="gb / ba / us"
            className="w-28 rounded-lg border border-white/[0.08] bg-slate-900/60 px-3 py-2 text-sm text-slate-200 outline-none focus:border-[#2dd4bf]/50"
          />
        </Field>
        <Field label="代码 code">
          <input
            value={form.code}
            onChange={(e) => setForm({ ...form, code: e.target.value.toUpperCase() })}
            placeholder="GBPUSD"
            className="w-40 rounded-lg border border-white/[0.08] bg-slate-900/60 px-3 py-2 font-mono text-sm text-slate-200 outline-none focus:border-[#2dd4bf]/50"
          />
        </Field>
        <button
          type="submit"
          className="flex items-center gap-2 rounded-lg bg-[#2dd4bf] px-4 py-2 text-sm font-semibold text-slate-900 transition-opacity hover:opacity-90"
        >
          <Search className="h-4 w-4" /> 查询
        </button>
      </form>

      {/* Result */}
      {quoteQ.isLoading ? (
        <div className="flex min-h-[30vh] items-center justify-center text-slate-500">
          <Loader2 className="mr-2 h-5 w-5 animate-spin" /> 拉取行情…
        </div>
      ) : quoteQ.isError ? (
        <div className={`${cardClass} text-sm text-[#f87171]`}>{errorText(quoteQ.error)}</div>
      ) : quoteQ.data ? (
        <QuoteCard q={quoteQ.data} assetLabel={ASSET_LABEL[active.asset]} />
      ) : null}
    </div>
  )
}

function QuoteCard({ q, assetLabel }: { q: Quote; assetLabel: string }) {
  const up = q.ch >= 0
  const accent = up ? 'text-[#34d399]' : 'text-[#f87171]'
  const Arrow = up ? TrendingUp : TrendingDown
  const sign = up ? '+' : ''
  return (
    <div className={cardClass}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-baseline gap-3">
          <h2 className="font-mono text-xl text-slate-100">{q.s}</h2>
          <span className="rounded-md border border-white/[0.06] px-2 py-0.5 text-xs text-slate-400">
            {assetLabel} · {q.r}
          </span>
        </div>
        <span className="text-xs text-slate-500">{new Date(q.t).toLocaleString('zh-CN')}</span>
      </div>

      <div className="mt-4 flex flex-wrap items-end gap-4">
        <span className={`font-mono text-4xl ${accent}`}>{formatNumber(q.ld, priceDigits(q.ld))}</span>
        <span className={`flex items-center gap-1 text-sm ${accent}`}>
          <Arrow className="h-4 w-4" />
          {sign}
          {formatNumber(q.ch, priceDigits(q.ch))} ({sign}
          {formatNumber(q.chp, 2)}%)
        </span>
      </div>

      <div className="mt-5 grid grid-cols-2 gap-4 sm:grid-cols-4">
        <Metric label="开盘" value={formatNumber(q.o, priceDigits(q.o))} />
        <Metric label="最高" value={formatNumber(q.h, priceDigits(q.h))} />
        <Metric label="最低" value={formatNumber(q.l, priceDigits(q.l))} />
        <Metric label="成交量" value={formatNumber(q.v, 2)} />
      </div>
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-xs uppercase tracking-wider text-slate-500">{label}</span>
      {children}
    </label>
  )
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-xs uppercase tracking-wider text-slate-500">{label}</p>
      <p className="mt-1 font-mono text-base text-slate-200">{value}</p>
    </div>
  )
}

// FX prices need more decimals than crypto/stock; pick by magnitude.
function priceDigits(v: number): number {
  const a = Math.abs(v)
  if (a !== 0 && a < 10) return 5
  return 2
}

function errorText(err: unknown): string {
  if (err instanceof ApiRequestError && err.status === 503) {
    return '服务端未配置 iTick token（ITICK_TOKEN），暂无法获取行情。'
  }
  return `获取行情失败：${(err as Error).message}`
}
