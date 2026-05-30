import { apiRequest } from '@/lib/api'

// GeneRecord mirrors store.GeneRecord (GORM PascalCase fields).
export interface GeneRecord {
  ID: number
  CreatedAt: string
  StrategyID: number
  Role: string // challenger / champion / retired
  ParamPack: unknown
  ScoreTotal: number
  MaxDrawdown: number
}

// EvolutionTask mirrors store.EvolutionTask.
export interface EvolutionTask {
  ID: number
  CreatedAt: string
  StrategyID: number
  Status: string // pending / running / done / failed
  Progress: number
  Config: unknown
}

// ChallengerSummary mirrors api.ChallengerSummary (explicit snake_case tags).
export interface ChallengerSummary {
  id: number
  score_total: number
  max_drawdown: number
  created_at: string
  param_pack?: unknown
}

export interface ListTasksResponse {
  tasks: EvolutionTask[]
  challengers: ChallengerSummary[]
}

export function getTasks(strategyId: number) {
  return apiRequest<ListTasksResponse>(`/evolution/tasks?strategy_id=${strategyId}`)
}

export function getChampion(strategyId: number) {
  return apiRequest<GeneRecord>(`/genome/champion?strategy_id=${strategyId}`)
}

export interface CreateTaskBody {
  strategy_id: number
  symbol: string
  pop_size: number
  max_generations: number
  spawn_mode: string // "inherit" | "random_once" | "manual"
}

export function createTask(body: CreateTaskBody) {
  return apiRequest<EvolutionTask>('/evolution/tasks', { method: 'POST', body })
}

// Promotes a challenger to champion. gene_record_id=0 promotes the latest.
export function promoteChallenger(taskId: number, geneRecordId = 0) {
  return apiRequest<{ promoted_id: number }>(`/evolution/tasks/${taskId}/promote`, {
    method: 'POST',
    body: { gene_record_id: geneRecordId },
  })
}
