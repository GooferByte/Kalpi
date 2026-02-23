'use client'

import { useEffect, useState } from 'react'
import { CheckCircle2, XCircle, RefreshCw, Clock, AlertCircle, Wifi } from 'lucide-react'
import type { ExecutionResult, OrderResult, OrderStatus } from '@/types'

interface Props {
  result: ExecutionResult | null
  onReset: () => void
}

const WS_URL = (process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8080') + '/ws/notifications'

const STATUS_CONFIG: Record<OrderStatus, { label: string; className: string }> = {
  COMPLETE:  { label: 'Complete',  className: 'badge-success' },
  PENDING:   { label: 'Pending',   className: 'badge-pending' },
  OPEN:      { label: 'Open',      className: 'badge-pending' },
  REJECTED:  { label: 'Rejected',  className: 'badge-failed' },
  CANCELLED: { label: 'Cancelled', className: 'badge-failed' },
  FAILED:    { label: 'Failed',    className: 'badge-failed' },
}

export default function ResultsPanel({ result, onReset }: Props) {
  const [wsStatus, setWsStatus] = useState<'connecting' | 'connected' | 'disconnected'>('connecting')
  const [liveUpdates, setLiveUpdates] = useState<ExecutionResult[]>([])

  // WebSocket — connect and listen for live execution pushes
  useEffect(() => {
    let ws: WebSocket
    try {
      ws = new WebSocket(WS_URL)
      ws.onopen = () => setWsStatus('connected')
      ws.onmessage = (e) => {
        try {
          const data: ExecutionResult = JSON.parse(e.data)
          setLiveUpdates((prev) => [data, ...prev].slice(0, 5))
        } catch {}
      }
      ws.onclose = () => setWsStatus('disconnected')
      ws.onerror = () => setWsStatus('disconnected')
    } catch {
      setWsStatus('disconnected')
    }
    return () => ws?.close()
  }, [])

  if (!result) return null

  const allSuccess = result.failure_count === 0
  const allFailed = result.success_count === 0

  return (
    <div className="max-w-2xl mx-auto space-y-5">
      {/* Result header card */}
      <div className="card">
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-3">
            {allFailed ? (
              <XCircle className="w-10 h-10 text-red-500" />
            ) : allSuccess ? (
              <CheckCircle2 className="w-10 h-10 text-emerald-500" />
            ) : (
              <AlertCircle className="w-10 h-10 text-amber-500" />
            )}
            <div>
              <h2 className="text-lg font-semibold">
                {allFailed ? 'Execution Failed' : allSuccess ? 'Execution Complete' : 'Partially Complete'}
              </h2>
              <p className="text-sm text-slate-500">
                {result.broker} · {result.mode === 'first_time' ? 'First-Time' : 'Rebalance'} ·{' '}
                {new Date(result.timestamp).toLocaleTimeString()}
              </p>
            </div>
          </div>
          <button onClick={onReset} className="btn-secondary text-sm">
            <RefreshCw className="w-4 h-4" /> New Execution
          </button>
        </div>

        {/* Stats */}
        <div className="grid grid-cols-3 gap-3 mt-5">
          <StatCard label="Total" value={result.total_orders} color="slate" />
          <StatCard label="Success" value={result.success_count} color="emerald" />
          <StatCard label="Failed" value={result.failure_count} color="red" />
        </div>
      </div>

      {/* Successful orders */}
      {result.successful_orders?.length > 0 && (
        <OrderTable title="Successful Orders" orders={result.successful_orders} />
      )}

      {/* Failed orders */}
      {result.failed_orders?.length > 0 && (
        <OrderTable title="Failed Orders" orders={result.failed_orders} failed />
      )}

      {/* WebSocket live feed */}
      <div className="card">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-semibold text-sm flex items-center gap-2">
            <Wifi className="w-4 h-4 text-slate-400" /> Live Notifications
          </h3>
          <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${
            wsStatus === 'connected'
              ? 'bg-emerald-50 text-emerald-700'
              : wsStatus === 'connecting'
              ? 'bg-amber-50 text-amber-700'
              : 'bg-slate-100 text-slate-500'
          }`}>
            {wsStatus}
          </span>
        </div>
        {liveUpdates.length === 0 ? (
          <p className="text-sm text-slate-400 text-center py-4">
            {wsStatus === 'connected'
              ? 'Waiting for new executions…'
              : 'WebSocket not connected. Is the backend running?'}
          </p>
        ) : (
          <div className="space-y-2">
            {liveUpdates.map((u) => (
              <div key={u.execution_id} className="flex items-center justify-between text-xs bg-slate-50 rounded-lg px-3 py-2">
                <span className="font-mono text-slate-600">{u.execution_id.slice(0, 8)}…</span>
                <span>{u.broker} · {u.mode}</span>
                <span className="font-semibold text-emerald-600">{u.success_count} ok / {u.failure_count} fail</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

// ── Sub-components ────────────────────────────────────────────────────────────

function StatCard({ label, value, color }: { label: string; value: number; color: string }) {
  const colorMap: Record<string, string> = {
    slate: 'text-slate-700 bg-slate-50',
    emerald: 'text-emerald-700 bg-emerald-50',
    red: 'text-red-700 bg-red-50',
  }
  return (
    <div className={`rounded-xl p-3 text-center ${colorMap[color]}`}>
      <div className="text-2xl font-bold">{value}</div>
      <div className="text-xs font-medium mt-0.5">{label}</div>
    </div>
  )
}

function OrderTable({ title, orders, failed = false }: { title: string; orders: OrderResult[]; failed?: boolean }) {
  return (
    <div className="card">
      <h3 className={`text-sm font-semibold mb-4 flex items-center gap-2 ${failed ? 'text-red-700' : 'text-emerald-700'}`}>
        {failed ? <XCircle className="w-4 h-4" /> : <CheckCircle2 className="w-4 h-4" />}
        {title}
      </h3>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-xs text-slate-400 border-b border-slate-100">
              <th className="pb-2 pr-4 font-medium">Symbol</th>
              <th className="pb-2 pr-4 font-medium">Side</th>
              <th className="pb-2 pr-4 font-medium">Qty</th>
              <th className="pb-2 pr-4 font-medium">Status</th>
              <th className="pb-2 font-medium">Order ID</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-50">
            {orders.map((order, i) => {
              const statusCfg = STATUS_CONFIG[order.status] ?? { label: order.status, className: 'badge-pending' }
              return (
                <tr key={i} className="hover:bg-slate-50 transition-colors">
                  <td className="py-2.5 pr-4 font-mono font-semibold">{order.symbol}</td>
                  <td className="py-2.5 pr-4">
                    <span className={order.side === 'BUY' ? 'badge-buy' : 'badge-sell'}>{order.side}</span>
                  </td>
                  <td className="py-2.5 pr-4 tabular-nums">{order.quantity}</td>
                  <td className="py-2.5 pr-4">
                    <span className={statusCfg.className}>{statusCfg.label}</span>
                  </td>
                  <td className="py-2.5 font-mono text-xs text-slate-400">
                    {order.order_id || '—'}
                    {order.message && (
                      <p className="text-red-500 text-xs mt-0.5 font-sans">{order.message}</p>
                    )}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}
