'use client'

import { useState } from 'react'
import { Plug, ChevronDown, Loader2 } from 'lucide-react'
import { api } from '@/lib/api'
import type { BrokerField, Credentials } from '@/types'

const BROKERS: Record<string, { label: string; fields: BrokerField[] }> = {
  mock: {
    label: 'Mock (Testing — no real credentials)',
    fields: [{ key: 'api_key', label: 'API Key', type: 'text', placeholder: 'any string e.g. test' }],
  },
  zerodha: {
    label: 'Zerodha (Kite Connect)',
    fields: [
      { key: 'api_key', label: 'API Key', type: 'text' },
      { key: 'api_secret', label: 'API Secret', type: 'password' },
      { key: 'request_token', label: 'Request Token', type: 'text' },
    ],
  },
  fyers: {
    label: 'Fyers',
    fields: [
      { key: 'app_id', label: 'App ID', type: 'text' },
      { key: 'api_secret', label: 'App Secret', type: 'password' },
      { key: 'auth_code', label: 'Auth Code', type: 'text' },
    ],
  },
  angelone: {
    label: 'AngelOne (SmartAPI)',
    fields: [
      { key: 'api_key', label: 'API Key', type: 'text' },
      { key: 'client_code', label: 'Client Code', type: 'text' },
      { key: 'password', label: 'Password', type: 'password' },
      { key: 'totp', label: 'TOTP (6-digit)', type: 'text', placeholder: '123456' },
    ],
  },
  upstox: {
    label: 'Upstox',
    fields: [
      { key: 'api_key', label: 'Client ID', type: 'text' },
      { key: 'api_secret', label: 'Client Secret', type: 'password' },
      { key: 'auth_code', label: 'Auth Code', type: 'text' },
      { key: 'redirect_uri', label: 'Redirect URI', type: 'text', placeholder: 'https://your-app.com/callback' },
    ],
  },
  groww: {
    label: 'Groww',
    fields: [{ key: 'api_key', label: 'API Key', type: 'text' }],
  },
}

interface Props {
  onConnect: (sessionId: string, broker: string) => void
}

export default function BrokerConnect({ onConnect }: Props) {
  const [broker, setBroker] = useState('mock')
  const [creds, setCreds] = useState<Credentials>({ api_key: '' })
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const currentBroker = BROKERS[broker]

  const handleBrokerChange = (b: string) => {
    setBroker(b)
    setCreds({})
    setError('')
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError('')
    try {
      const res = await api.authenticate(broker, creds)
      onConnect(res.session_id, broker)
    } catch (err: any) {
      setError(err.message || 'Authentication failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="card max-w-lg mx-auto">
      <div className="flex items-center gap-3 mb-6">
        <div className="w-10 h-10 bg-emerald-50 rounded-xl flex items-center justify-center">
          <Plug className="w-5 h-5 text-emerald-600" />
        </div>
        <div>
          <h2 className="text-lg font-semibold text-slate-900">Connect Your Broker</h2>
          <p className="text-sm text-slate-500">Authenticate to start trading</p>
        </div>
      </div>

      <form onSubmit={handleSubmit} className="space-y-4">
        {/* Broker selector */}
        <div>
          <label className="label">Select Broker</label>
          <div className="relative">
            <select
              value={broker}
              onChange={(e) => handleBrokerChange(e.target.value)}
              className="input appearance-none pr-10"
            >
              {Object.entries(BROKERS).map(([key, { label }]) => (
                <option key={key} value={key}>{label}</option>
              ))}
            </select>
            <ChevronDown className="absolute right-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400 pointer-events-none" />
          </div>
        </div>

        {/* Dynamic credential fields */}
        {currentBroker.fields.map((field) => (
          <div key={field.key}>
            <label className="label">{field.label}</label>
            <input
              type={field.type}
              placeholder={field.placeholder}
              value={(creds[field.key] as string) || ''}
              onChange={(e) => setCreds((prev) => ({ ...prev, [field.key]: e.target.value }))}
              className="input"
              required
            />
          </div>
        ))}

        {error && (
          <div className="bg-red-50 border border-red-200 text-red-700 text-sm rounded-xl px-4 py-3">
            {error}
          </div>
        )}

        <button type="submit" className="btn-primary w-full justify-center" disabled={loading}>
          {loading ? (
            <><Loader2 className="w-4 h-4 animate-spin" /> Connecting…</>
          ) : (
            <><Plug className="w-4 h-4" /> Connect to {currentBroker.label.split(' ')[0]}</>
          )}
        </button>

        {broker === 'mock' && (
          <p className="text-xs text-center text-slate-400">
            Mock broker simulates real trades — safe to test with any API key string.
          </p>
        )}
      </form>
    </div>
  )
}
