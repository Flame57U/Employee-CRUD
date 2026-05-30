import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Dna, Loader2, Plus, Trophy, X, Crown, FlaskConical } from 'lucide-react'
import { relativeTime, formatNumber } from '@/lib/format'
import { getTemplates } from '../templates/templatesApi'
import {
  getTasks,
  getChampion,
  createTask,
  promoteChallenger,
  type ChallengerSummary,
  type EvolutionTask,
} from './evolutionApi'

const cardClass = 'rounded-xl border border-white/[0.04] bg-white/[0.02] p-5 backdrop-blur-sm'

const TASK_STATUS: Record<string, { label: string; cls: string }> = {
  pending: { label: '排队中', cls: 'text-slate-400' },
  running: { label: '运行中', cls: 'text-[#2dd4bf]' },
  done: { label: '已完成', cls: 'text-[#34d399]' },
  failed: { label: '失败', cls: 'text-[#f87171]' },
}

export function EvolutionPage() {
  const [strategyId, setStrategyId] = useState<number | null>(null)
  const [creating, setCreating] = useState(false)

  const templatesQ = useQuery({ queryKey: ['strategies'], queryFn: getTemplates })
  const templates = templatesQ.data ?? []
  const effectiveId = strategyId ?? templates[0]?.ID ?? null

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-lg font-semibold text-slate-200">进化实验室</h1>
          <p className="text-sm text-slate-500">
            对策略模板运行遗传算法进化,审阅挑战者并将最优基因晋升为 champion。
          </p>
        </div>
        <button
          onClick={() => setCreating((v) => !v)}
          disabled={effectiveId == null}
          className="flex items-center gap-2 rounded-lg bg-[#2dd4bf] px-4 py-2 text-sm font-semibold text-slate-900 transition-opacity hover:opacity-90 disabled:opacity-50"
        >
          <Plus className="h-4 w-4" /> 新建进化任务
        </button>
      </header>

      {/* Strategy selector */}
      <div className={cardClass}>
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
              className="min-w-[16rem] max-w-md rounded-lg border border-white/[0.08] bg-slate-900/60 px-3 py-2 text-sm text-slate-200 outline-none focus:border-[#2dd4bf]/50"
            >
              {templates.map((t) => (
                <option key={t.ID} value={t.ID}>
                  {t.Name} · {t.Version}
                </option>
              ))}
            </select>
          )}
        </label>
      </div>

      {effectiveId != null && (
        <>
          {creating && (
            <CreateTaskPanel strategyId={effectiveId} onClose={() => setCreating(false)} />
          )}
          <ChampionCard strategyId={effectiveId} />
          <TasksAndChallengers strategyId={effectiveId} />
        </>
      )}
    </div>
  )
}

function ChampionCard({ strategyId }: { strategyId: number }) {
  const championQ = useQuery({
    queryKey: ['champion', strategyId],
    queryFn: () => getChampion(strategyId),
    retry: false,
  })

  return (
    <div className={cardClass}>
      <h2 className="flex items-center gap-2 text-sm font-semibold text-slate-200">
        <Crown className="h-4 w-4 text-[#fbbf24]" /> 当前 Champion 基因
      </h2>
      {championQ.isLoading ? (
        <div className="mt-3 flex items-center gap-2 text-sm text-slate-500">
          <Loader2 className="h-4 w-4 animate-spin" /> 查询中…
        </div>
      ) : championQ.isError ? (
        <p className="mt-3 text-sm text-slate-500">该策略暂无 champion 基因(尚未晋升任何挑战者)。</p>
      ) : (
        <div className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
          <Metric label="基因 ID" value={`#${championQ.data!.ID}`} />
          <Metric label="综合得分" value={formatNumber(championQ.data!.ScoreTotal, 4)} tone="teal" />
          <Metric label="最大回撤" value={formatNumber(championQ.data!.MaxDrawdown, 4)} tone="amber" />
          <Metric label="晋升时间" value={relativeTime(championQ.data!.CreatedAt)} />
        </div>
      )}
    </div>
  )
}

function TasksAndChallengers({ strategyId }: { strategyId: number }) {
  const qc = useQueryClient()
  const tasksQ = useQuery({
    queryKey: ['evolution-tasks', strategyId],
    queryFn: () => getTasks(strategyId),
    refetchInterval: 15_000,
  })

  const promote = useMutation({
    mutationFn: (taskId: number) => promoteChallenger(taskId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['champion', strategyId] })
      qc.invalidateQueries({ queryKey: ['evolution-tasks', strategyId] })
    },
  })

  const tasks = tasksQ.data?.tasks ?? []
  const challengers = tasksQ.data?.challengers ?? []

  return (
    <div className="grid grid-cols-1 gap-6 xl:grid-cols-2">
      {/* Tasks */}
      <div className={`${cardClass} p-0`}>
        <h2 className="flex items-center gap-2 border-b border-white/[0.06] px-5 py-4 text-sm font-semibold text-slate-200">
          <FlaskConical className="h-4 w-4 text-[#2dd4bf]" /> 进化任务
        </h2>
        {tasksQ.isLoading ? (
          <div className="flex items-center gap-2 p-5 text-sm text-slate-500">
            <Loader2 className="h-4 w-4 animate-spin" /> 加载…
          </div>
        ) : tasks.length === 0 ? (
          <p className="p-5 text-sm text-slate-500">暂无进化任务。</p>
        ) : (
          <div className="divide-y divide-white/[0.03]">
            {tasks.map((t) => (
              <TaskRow
                key={t.ID}
                task={t}
                onPromote={() => {
                  if (window.confirm(`将任务 #${t.ID} 的最优挑战者晋升为 champion?该基因将用于运行中的实例。`))
                    promote.mutate(t.ID)
                }}
                promoting={promote.isPending && promote.variables === t.ID}
              />
            ))}
          </div>
        )}
        {promote.isError && (
          <p className="px-5 pb-4 text-sm text-[#f87171]">晋升失败:{(promote.error as Error).message}</p>
        )}
      </div>

      {/* Challengers */}
      <div className={`${cardClass} p-0`}>
        <h2 className="flex items-center gap-2 border-b border-white/[0.06] px-5 py-4 text-sm font-semibold text-slate-200">
          <Dna className="h-4 w-4 text-[#2dd4bf]" /> 挑战者
        </h2>
        {tasksQ.isLoading ? (
          <div className="flex items-center gap-2 p-5 text-sm text-slate-500">
            <Loader2 className="h-4 w-4 animate-spin" /> 加载…
          </div>
        ) : challengers.length === 0 ? (
          <p className="p-5 text-sm text-slate-500">暂无挑战者。</p>
        ) : (
          <div className="divide-y divide-white/[0.03]">
            {challengers.map((c) => (
              <ChallengerRow key={c.id} challenger={c} />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function TaskRow({
  task,
  onPromote,
  promoting,
}: {
  task: EvolutionTask
  onPromote: () => void
  promoting: boolean
}) {
  const meta = TASK_STATUS[task.Status] ?? TASK_STATUS.pending
  return (
    <div className="flex items-center justify-between gap-3 px-5 py-3">
      <div className="min-w-0">
        <p className="text-sm text-slate-200">任务 #{task.ID}</p>
        <p className="text-xs text-slate-500">
          <span className={meta.cls}>{meta.label}</span> · {Math.round(task.Progress * 100)}% ·{' '}
          {relativeTime(task.CreatedAt)}
        </p>
      </div>
      <button
        onClick={onPromote}
        disabled={promoting}
        className="flex shrink-0 items-center gap-1 rounded-md border border-white/[0.06] px-2 py-1 text-xs text-slate-300 transition-colors hover:border-[#fbbf24]/40 hover:text-[#fbbf24] disabled:opacity-50"
      >
        {promoting ? <Loader2 className="h-3 w-3 animate-spin" /> : <Trophy className="h-3 w-3" />}
        晋升
      </button>
    </div>
  )
}

function ChallengerRow({ challenger }: { challenger: ChallengerSummary }) {
  return (
    <div className="flex items-center justify-between gap-3 px-5 py-3">
      <div>
        <p className="text-sm text-slate-200">基因 #{challenger.id}</p>
        <p className="text-xs text-slate-500">{relativeTime(challenger.created_at)}</p>
      </div>
      <div className="flex gap-4 text-right">
        <div>
          <p className="text-xs text-slate-500">得分</p>
          <p className="font-mono text-sm text-[#2dd4bf]">{formatNumber(challenger.score_total, 4)}</p>
        </div>
        <div>
          <p className="text-xs text-slate-500">回撤</p>
          <p className="font-mono text-sm text-[#fbbf24]">{formatNumber(challenger.max_drawdown, 4)}</p>
        </div>
      </div>
    </div>
  )
}

function CreateTaskPanel({ strategyId, onClose }: { strategyId: number; onClose: () => void }) {
  const qc = useQueryClient()
  const [symbol, setSymbol] = useState('')
  const [popSize, setPopSize] = useState(30)
  const [maxGen, setMaxGen] = useState(50)
  const [spawnMode, setSpawnMode] = useState('inherit')

  const create = useMutation({
    mutationFn: () =>
      createTask({
        strategy_id: strategyId,
        symbol: symbol.trim(),
        pop_size: popSize,
        max_generations: maxGen,
        spawn_mode: spawnMode,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['evolution-tasks', strategyId] })
      onClose()
    },
  })

  return (
    <div className={cardClass}>
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold text-slate-200">新建进化任务</h2>
        <button onClick={onClose} className="text-slate-500 hover:text-slate-300">
          <X className="h-4 w-4" />
        </button>
      </div>
      <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Field label="标的代码 (symbol)">
          <input
            value={symbol}
            onChange={(e) => setSymbol(e.target.value)}
            placeholder="例如 510300.SH"
            className="w-full rounded-lg border border-white/[0.08] bg-slate-900/60 px-3 py-2 text-sm text-slate-200 outline-none focus:border-[#2dd4bf]/50"
          />
        </Field>
        <Field label="种群大小 (pop_size)">
          <input
            type="number"
            min={2}
            value={popSize}
            onChange={(e) => setPopSize(Number(e.target.value))}
            className="w-full rounded-lg border border-white/[0.08] bg-slate-900/60 px-3 py-2 text-sm text-slate-200 outline-none focus:border-[#2dd4bf]/50"
          />
        </Field>
        <Field label="最大代数 (max_generations)">
          <input
            type="number"
            min={1}
            value={maxGen}
            onChange={(e) => setMaxGen(Number(e.target.value))}
            className="w-full rounded-lg border border-white/[0.08] bg-slate-900/60 px-3 py-2 text-sm text-slate-200 outline-none focus:border-[#2dd4bf]/50"
          />
        </Field>
        <Field label="种子模式 (spawn_mode)">
          <select
            value={spawnMode}
            onChange={(e) => setSpawnMode(e.target.value)}
            className="w-full rounded-lg border border-white/[0.08] bg-slate-900/60 px-3 py-2 text-sm text-slate-200 outline-none focus:border-[#2dd4bf]/50"
          >
            <option value="inherit">inherit（继承现有 champion）</option>
            <option value="random_once">random_once（随机初始化一次）</option>
          </select>
        </Field>
      </div>
      <div className="mt-4 flex items-center gap-3">
        <button
          onClick={() => create.mutate()}
          disabled={create.isPending || symbol.trim() === ''}
          className="flex items-center gap-2 rounded-lg bg-[#2dd4bf] px-4 py-2 text-sm font-semibold text-slate-900 transition-opacity hover:opacity-90 disabled:opacity-50"
        >
          {create.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />}
          启动进化
        </button>
        {create.isError && (
          <span className="text-sm text-[#f87171]">{(create.error as Error).message}</span>
        )}
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

function Metric({
  label,
  value,
  tone = 'slate',
}: {
  label: string
  value: string
  tone?: 'slate' | 'teal' | 'amber'
}) {
  const tones = { slate: 'text-slate-200', teal: 'text-[#2dd4bf]', amber: 'text-[#fbbf24]' }
  return (
    <div className="rounded-lg border border-white/[0.04] bg-white/[0.02] p-3">
      <p className="text-xs text-slate-500">{label}</p>
      <p className={`mt-1 font-mono text-sm ${tones[tone]}`}>{value}</p>
    </div>
  )
}
