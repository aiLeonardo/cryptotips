import { useState, useEffect, useCallback } from 'react'
import { fetchKLines, fetchKLinesMeta } from './api/klines'
import { fetchFearGreedHistory } from './api/feargreed'
import type { KLineItem, ReversalSignalItem } from './types/kline'
import type { FearGreedItem } from './types/feargreed'
import Toolbar, { type PageKey } from './components/Toolbar'
import Sidebar from './components/Sidebar'
import KLineChart from './components/KLineChart'
import FearGreedPage from './components/FearGreedPage'
import styles from './App.module.css'

const DEFAULT_LIMIT = 3000

export default function App() {
  const [page, setPage] = useState<PageKey>('klines')

  const [symbols,   setSymbols]   = useState<string[]>([])
  const [intervals, setIntervals] = useState<string[]>([])

  const [symbol,   setSymbol]   = useState('')
  const [interval, setInterval] = useState('')
  const [limit,    setLimit]    = useState(DEFAULT_LIMIT)

  const [klines,  setKlines]  = useState<KLineItem[]>([])
  const [quoteVolumeLogEma, setQuoteVolumeLogEma] = useState<number[]>([])
  const [quoteVolumeZ, setQuoteVolumeZ] = useState<number[]>([])
  const [reversalSignals, setReversalSignals] = useState<ReversalSignalItem[]>([])
  const [fearGreedHistory, setFearGreedHistory] = useState<FearGreedItem[]>([])
  const [loading, setLoading] = useState(false)
  const [error,   setError]   = useState<string | null>(null)
  const [showVol, setShowVol] = useState(false)   // 默认隐藏成交量柱图
  const [showAmountPanel, setShowAmountPanel] = useState(false)
  const [showEmaScorePanel, setShowEmaScorePanel] = useState(true)
  const [showScoreLine, setShowScoreLine] = useState(true)

  /** 加载 meta（symbol + interval 列表） */
  useEffect(() => {
    fetchKLinesMeta()
      .then(meta => {
        const syms = [...meta.symbols].sort()
        const ints = [...meta.intervals]
        setSymbols(syms)
        setIntervals(ints)
        if (syms.length > 0) setSymbol(syms[0])
        if (ints.length > 0) setInterval(ints[0])
      })
      .catch(err => setError(String(err)))
  }, [])

  /** 加载 K 线数据 */
  const loadKLines = useCallback(() => {
    if (!symbol || !interval) return
    setLoading(true)
    setError(null)
    fetchKLines({ symbol, interval, limit })
      .then(data => {
        setKlines(data.klines)
        setQuoteVolumeLogEma(data.quoteVolumeLogEma ?? [])
        setQuoteVolumeZ(data.quoteVolumeZ ?? [])
        setReversalSignals(data.reversalSignals ?? [])
      })
      .catch(err => setError(String(err)))
      .finally(() => setLoading(false))
  }, [symbol, interval, limit])

  /** symbol / interval 变化时自动刷新 */
  useEffect(() => {
    if (page === 'klines') loadKLines()
  }, [loadKLines, page])

  /** 日线时加载贪婪指数历史（仅日线展示子面板） */
  useEffect(() => {
    if (page !== 'klines' || interval !== '1d') {
      setFearGreedHistory([])
      return
    }
    fetchFearGreedHistory(limit)
      .then(d => setFearGreedHistory(d.history ?? []))
      .catch(() => setFearGreedHistory([]))
  }, [page, interval, limit])

  return (
    <div className={styles.layout}>
      {/* 顶部工具栏：品牌 + 页面专属控件 */}
      <Toolbar
        page={page}
        symbols={symbols}
        intervals={intervals}
        selectedSymbol={symbol}
        selectedInterval={interval}
        limit={limit}
        loading={loading}
        showVol={showVol}
        showAmountPanel={showAmountPanel}
        showEmaScorePanel={showEmaScorePanel}
        showScoreLine={showScoreLine}
        onSymbolChange={v => setSymbol(v)}
        onIntervalChange={v => setInterval(v)}
        onLimitChange={v => setLimit(v)}
        onRefresh={loadKLines}
        onToggleVol={() => setShowVol(v => !v)}
        onToggleAmountPanel={() => setShowAmountPanel(v => !v)}
        onToggleEmaScorePanel={() => setShowEmaScorePanel(v => !v)}
        onToggleScoreLine={() => setShowScoreLine(v => !v)}
      />

      {/* 下方主体：左侧菜单 + 右侧内容 */}
      <div className={styles.body}>
        <Sidebar page={page} onPageChange={setPage} />

        <main className={styles.main}>
          {page === 'klines' && (
            <>
              {error && (
                <div className={styles.error}>
                  <span>错误：{error}</span>
                  <button onClick={loadKLines}>重试</button>
                </div>
              )}
              {loading && klines.length === 0 && (
                <div className={styles.loading}>加载中…</div>
              )}
              <KLineChart
                klines={klines}
                symbol={symbol}
                interval={interval}
                showVol={showVol}
                showAmountPanel={showAmountPanel}
                showEmaScorePanel={showEmaScorePanel}
                showScoreLine={showScoreLine}
                quoteVolumeLogEma={quoteVolumeLogEma}
                quoteVolumeZ={quoteVolumeZ}
                reversalSignals={reversalSignals}
                showFearGreedPanel={interval === '1d'}
                fearGreedHistory={fearGreedHistory}
              />
            </>
          )}

          {page === 'feargreed' && <FearGreedPage />}
        </main>
      </div>
    </div>
  )
}
