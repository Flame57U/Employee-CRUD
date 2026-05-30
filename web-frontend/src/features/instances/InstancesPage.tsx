import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Play, Pause, Plus, Boxes, Loader2, Trash2, ExternalLink, X } from 'lucide-react'
import { relativeTime } from '@/lib/format'
import { ApiRequestError } from '@/lib/api'
import {
  getInstances,
  getStrategies,
  createInstance,
  deleteInstance,
  startInstance,
  stopInstance,
  type InstanceRow,
} from '../dashboard/dashboardApi'

const STATUS_META: Record<string, { label: string; dot: string; text: string }> = {
  RUNNING: { label: '运行中', dot: 'bg-[#34d399]', text: 'text-[#34d399]' },
  STOPPED: { label: '已暂停', dot: 'bg-slate-500', text: 'text-slate-400' },
  ERROR: { label: '异常', dot: 'bg-[#f87171]', text: 'text-[#f87171]' },
}

function statusMeta(s: string) {
  return STATUS_META[s] ?? STATUS_META.STOPPED
}

const cardClass = 'rounded-xl border border-white/[0.04] bg-white/[0.02] p-4 backdrop-blur-sm'

export function InstancesPage() {
  const qc = useQueryClient()
  const navigate = useNavigate()
  const [creating, setCreating] = useState(false)

  const instancesQ = useQuery({
    queryKey: ['instances'],
    queryFn: getInstances,
    refetchInterval: 30_000,
  })

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ['instances'] })
    qc.invalidateQueries({ queryKey: ['dashboard'] })
  }

  const toggle = useMutation({
    mutationFn: (inst: InstanceRow) =>
      inst.Status === 'RUNNING' ? stopInstance(inst.ID) : startInstance(inst.ID),
    onSuccess: invalidate,
  })

  const remove = useMutation({
    mutationFn: (id: number) => deleteInstance(id),
    onSuccess: invalidate,
  })

  const instances = instancesQ.data ?? []

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-lg font-semibold text-slate-200">实例管理</h1>
          <p className="text-sm text-slate-500">从策略模板创建实例，控制其运行状态</p>
        </div>
        <button
          onClick={() => setCreating(true)}
          className="flex items-center gap-2 rounded-lg bg-[#2dd4bf] px-4 py-2 text-sm font-semibold text-slate-900 transition-opacity hover:opacity-90"
        >
          <Plus className="h-4 w-4" /> 新建实例
        </button>
      </header>

      {creating && <CreateInstancePanel onClose={() => setCreating(false)} onCreated={invalidate} />}

      {instancesQ.isLoading ? (
        <div className="flex min-h-[40vh] items-center justify-center text-slate-500">
          <Loader2 className="mr-2 h-5 w-5 animate-spin" /> 加载实例…
        </div>
      ) : instances.length === 0 ? (
        <EmptyInstances onCreate={() => setCreating(true)} />
      ) : (
        <div className={`${cardClass} overflow-x-auto p-0`}>
          <table className="w-full min-w-[640px] text-sm">
            <thead>
              <tr className="border-b border-white/[0.06] text-left text-xs uppercase tracking-wider text-slate-500">
                <th className="px-4 py-3 font-medium">实例</th>
                <th className="px-4 py-3 font-medium">版本</th>
                <th className="px-4 py-3 font-medium">状态</th>
                <th className="px-4 py-3 font-medium">创建时间</th>
                <th className="px-4 py-3 text-right font-medium">操作</th>
              </tr>
            </thead>
            <tbody>
              {instances.map((inst) => {
                const meta = statusMeta(inst.Status)
                const busy = toggle.isPending && toggle.variables?.ID === inst.ID
                const deleting = remove.isPending && remove.variables === inst.ID
                return (
                  <tr key={inst.ID} className="border-b border-white/[0.03] last:border-0">
                    <td className="px-4 py-3">
                      <span className="text-slate-200">
                        {inst.Template?.Name ?? '策略'} #{inst.ID}
                      </span>
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-slate-400">
                      {inst.Template?.Version ?? '—'}
                    </td>
                    <td className="px-4 py-3">
                      <span className={`flex items-center gap-1.5 text-xs ${meta.text}`}>
                        <span className={`h-1.5 w-1.5 rounded-full ${meta.dot}`} />
                        {meta.label}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-xs text-slate-400">{relativeTime(inst.CreatedAt)}</td>
                    <td className="px-4 py-3">
                      <div className="flex items-center justify-end gap-2">
                        <button
                          onClick={() => !busy && toggle.mutate(inst)}
                          disabled={busy}
                          className="flex items-center gap-1 rounded-md border border-white/[0.06] px-2 py-1 text-xs text-slate-300 transition-colors hover:border-[#2dd4bf]/40 hover:text-[#2dd4bf] disabled:opacity-50"
                        >
                          {busy ? (
                            <Loader2 className="h-3 w-3 animate-spin" />
                          ) : inst.Status === 'RUNNING' ? (
                            <Pause className="h-3 w-3" />
                          ) : (
                            <Play className="h-3 w-3" />
                          )}
                          {inst.Status === 'RUNNING' ? '暂停' : '启动'}
                        </button>
                        <button
                          onClick={() => navigate(`/?instance=${inst.ID}`)}
                          className="flex items-center gap-1 rounded-md border border-white/[0.06] px-2 py-1 text-xs text-slate-300 transition-colors hover:border-white/20 hover:text-slate-100"
                        >
                          <ExternalLink className="h-3 w-3" /> 详情
                        </button>
                        <button
                          onClick={() => {
                            if (
                              !deleting &&
                              window.confirm(`确定删除实例 #${inst.ID}？此操作不可撤销。`)
                            )
                              remove.mutate(inst.ID)
                          }}
                          disabled={deleting}
                          className="flex items-center gap-1 rounded-md border border-white/[0.06] px-2 py-1 text-xs text-slate-400 transition-colors hover:border-[#f87171]/40 hover:text-[#f87171] disabled:opacity-50"
                        >
                          {deleting ? (
                            <Loader2 className="h-3 w-3 animate-spin" />
                          ) : (
                            <Trash2 className="h-3 w-3" />
                          )}
                        </button>
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}

      {remove.isError && (
        <p className="text-sm text-[#f87171]">删除失败：{(remove.error as Error).message}</p>
      )}
    </div>
  )
}

function CreateInstancePanel({
  onClose,
  onCreated,
}: {
  onClose: () => void
  onCreated: () => void
}) {
  const [templateId, setTemplateId] = useState<number | null>(null)

  const strategiesQ = useQuery({ queryKey: ['strategies'], queryFn: getStrategies })
  const templates = strategiesQ.data ?? []

  const create = useMutation({
    mutationFn: (id: number) => createInstance(id),
    onSuccess: () => {
      onCreated()
      onClose()
    },
  })

  const effectiveId = templateId ?? templates[0]?.ID ?? null

  return (
    <div className={cardClass}>
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold text-slate-200">新建实例</h2>
        <button onClick={onClose} className="text-slate-500 hover:text-slate-300">
          <X className="h-4 w-4" />
        </button>
      </div>

      {strategiesQ.isLoading ? (
        <div className="mt-4 flex items-center gap-2 text-sm text-slate-500">
          <Loader2 className="h-4 w-4 animate-spin" /> 加载策略模板…
        </div>
      ) : templates.length === 0 ? (
        <p className="mt-4 text-sm text-slate-500">
          暂无可用的策略模板。模板由管理员预置，请稍后再试。
        </p>
      ) : (
        <div className="mt-4 flex flex-wrap items-end gap-3">
          <label className="flex flex-col gap-1.5">
            <span className="text-xs uppercase tracking-wider text-slate-500">策略模板</span>
            <select
              value={effectiveId ?? ''}
              onChange={(e) => setTemplateId(Number(e.target.value))}
              className="min-w-[16rem] rounded-lg border border-white/[0.08] bg-slate-900/60 px-3 py-2 text-sm text-slate-200 outline-none focus:border-[#2dd4bf]/50"
            >
              {templates.map((t) => (
                <option key={t.ID} value={t.ID}>
                  {t.Name} · {t.Version} {t.IsSpot ? '(现货)' : ''}
                </option>
              ))}
            </select>
          </label>
          <button
            onClick={() => effectiveId && create.mutate(effectiveId)}
            disabled={create.isPending || effectiveId == null}
            className="flex items-center gap-2 rounded-lg bg-[#2dd4bf] px-4 py-2 text-sm font-semibold text-slate-900 transition-opacity hover:opacity-90 disabled:opacity-50"
          >
            {create.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />}
            创建
          </button>
        </div>
      )}

      {create.isError && <p className="mt-3 text-sm text-[#f87171]">{createError(create.error)}</p>}
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

function EmptyInstances({ onCreate }: { onCreate: () => void }) {
  return (
    <div className={`${cardClass} flex flex-col items-center justify-center gap-4 py-16 text-center`}>
      <div className="flex h-14 w-14 items-center justify-center rounded-2xl border border-white/[0.06] bg-white/[0.02]">
        <Boxes className="h-6 w-6 text-[#2dd4bf]" />
      </div>
      <div>
        <p className="text-base text-slate-200">还没有策略实例</p>
        <p className="mt-1 text-sm text-slate-500">从策略模板创建第一个实例，开始运行您的量化策略。</p>
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
