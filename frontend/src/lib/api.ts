import type { Credentials, ExecutionMode, OrderPayload } from '@/types'

// Use empty base so all requests go through Next.js rewrite proxy (same-origin → no CORS).
// next.config.mjs rewrites /api/* → backend on the server side.
const BASE = ''

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  const json = await res.json()
  if (!json.success) throw new Error(json.error || 'Request failed')
  return json.data as T
}

export const api = {
  /** Authenticate with a broker and return { session_id, access_token, ... } */
  authenticate: (broker: string, credentials: Credentials) =>
    request<{ session_id: string; broker: string; message: string }>(
      `/api/v1/auth/${broker}`,
      { method: 'POST', body: JSON.stringify({ credentials }) }
    ),

  /** Fetch current holdings for a session */
  getHoldings: (sessionId: string) =>
    request<{ symbol: string; quantity: number; avg_price: number; current_price: number; pnl: number }[]>(
      `/api/v1/holdings?session_id=${sessionId}`
    ),

  /** Execute a first-time portfolio */
  execute: (sessionId: string, broker: string, orders: OrderPayload, webhookUrl?: string) =>
    request<import('@/types').ExecutionResult>('/api/v1/portfolio/execute', {
      method: 'POST',
      body: JSON.stringify({
        broker,
        mode: 'first_time' as ExecutionMode,
        session_id: sessionId,
        orders,
        webhook_url: webhookUrl || undefined,
      }),
    }),

  /** Execute a rebalance */
  rebalance: (sessionId: string, broker: string, orders: OrderPayload, webhookUrl?: string) =>
    request<import('@/types').ExecutionResult>('/api/v1/portfolio/rebalance', {
      method: 'POST',
      body: JSON.stringify({
        broker,
        mode: 'rebalance' as ExecutionMode,
        session_id: sessionId,
        orders,
        webhook_url: webhookUrl || undefined,
      }),
    }),

  /** Fetch a single execution result */
  getExecution: (execId: string) =>
    request<import('@/types').ExecutionResult>(`/api/v1/orders/${execId}`),

  /** Fetch all execution results */
  listExecutions: () =>
    request<import('@/types').ExecutionResult[]>('/api/v1/orders'),

  /** List supported brokers */
  listBrokers: () => request<string[]>('/api/v1/brokers'),
}
