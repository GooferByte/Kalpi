'use client'

import { useState } from 'react'
import { Plus, Trash2, ArrowLeft, ArrowRight, Upload } from 'lucide-react'
import type { ExecutionMode, OrderPayload, TradeInstruction } from '@/types'

interface Props {
  onNext: (mode: ExecutionMode, orders: OrderPayload) => void
  onBack: () => void
}

const emptyRow = (): TradeInstruction => ({ symbol: '', qty: 0 })
const emptyRebalanceRow = (): TradeInstruction => ({ symbol: '', qty_change: 0 })

export default function PortfolioBuilder({ onNext, onBack }: Props) {
  const [mode, setMode] = useState<ExecutionMode>('first_time')
  const [buyRows, setBuyRows] = useState<TradeInstruction[]>([emptyRow()])
  const [sellRows, setSellRows] = useState<TradeInstruction[]>([emptyRow()])
  const [rebalanceRows, setRebalanceRows] = useState<TradeInstruction[]>([emptyRebalanceRow()])
  const [error, setError] = useState('')

  // ── CSV upload ───────────────────────────────────────────────────────────
  const handleCSV = (e: React.ChangeEvent<HTMLInputElement>, type: 'buy' | 'sell') => {
    const file = e.target.files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = (ev) => {
      const lines = (ev.target?.result as string).trim().split('\n')
      const parsed: TradeInstruction[] = lines
        .filter((l) => l.trim() && !l.startsWith('#'))
        .map((l) => {
          const [symbol, qty] = l.split(',')
          return { symbol: symbol.trim().toUpperCase(), qty: parseInt(qty?.trim() || '0') }
        })
        .filter((r) => r.symbol && r.qty > 0)
      if (type === 'buy') setBuyRows(parsed.length ? parsed : [emptyRow()])
      else setSellRows(parsed.length ? parsed : [emptyRow()])
    }
    reader.readAsText(file)
    e.target.value = ''
  }

  // ── Row helpers ───────────────────────────────────────────────────────────
  const updateRow = <T extends TradeInstruction>(
    rows: T[], setRows: (r: T[]) => void, idx: number, field: keyof T, value: string
  ) => {
    const updated = [...rows]
    updated[idx] = { ...updated[idx], [field]: field === 'symbol' ? value.toUpperCase() : Number(value) }
    setRows(updated)
  }

  const addRow = <T extends TradeInstruction>(rows: T[], setRows: (r: T[]) => void, template: T) =>
    setRows([...rows, { ...template }])

  const removeRow = <T extends TradeInstruction>(rows: T[], setRows: (r: T[]) => void, idx: number) =>
    setRows(rows.filter((_, i) => i !== idx))

  // ── Submit ────────────────────────────────────────────────────────────────
  const handleNext = () => {
    setError('')
    const orders: OrderPayload = { buy: [], sell: [], rebalance: [] }

    if (mode === 'first_time') {
      orders.buy = buyRows.filter((r) => r.symbol && (r.qty || 0) > 0)
      if (!orders.buy.length) { setError('Add at least one BUY order.'); return }
    } else {
      orders.buy = buyRows.filter((r) => r.symbol && (r.qty || 0) > 0)
      orders.sell = sellRows.filter((r) => r.symbol && (r.qty || 0) > 0)
      orders.rebalance = rebalanceRows.filter((r) => r.symbol && (r.qty_change || 0) !== 0)
      if (!orders.buy.length && !orders.sell.length && !orders.rebalance.length) {
        setError('Add at least one order instruction.')
        return
      }
    }
    onNext(mode, orders)
  }

  return (
    <div className="card max-w-2xl mx-auto space-y-6">
      <div>
        <h2 className="text-lg font-semibold text-slate-900">Build Your Portfolio</h2>
        <p className="text-sm text-slate-500 mt-0.5">Define what trades to execute</p>
      </div>

      {/* Mode toggle */}
      <div className="flex gap-3">
        {(['first_time', 'rebalance'] as ExecutionMode[]).map((m) => (
          <button
            key={m}
            onClick={() => setMode(m)}
            className={`flex-1 py-2.5 rounded-xl text-sm font-semibold border-2 transition-all ${
              mode === m
                ? 'border-emerald-500 bg-emerald-50 text-emerald-700'
                : 'border-slate-200 text-slate-500 hover:border-slate-300'
            }`}
          >
            {m === 'first_time' ? '🚀 First-Time Portfolio' : '⚖️ Rebalance Existing'}
          </button>
        ))}
      </div>

      {/* Description */}
      <p className="text-xs text-slate-400 bg-slate-50 rounded-xl px-4 py-3">
        {mode === 'first_time'
          ? 'No existing holdings. All listed stocks will be purchased at market price.'
          : 'Existing portfolio. SELL → ADJUST → BUY will be executed in that order.'}
      </p>

      {/* BUY section */}
      <Section
        title={mode === 'first_time' ? 'Stocks to Buy' : 'New BUY Orders'}
        color="emerald"
        onUpload={(e) => handleCSV(e, 'buy')}
      >
        {buyRows.map((row, i) => (
          <OrderRow
            key={i}
            symbol={row.symbol}
            qty={row.qty ?? 0}
            onSymbol={(v) => updateRow(buyRows, setBuyRows, i, 'symbol', v)}
            onQty={(v) => updateRow(buyRows, setBuyRows, i, 'qty', v)}
            onRemove={buyRows.length > 1 ? () => removeRow(buyRows, setBuyRows, i) : undefined}
          />
        ))}
        <button onClick={() => addRow(buyRows, setBuyRows, emptyRow())} className="text-sm text-emerald-600 hover:text-emerald-700 flex items-center gap-1 font-medium">
          <Plus className="w-3.5 h-3.5" /> Add row
        </button>
      </Section>

      {/* SELL section (rebalance only) */}
      {mode === 'rebalance' && (
        <Section title="SELL Orders" color="red" onUpload={(e) => handleCSV(e, 'sell')}>
          {sellRows.map((row, i) => (
            <OrderRow
              key={i}
              symbol={row.symbol}
              qty={row.qty ?? 0}
              onSymbol={(v) => updateRow(sellRows, setSellRows, i, 'symbol', v)}
              onQty={(v) => updateRow(sellRows, setSellRows, i, 'qty', v)}
              onRemove={sellRows.length > 1 ? () => removeRow(sellRows, setSellRows, i) : undefined}
            />
          ))}
          <button onClick={() => addRow(sellRows, setSellRows, emptyRow())} className="text-sm text-red-600 hover:text-red-700 flex items-center gap-1 font-medium">
            <Plus className="w-3.5 h-3.5" /> Add row
          </button>
        </Section>
      )}

      {/* ADJUST section (rebalance only) */}
      {mode === 'rebalance' && (
        <Section title="ADJUST (qty_change)" color="amber" hint="Negative = sell that many units · Positive = buy more">
          {rebalanceRows.map((row, i) => (
            <AdjustRow
              key={i}
              symbol={row.symbol}
              qtyChange={row.qty_change ?? 0}
              onSymbol={(v) => updateRow(rebalanceRows, setRebalanceRows, i, 'symbol', v)}
              onQtyChange={(v) => updateRow(rebalanceRows, setRebalanceRows, i, 'qty_change', v)}
              onRemove={rebalanceRows.length > 1 ? () => removeRow(rebalanceRows, setRebalanceRows, i) : undefined}
            />
          ))}
          <button onClick={() => addRow(rebalanceRows, setRebalanceRows, emptyRebalanceRow())} className="text-sm text-amber-600 hover:text-amber-700 flex items-center gap-1 font-medium">
            <Plus className="w-3.5 h-3.5" /> Add row
          </button>
        </Section>
      )}

      {error && <p className="text-sm text-red-600 bg-red-50 rounded-xl px-4 py-3">{error}</p>}

      <div className="flex gap-3 pt-2">
        <button onClick={onBack} className="btn-secondary">
          <ArrowLeft className="w-4 h-4" /> Back
        </button>
        <button onClick={handleNext} className="btn-primary flex-1 justify-center">
          Continue <ArrowRight className="w-4 h-4" />
        </button>
      </div>
    </div>
  )
}

// ── Sub-components ────────────────────────────────────────────────────────────

function Section({ title, color, hint, onUpload, children }: {
  title: string; color: string; hint?: string; onUpload?: (e: React.ChangeEvent<HTMLInputElement>) => void; children: React.ReactNode
}) {
  const colors: Record<string, string> = {
    emerald: 'text-emerald-700 bg-emerald-50 border-emerald-200',
    red: 'text-red-700 bg-red-50 border-red-200',
    amber: 'text-amber-700 bg-amber-50 border-amber-200',
  }
  return (
    <div className={`rounded-xl border p-4 space-y-3 ${colors[color]}`}>
      <div className="flex items-center justify-between">
        <span className="text-sm font-semibold">{title}</span>
        {onUpload && (
          <label className="cursor-pointer text-xs flex items-center gap-1 opacity-70 hover:opacity-100">
            <Upload className="w-3.5 h-3.5" /> Upload CSV
            <input type="file" accept=".csv,.txt" onChange={onUpload} className="hidden" />
          </label>
        )}
      </div>
      {hint && <p className="text-xs opacity-60">{hint}</p>}
      {children}
    </div>
  )
}

function OrderRow({ symbol, qty, onSymbol, onQty, onRemove }: {
  symbol: string; qty: number; onSymbol: (v: string) => void; onQty: (v: string) => void; onRemove?: () => void
}) {
  return (
    <div className="flex gap-2 items-center">
      <input value={symbol} onChange={(e) => onSymbol(e.target.value)} placeholder="SYMBOL" className="input flex-1 uppercase text-sm bg-white" />
      <input value={qty || ''} onChange={(e) => onQty(e.target.value)} placeholder="Qty" type="number" min={1} className="input w-24 text-sm bg-white" />
      {onRemove && <button onClick={onRemove} className="text-slate-400 hover:text-red-500 transition-colors"><Trash2 className="w-4 h-4" /></button>}
    </div>
  )
}

function AdjustRow({ symbol, qtyChange, onSymbol, onQtyChange, onRemove }: {
  symbol: string; qtyChange: number; onSymbol: (v: string) => void; onQtyChange: (v: string) => void; onRemove?: () => void
}) {
  return (
    <div className="flex gap-2 items-center">
      <input value={symbol} onChange={(e) => onSymbol(e.target.value)} placeholder="SYMBOL" className="input flex-1 uppercase text-sm bg-white" />
      <input value={qtyChange || ''} onChange={(e) => onQtyChange(e.target.value)} placeholder="e.g. -3 or +5" type="number" className="input w-28 text-sm bg-white" />
      {onRemove && <button onClick={onRemove} className="text-slate-400 hover:text-red-500 transition-colors"><Trash2 className="w-4 h-4" /></button>}
    </div>
  )
}
