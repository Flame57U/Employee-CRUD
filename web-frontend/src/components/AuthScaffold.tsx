import { Activity } from 'lucide-react'
import type { ReactNode } from 'react'
import { AppBackground } from './AppBackground'

// Centred glass card over the cosmic backdrop — shared by Login and Register.
export function AuthScaffold({
  slogan,
  children,
}: {
  slogan: string
  children: ReactNode
}) {
  return (
    <div className="relative flex min-h-screen items-center justify-center p-4">
      <AppBackground />
      <div className="w-[400px] max-w-full rounded-2xl border border-white/10 bg-slate-900/60 p-8 backdrop-blur-xl">
        <div className="mb-8 flex flex-col items-center text-center">
          <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-[#ff8c6b]/10 shadow-[0_0_30px_rgba(255,140,107,0.35)]">
            <Activity className="h-6 w-6 text-[#ff8c6b]" />
          </div>
          <h1 className="text-2xl font-semibold tracking-wider text-slate-200">QuantSaaS</h1>
          <p className="mt-1 text-sm text-slate-400">{slogan}</p>
        </div>
        {children}
      </div>
    </div>
  )
}
