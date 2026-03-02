/** 单根 K 线（与后端 KLineItem 对应） */
export interface KLineItem {
  time: number    // Unix 秒（TradingView 格式）
  open: number
  high: number
  low: number
  close: number
  volume: number
  quoteVolume: number
}

/** 后端 /api/klines 响应的 data 字段 */
export interface ReversalSignalItem {
  time: number
  type: 'top' | 'bottom'
  score: number
}

export interface RegimeStartpointItem {
  time: number
  state: 'BULL' | 'BEAR' | 'RANGE'
  confidence: number
}

export interface KLinesData {
  symbol: string
  interval: string
  klines: KLineItem[]
  quoteVolumeLog?: number[]
  quoteVolumeLogEma?: number[]
  quoteVolumeZ?: number[]
  reversalSignals?: ReversalSignalItem[]
  regimeStartpoints?: RegimeStartpointItem[]
}

/** 后端 /api/klines/meta 响应的 data 字段 */
export interface KLinesMeta {
  symbols: string[]
  intervals: string[]
}

/** 通用响应体 */
export interface ApiResponse<T> {
  code: number
  message: string
  data: T
}
