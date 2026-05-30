import { apiRequest } from '@/lib/api'

// Backtest mirrors store.Backtest (GORM PascalCase fields).
export interface Backtest {
  ID: number
  CreatedAt: string
  UserID: number
  TemplateID: number
  Symbol: string
  Status: string // pending / running / done / failed
  ParamPack: unknown
  Result: unknown
  ErrorMessage: string
}

export interface CreateBacktestBody {
  template_id: number
  symbol: string
  param_pack: unknown
}

export function createBacktest(body: CreateBacktestBody) {
  return apiRequest<Backtest>('/backtests', { method: 'POST', body })
}

export function getBacktest(id: number) {
  return apiRequest<Backtest>(`/backtests/${id}`)
}
