export type OrderSide = 'BUY' | 'SELL'
export type OrderStatus = 'PENDING' | 'OPEN' | 'COMPLETE' | 'REJECTED' | 'CANCELLED' | 'FAILED'
export type ExecutionMode = 'first_time' | 'rebalance'

export interface TradeInstruction {
  symbol: string
  qty?: number
  qty_change?: number   // for rebalance: negative = sell, positive = buy
  order_type?: string
  price?: number
}

export interface OrderPayload {
  buy: TradeInstruction[]
  sell: TradeInstruction[]
  rebalance: TradeInstruction[]
}

export interface OrderResult {
  order_id: string
  symbol: string
  quantity: number
  side: OrderSide
  status: OrderStatus
  message?: string
  price?: number
  timestamp: string
}

export interface ExecutionResult {
  execution_id: string
  broker: string
  mode: ExecutionMode
  status: string
  successful_orders: OrderResult[]
  failed_orders: OrderResult[]
  total_orders: number
  success_count: number
  failure_count: number
  timestamp: string
  completed_at?: string
}

export interface Credentials {
  api_key?: string
  api_secret?: string
  request_token?: string
  client_code?: string
  password?: string
  totp?: string
  app_id?: string
  auth_code?: string
  redirect_uri?: string
}

export interface BrokerField {
  key: keyof Credentials
  label: string
  type: 'text' | 'password'
  placeholder?: string
}
