import { apiRequest } from '@/lib/api'

// Policy mirrors quant.Policy. The Go struct carries no json tags, so GORM/encoding
// serializes these nested fields with their PascalCase names.
export interface TemplatePolicy {
  Symbol?: string
  AssetClass?: string // "A_STOCK_ETF" | "GOLD_ETF"
  TotalCapitalCNY?: number
  MonthlyInjectCNY?: number
  DeadlineRatio?: number
  MacroMinOrderCNY?: number
  QuietDustThreshold?: number
  MaxLotsPerTick?: number
}

export interface TemplateSpawnPoint {
  Policy?: TemplatePolicy
  Risk?: Record<string, unknown>
}

// Manifest is a free-form jsonb blob. Two shapes exist in practice:
//  - seeded templates carry a simple { desc, engine } summary;
//  - the runtime shape (instance.TemplateManifest) adds { symbol, interval, spawn_point }.
// All fields are optional so the UI degrades gracefully whichever is present.
export interface TemplateManifest {
  desc?: string
  engine?: string
  symbol?: string
  interval?: string
  spawn_point?: TemplateSpawnPoint
}

// /strategies serializes GORM structs with default (PascalCase) field names.
export interface StrategyTemplate {
  ID: number
  CreatedAt: string
  UpdatedAt: string
  Name: string
  Version: string
  IsSpot: boolean
  Manifest: TemplateManifest | null
}

export function getTemplates() {
  return apiRequest<{ strategies: StrategyTemplate[] }>('/strategies').then((r) => r.strategies)
}

export function getTemplate(id: number) {
  return apiRequest<StrategyTemplate>(`/strategies/${id}`)
}

// Human-readable label for the policy asset class; falls back to the raw code.
const ASSET_CLASS_LABELS: Record<string, string> = {
  A_STOCK_ETF: 'A股 ETF',
  GOLD_ETF: '黄金 ETF',
}

export function assetClassLabel(code: string | undefined): string | null {
  if (!code) return null
  return ASSET_CLASS_LABELS[code] ?? code
}
