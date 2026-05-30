import { apiRequest } from '@/lib/api'

// /dashboard uses explicit snake_case json tags.
export interface InstanceSummary {
  id: number
  template_id: number
  status: string
  cny_balance: number
  total_equity: number
}

export interface DashboardResponse {
  total_instances: number
  running_count: number
  total_cny: number
  total_equity: number
  instances: InstanceSummary[]
}

// /instances serializes GORM structs with default (PascalCase) field names.
export interface InstanceTemplate {
  ID: number
  Name: string
  Version: string
  IsSpot: boolean
}

export interface InstanceRow {
  ID: number
  CreatedAt: string
  TemplateID: number
  Status: string
  Template: InstanceTemplate
}

export interface TradeRow {
  ID: number
  CreatedAt: string
  Action: string
  Engine: string
  Symbol: string
  FilledQty: number
  FilledPrice: number
}

export function getDashboard() {
  return apiRequest<DashboardResponse>('/dashboard')
}

export function getInstances() {
  return apiRequest<{ instances: InstanceRow[] }>('/instances').then((r) => r.instances)
}

export function getTrades(instanceId: number) {
  return apiRequest<{ trades: TradeRow[] }>(`/instances/${instanceId}/trades?limit=1000`).then(
    (r) => r.trades,
  )
}

export function startInstance(id: number) {
  return apiRequest<{ id: number; status: string }>(`/instances/${id}/start`, { method: 'POST' })
}

export function stopInstance(id: number) {
  return apiRequest<{ id: number; status: string }>(`/instances/${id}/stop`, { method: 'POST' })
}

// Templates are read-only blueprints; /strategies serializes GORM structs (PascalCase).
export function getStrategies() {
  return apiRequest<{ strategies: InstanceTemplate[] }>('/strategies').then((r) => r.strategies)
}

export function createInstance(templateId: number) {
  return apiRequest<InstanceRow>('/instances', {
    method: 'POST',
    body: { template_id: templateId },
  })
}

export function deleteInstance(id: number) {
  return apiRequest<{ id: number; deleted: boolean }>(`/instances/${id}`, { method: 'DELETE' })
}
