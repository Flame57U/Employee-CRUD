import { Construction } from 'lucide-react'

export function ComingSoonPage({ title }: { title: string }) {
  return (
    <div className="flex min-h-[60vh] flex-col items-center justify-center gap-4 text-center">
      <div className="flex h-14 w-14 items-center justify-center rounded-2xl border border-white/[0.06] bg-white/[0.02]">
        <Construction className="h-6 w-6 text-[#fbbf24]" />
      </div>
      <h2 className="text-xl font-semibold text-slate-200">{title}</h2>
      <p className="max-w-sm text-sm text-slate-500">该模块即将上线。当前版本已开放账户总览。</p>
    </div>
  )
}
