import type { ApiResponse, KLinesData, KLinesMeta } from '../types/kline'

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? ''

async function request<T>(url: string): Promise<T> {
  const resp = await fetch(BASE_URL + url)
  if (!resp.ok) {
    throw new Error(`HTTP ${resp.status}: ${resp.statusText}`)
  }
  const json: ApiResponse<T> = await resp.json()
  if (json.code !== 0) {
    throw new Error(json.message)
  }
  return json.data
}

export interface FetchKLinesParams {
  symbol: string
  interval: string
  /** Unix 毫秒，可选 */
  start?: number
  /** Unix 毫秒，可选 */
  end?: number
  /** 最多返回条数，默认 3000 */
  limit?: number
}

/** 获取 K 线数据 */
export function fetchKLines(params: FetchKLinesParams): Promise<KLinesData> {
  const q = new URLSearchParams({
    symbol: params.symbol,
    interval: params.interval,
  })
  if (params.start != null)  q.set('start',  String(params.start))
  if (params.end != null)    q.set('end',    String(params.end))
  if (params.limit != null)  q.set('limit',  String(params.limit))
  return request<KLinesData>(`/api/klines?${q}`)
}

/** 获取数据库中可用的 symbol / interval 列表 */
export function fetchKLinesMeta(): Promise<KLinesMeta> {
  return request<KLinesMeta>('/api/klines/meta')
}
