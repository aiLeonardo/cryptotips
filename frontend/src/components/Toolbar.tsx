import type { FC, ChangeEvent } from 'react'
import styles from './Toolbar.module.css'

export type PageKey = 'klines' | 'feargreed'

interface Props {
  page:             PageKey

  symbols:          string[]
  intervals:        string[]
  selectedSymbol:   string
  selectedInterval: string
  limit:            number
  loading:          boolean
  showVol:          boolean
  showAmountPanel:  boolean
  showEmaScorePanel:boolean
  showScoreLine:    boolean
  showReversalMarkers:boolean
  showRegimeMarkers:boolean
  onSymbolChange:   (v: string) => void
  onIntervalChange: (v: string) => void
  onLimitChange:    (v: number) => void
  onRefresh:        () => void
  onToggleVol:      () => void
  onToggleAmountPanel: () => void
  onToggleEmaScorePanel: () => void
  onToggleScoreLine:() => void
  onToggleReversalMarkers: () => void
  onToggleRegimeMarkers: () => void
}

// 常用 interval 排序权重（数据库可能只有部分）
const INTERVAL_ORDER: Record<string, number> = {
  '1m': 1, '3m': 2, '5m': 3, '15m': 4, '30m': 5,
  '1h': 6, '2h': 7, '4h': 8, '6h': 9, '8h': 10, '12h': 11,
  '1d': 12, '3d': 13, '1w': 14, '1M': 15,
}

const LIMIT_OPTIONS = [1000, 1500, 3000, 5000]

const Toolbar: FC<Props> = ({
  page,
  symbols, intervals,
  selectedSymbol, selectedInterval, limit, loading,
  showVol,
  showAmountPanel,
  showEmaScorePanel,
  showScoreLine,
  showReversalMarkers,
  showRegimeMarkers,
  onSymbolChange, onIntervalChange, onLimitChange, onRefresh, onToggleVol, onToggleAmountPanel, onToggleEmaScorePanel, onToggleScoreLine, onToggleReversalMarkers, onToggleRegimeMarkers,
}) => {
  const sortedIntervals = [...intervals].sort(
    (a, b) => (INTERVAL_ORDER[a] ?? 99) - (INTERVAL_ORDER[b] ?? 99),
  )

  const handleSymbol   = (e: ChangeEvent<HTMLSelectElement>) => onSymbolChange(e.target.value)
  const handleInterval = (e: ChangeEvent<HTMLSelectElement>) => onIntervalChange(e.target.value)
  const handleLimit    = (e: ChangeEvent<HTMLSelectElement>) => onLimitChange(Number(e.target.value))

  return (
    <div className={styles.toolbar}>
      {/* 品牌 */}
      <div className={styles.brand}>
        <span className={styles.logo}>📈</span>
        <span className={styles.title}>Crypto Viewer</span>
      </div>

      {/* K 线图专属控件 */}
      {page === 'klines' && (
        <div className={styles.controls}>
          <label className={styles.group}>
            <span className={styles.label}>Symbol</span>
            <select
              className={styles.select}
              value={selectedSymbol}
              onChange={handleSymbol}
              disabled={symbols.length === 0}
            >
              {symbols.length === 0 && <option value="">加载中…</option>}
              {symbols.map(s => (
                <option key={s} value={s}>{s}</option>
              ))}
            </select>
          </label>

          <label className={styles.group}>
            <span className={styles.label}>Interval</span>
            <select
              className={styles.select}
              value={selectedInterval}
              onChange={handleInterval}
              disabled={sortedIntervals.length === 0}
            >
              {sortedIntervals.length === 0 && <option value="">加载中…</option>}
              {sortedIntervals.map(i => (
                <option key={i} value={i}>{i}</option>
              ))}
            </select>
          </label>

          <label className={styles.group}>
            <span className={styles.label}>显示条数</span>
            <select className={styles.select} value={limit} onChange={handleLimit}>
              {LIMIT_OPTIONS.map(n => (
                <option key={n} value={n}>{n}</option>
              ))}
            </select>
          </label>

          <button
            className={`${styles.btnVol}${showVol ? ` ${styles.btnVolActive}` : ''}`}
            onClick={onToggleVol}
            title={showVol ? '隐藏成交量柱图' : '显示成交量柱图'}
          >
            Vol
          </button>

          <button
            className={`${styles.btnVol}${showAmountPanel ? ` ${styles.btnVolActive}` : ''}`}
            onClick={onToggleAmountPanel}
            title={showAmountPanel ? '隐藏成交额面板' : '显示成交额面板'}
          >
            Amt
          </button>

          <button
            className={`${styles.btnVol}${showEmaScorePanel ? ` ${styles.btnVolActive}` : ''}`}
            onClick={onToggleEmaScorePanel}
            title={showEmaScorePanel ? '隐藏成交额EMA分数面板' : '显示成交额EMA分数面板'}
          >
            EMA分
          </button>

          <button
            className={`${styles.btnVol}${showScoreLine ? ` ${styles.btnVolActive}` : ''}`}
            onClick={onToggleScoreLine}
            title={showScoreLine ? '隐藏Top/Bottom分数走势线' : '显示Top/Bottom分数走势线'}
          >
            Score
          </button>

          <button
            className={`${styles.btnVol}${showReversalMarkers ? ` ${styles.btnVolActive}` : ''}`}
            onClick={onToggleReversalMarkers}
            title={showReversalMarkers ? '隐藏Top/Bottom标记' : '显示Top/Bottom标记'}
          >
            TopBot
          </button>

          <button
            className={`${styles.btnVol}${showRegimeMarkers ? ` ${styles.btnVolActive}` : ''}`}
            onClick={onToggleRegimeMarkers}
            title={showRegimeMarkers ? '隐藏牛熊盘整起点标记' : '显示牛熊盘整起点标记'}
          >
            Regime
          </button>

          <button
            className={styles.btn}
            onClick={onRefresh}
            disabled={loading || !selectedSymbol || !selectedInterval}
          >
            {loading ? '加载中…' : '刷新'}
          </button>
        </div>
      )}
    </div>
  )
}

export default Toolbar
