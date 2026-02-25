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
  onSymbolChange:   (v: string) => void
  onIntervalChange: (v: string) => void
  onLimitChange:    (v: number) => void
  onRefresh:        () => void
  onToggleVol:      () => void
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
  onSymbolChange, onIntervalChange, onLimitChange, onRefresh, onToggleVol,
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
