import { apiRequest } from '@/lib/api'

// Mirrors the JSON returned by GET /api/v1/agents/status.
export interface AgentStatus {
  user_id: number
  connected: boolean
}

// Reports whether THIS user's Agent is currently connected to the Hub.
export function getAgentStatus() {
  return apiRequest<AgentStatus>('/agents/status')
}
