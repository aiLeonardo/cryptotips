export interface FearGreedItem {
  value: number
  value_classification: string
  timestamp: number
}

export interface FearGreedHistoryData {
  current: FearGreedItem | null
  history: FearGreedItem[]
}
