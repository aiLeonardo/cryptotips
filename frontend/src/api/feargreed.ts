import type { ApiResponse } from '../types/kline'
import type { FearGreedHistoryData } from '../types/feargreed'

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

export function fetchFearGreedHistory(limit = 3000): Promise<FearGreedHistoryData> {
  return request<FearGreedHistoryData>(`/api/feargreed/history?limit=${limit}`)
}
