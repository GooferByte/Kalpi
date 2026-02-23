'use client'

import { useState } from 'react'
import { Rocket, ArrowLeft, Loader2, Webhook } from 'lucide-react'
import { api } from '@/lib/api'
import type { ExecutionMode, ExecutionResult, OrderPayload } from '@/types'

interface Props {
  broker: string
  sessionId: string
  mode: ExecutionMode
  orders: OrderPayload
  onResult: (result: ExecutionResult) => void
  onBack: () => void
}

export default function ExecutePanel({ broker, sessionId, mode, orders, onResult, onBack }: Props) {
  const [webhookUrl, setWebhookUrl] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const totalOrders =
    orders.buy.filter((r) => r.qty && r.qty > 0).length +
    orders.sell.filter((r) => r.qty && r.qty > 0).length +
    orders.rebalance.filter((r) => r.qty_change && r.qty_change !== 0).length

  const handleExecute = async () => {
    setLoading(true)
    setError('')
    try {
      const result =
        mode === 'first_time'
          ? await api.execute(sessionId, broker, orders, webhookUrl)
          : await api.rebalance(sessionId, broker, orders, webhookUrl)
      onResult(result)
    } catch (err: any) {
      setError(err.message || 'Execution failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="card max-w-lg mx-auto space-y-6">
      <div className="flex items-center gap-3">
        <div className="w-10 h-10 bg-emerald-50 rounded-xl flex items-center justify-center">
          <Rocket className="w-5 h-5 text-emerald-600" />
        </div>
        <div>
          <h2 className="text-lg font-semibold">Ready to Execute</h2>
          <p className="text-sm text-slate-500">Review and confirm</p>
        </div>
      </div>

      {/* Summary */}
      <div className="bg-slate-50 rounded-xl p-4 space-y-3">
        <SummaryRow label="Broker" value={broker.charAt(0).toUpperCase() + broker.slice(1)} />
        <SummaryRow
          label="Mode"
          value={mode === 'first_time' ? 'First-Time Portfolio' : 'Rebalance'}
        />
        <SummaryRow label="Total Orders" value={String(totalOrders)} />

        {/* Order breakdown */}
        {orders.buy.filter((r) => r.qty && r.qty > 0).length > 0 && (
          <div className="pt-2 border-t border-slate-200">
            <p className="text-xs font-semibold text-slate-500 mb-2">BUY</p>
            {orders.buy.filter((r) => r.qty && r.qty > 0).map((r, i) => (
              <div key={i} className="flex justify-between text-sm py-0.5">
                <span className="font-mono font-medium">{r.symbol}</span>
                <span className="text-slate-500">{r.qty} shares</span>
              </div>
            ))}
          </div>
        )}

        {orders.sell.filter((r) => r.qty && r.qty > 0).length > 0 && (
          <div className="pt-2 border-t border-slate-200">
            <p className="text-xs font-semibold text-slate-500 mb-2">SELL</p>
            {orders.sell.filter((r) => r.qty && r.qty > 0).map((r, i) => (
              <div key={i} className="flex justify-between text-sm py-0.5">
                <span className="font-mono font-medium">{r.symbol}</span>
                <span className="text-slate-500">{r.qty} shares</span>
              </div>
            ))}
          </div>
        )}

        {orders.rebalance.filter((r) => r.qty_change && r.qty_change !== 0).length > 0 && (
          <div className="pt-2 border-t border-slate-200">
            <p className="text-xs font-semibold text-slate-500 mb-2">ADJUST</p>
            {orders.rebalance.filter((r) => r.qty_change && r.qty_change !== 0).map((r, i) => (
              <div key={i} className="flex justify-between text-sm py-0.5">
                <span className="font-mono font-medium">{r.symbol}</span>
                <span className={r.qty_change! > 0 ? 'text-emerald-600' : 'text-red-600'}>
                  {r.qty_change! > 0 ? '+' : ''}{r.qty_change} shares
                </span>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Webhook URL */}
      <div>
        <label className="label flex items-center gap-1.5">
          <Webhook className="w-3.5 h-3.5 text-slate-400" /> Webhook URL
          <span className="text-slate-400 font-normal">(optional)</span>
        </label>
        <input
          type="url"
          placeholder="https://webhook.site/your-id"
          value={webhookUrl}
          onChange={(e) => setWebhookUrl(e.target.value)}
          className="input"
        />
        <p className="text-xs text-slate-400 mt-1">Results will be POSTed here on completion.</p>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 text-sm rounded-xl px-4 py-3">
          {error}
        </div>
      )}

      <div className="flex gap-3">
        <button onClick={onBack} className="btn-secondary" disabled={loading}>
          <ArrowLeft className="w-4 h-4" /> Back
        </button>
        <button onClick={handleExecute} className="btn-primary flex-1 justify-center" disabled={loading}>
          {loading ? (
            <><Loader2 className="w-4 h-4 animate-spin" /> Executing…</>
          ) : (
            <><Rocket className="w-4 h-4" /> Execute {totalOrders} Order{totalOrders !== 1 ? 's' : ''}</>
          )}
        </button>
      </div>
    </div>
  )
}

function SummaryRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between text-sm">
      <span className="text-slate-500">{label}</span>
      <span className="font-semibold">{value}</span>
    </div>
  )
}
