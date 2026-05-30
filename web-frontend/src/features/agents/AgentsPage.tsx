import { useQuery } from '@tanstack/react-query'
import {
  Cpu,
  Wifi,
  WifiOff,
  RefreshCw,
  Loader2,
  Terminal,
  Boxes,
  ShieldCheck,
} from 'lucide-react'
import { getAgentStatus } from './agentApi'

const cardClass = 'rounded-xl border border-white/[0.04] bg-white/[0.02] p-5 backdrop-blur-sm'

// Connection config the local Agent binary reads from config.agent.yaml.
const CONFIG_SAMPLE = `saas_url: "ws://31.97.13.54:8080/ws/agent"
email: "你的账号邮箱"
password: "你的账号密码"
broker:
  name: "your-broker"
  api_key: "..."
  secret_key: "..."
  simulated: true   # 先用模拟盘验证连通`

export function AgentsPage() {
  const statusQ = useQuery({
    queryKey: ['agent-status'],
    queryFn: getAgentStatus,
    refetchInterval: 10_000,
  })

  const connected = statusQ.data?.connected ?? false
  const agentId = statusQ.data?.user_id

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-lg font-semibold text-slate-200">Agent 管理</h1>
          <p className="text-sm text-slate-500">
            Agent 是运行在你本地的执行代理，负责把运行中实例产生的订单落地到券商。
          </p>
        </div>
        <button
          onClick={() => statusQ.refetch()}
          disabled={statusQ.isFetching}
          className="flex items-center gap-2 rounded-lg border border-white/[0.08] px-3 py-2 text-sm text-slate-300 transition-colors hover:border-white/20 disabled:opacity-50"
        >
          <RefreshCw className={`h-4 w-4 ${statusQ.isFetching ? 'animate-spin' : ''}`} />
          刷新
        </button>
      </header>

      {/* Status hero */}
      <div className={cardClass}>
        {statusQ.isLoading ? (
          <div className="flex items-center gap-2 text-slate-500">
            <Loader2 className="h-5 w-5 animate-spin" /> 查询连接状态…
          </div>
        ) : statusQ.isError ? (
          <p className="text-sm text-[#f87171]">状态查询失败：{(statusQ.error as Error).message}</p>
        ) : (
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div className="flex items-center gap-4">
              <div
                className={`flex h-14 w-14 items-center justify-center rounded-2xl border ${
                  connected
                    ? 'border-[#34d399]/30 bg-[#34d399]/[0.08]'
                    : 'border-white/[0.06] bg-white/[0.02]'
                }`}
              >
                {connected ? (
                  <Wifi className="h-6 w-6 text-[#34d399]" />
                ) : (
                  <WifiOff className="h-6 w-6 text-slate-500" />
                )}
              </div>
              <div>
                <div className="flex items-center gap-2">
                  <span
                    className={`h-2 w-2 rounded-full ${connected ? 'bg-[#34d399]' : 'bg-slate-600'}`}
                    style={connected ? { boxShadow: '0 0 8px #34d399' } : undefined}
                  />
                  <span className={`text-base font-semibold ${connected ? 'text-[#34d399]' : 'text-slate-400'}`}>
                    {connected ? 'Agent 在线' : 'Agent 离线'}
                  </span>
                </div>
                <p className="mt-1 text-sm text-slate-500">
                  {connected
                    ? '已连接到平台，运行中实例的订单将实时下发执行。'
                    : '未检测到连接，订单会排队等待 Agent 重新连接后再执行。'}
                </p>
              </div>
            </div>
            {agentId != null && (
              <div className="flex items-center gap-2 rounded-lg border border-white/[0.06] px-3 py-1.5 text-xs text-slate-400">
                <Cpu className="h-3.5 w-3.5" /> Agent ID: <span className="font-mono text-slate-300">{agentId}</span>
              </div>
            )}
          </div>
        )}
      </div>

      {/* How it works */}
      <div className={cardClass}>
        <h2 className="flex items-center gap-2 text-sm font-semibold text-slate-200">
          <ShieldCheck className="h-4 w-4 text-[#2dd4bf]" /> 工作原理
        </h2>
        <ul className="mt-3 flex flex-col gap-2 text-sm text-slate-400">
          <li className="flex gap-2">
            <Boxes className="mt-0.5 h-4 w-4 shrink-0 text-slate-500" />
            平台每分钟扫描运行中的实例，策略计算出的订单通过 WebSocket 下发给你的 Agent。
          </li>
          <li className="flex gap-2">
            <Cpu className="mt-0.5 h-4 w-4 shrink-0 text-slate-500" />
            Agent 用账号登录获取 JWT 后连接平台；每个账号同一时刻只保留一条连接，新连接会顶替旧连接。
          </li>
          <li className="flex gap-2">
            <WifiOff className="mt-0.5 h-4 w-4 shrink-0 text-slate-500" />
            Agent 离线时订单不会丢失，会排队等待重连后补发——但策略实时性会受影响。
          </li>
        </ul>
      </div>

      {/* How to connect */}
      <div className={cardClass}>
        <h2 className="flex items-center gap-2 text-sm font-semibold text-slate-200">
          <Terminal className="h-4 w-4 text-[#2dd4bf]" /> 如何连接 Agent
        </h2>
        <ol className="mt-3 flex list-decimal flex-col gap-2 pl-5 text-sm text-slate-400 marker:text-slate-600">
          <li>
            在本地准备配置文件 <code className="rounded bg-white/[0.06] px-1.5 py-0.5 font-mono text-xs text-slate-300">config.agent.yaml</code>：
          </li>
        </ol>
        <pre className="custom-scrollbar mt-3 overflow-x-auto rounded-lg border border-white/[0.06] bg-[#020617]/60 p-4 font-mono text-xs leading-relaxed text-slate-300">
          {CONFIG_SAMPLE}
        </pre>
        <p className="mt-3 text-sm text-slate-400">然后运行 Agent 二进制：</p>
        <pre className="mt-2 overflow-x-auto rounded-lg border border-white/[0.06] bg-[#020617]/60 p-4 font-mono text-xs text-slate-300">
          ./agent -config config.agent.yaml
        </pre>
        <p className="mt-3 text-xs text-slate-500">
          连接成功后,本页状态会在约 10 秒内变为「Agent 在线」。请妥善保管 broker 密钥，凭据仅保存在本地配置文件中。
        </p>
      </div>
    </div>
  )
}
