'use client'

import { useState } from 'react'
import StepIndicator from '@/components/StepIndicator'
import BrokerConnect from '@/components/BrokerConnect'
import PortfolioBuilder from '@/components/PortfolioBuilder'
import ExecutePanel from '@/components/ExecutePanel'
import ResultsPanel from '@/components/ResultsPanel'
import type { ExecutionMode, ExecutionResult, OrderPayload } from '@/types'

const EMPTY_ORDERS: OrderPayload = { buy: [], sell: [], rebalance: [] }

export default function Home() {
  const [step, setStep] = useState(1)
  const [sessionId, setSessionId] = useState('')
  const [broker, setBroker] = useState('')
  const [mode, setMode] = useState<ExecutionMode>('first_time')
  const [orders, setOrders] = useState<OrderPayload>(EMPTY_ORDERS)
  const [result, setResult] = useState<ExecutionResult | null>(null)

  const reset = () => {
    setStep(1)
    setSessionId('')
    setBroker('')
    setMode('first_time')
    setOrders(EMPTY_ORDERS)
    setResult(null)
  }

  return (
    <div className="min-h-screen bg-slate-50">
      {/* Header */}
      <header className="bg-white border-b border-slate-100 sticky top-0 z-10">
        <div className="max-w-4xl mx-auto px-4 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 bg-emerald-600 rounded-lg flex items-center justify-center">
              <span className="text-white font-bold text-sm">K</span>
            </div>
            <div>
              <span className="font-bold text-slate-900">Kalpi</span>
              <span className="text-slate-400 text-sm ml-2 hidden sm:inline">Portfolio Execution Engine</span>
            </div>
          </div>

          {/* Session info pill */}
          {sessionId && (
            <div className="flex items-center gap-2 bg-emerald-50 border border-emerald-200 rounded-full px-3 py-1">
              <span className="w-2 h-2 bg-emerald-500 rounded-full animate-pulse" />
              <span className="text-xs font-medium text-emerald-700 capitalize">{broker}</span>
              <span className="text-xs text-emerald-500 font-mono hidden sm:block">
                {sessionId.slice(0, 8)}…
              </span>
            </div>
          )}
        </div>
      </header>

      {/* Main content */}
      <main className="max-w-4xl mx-auto px-4 py-10">
        <StepIndicator currentStep={step} />

        {step === 1 && (
          <BrokerConnect
            onConnect={(id, b) => {
              setSessionId(id)
              setBroker(b)
              setStep(2)
            }}
          />
        )}

        {step === 2 && (
          <PortfolioBuilder
            onNext={(m, o) => {
              setMode(m)
              setOrders(o)
              setStep(3)
            }}
            onBack={() => setStep(1)}
          />
        )}

        {step === 3 && (
          <ExecutePanel
            broker={broker}
            sessionId={sessionId}
            mode={mode}
            orders={orders}
            onResult={(r) => {
              setResult(r)
              setStep(4)
            }}
            onBack={() => setStep(2)}
          />
        )}

        {step === 4 && (
          <ResultsPanel result={result} onReset={reset} />
        )}
      </main>

      {/* Footer */}
      <footer className="text-center text-xs text-slate-400 pb-8">
        Kalpi Capital · Portfolio Execution Engine ·{' '}
        <a href="http://localhost:8080/health" target="_blank" rel="noreferrer" className="hover:text-emerald-600 transition-colors">
          API Status
        </a>
      </footer>
    </div>
  )
}
