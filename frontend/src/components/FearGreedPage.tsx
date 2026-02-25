import { useEffect, useRef, useState, useCallback, type FC } from 'react'
import {
  createChart,
  CrosshairMode,
  type IChartApi,
  type ISeriesApi,
  type UTCTimestamp,
} from 'lightweight-charts'
import { fetchFearGreedHistory } from '../api/feargreed'
import type { FearGreedItem } from '../types/feargreed'
import styles from './FearGreedPage.module.css'

// ─── 仪表盘配置 ───────────────────────────────────────────────────────────────

const CX = 160
const CY = 158
const R_OUTER = 132
const R_INNER = 82

const ZONES: { from: number; to: number; color: string; label: string }[] = [
  { from: 178, to: 137, color: '#d94f3d', label: 'Extreme Fear' },
  { from: 133, to: 101, color: '#e07d3c', label: 'Fear' },
  { from: 97,  to: 83,  color: '#c9a227', label: 'Neutral' },
  { from: 79,  to: 47,  color: '#6aaa3a', label: 'Greed' },
  { from: 43,  to: 2,   color: '#16c784', label: 'Extreme Greed' },
]

const ZONE_LABELS: { angle: number; label: string }[] = [
  { angle: 157, label: 'Extreme\nFear' },
  { angle: 117, label: 'Fear' },
  { angle: 90,  label: 'Neutral' },
  { angle: 63,  label: 'Greed' },
  { angle: 23,  label: 'Extreme\nGreed' },
]

/** 将指数值(0-100)映射到角度(180°→0°) */
function valueToAngle(value: number) {
  return 180 - value * 1.8
}

/** 极坐标转 SVG 坐标（y 轴翻转） */
function polar(cx: number, cy: number, r: number, deg: number) {
  const rad = (deg * Math.PI) / 180
  return { x: cx + r * Math.cos(rad), y: cy - r * Math.sin(rad) }
}

/** 生成扇形弧路径（donut 型） */
function arcPath(startDeg: number, endDeg: number): string {
  const p1 = polar(CX, CY, R_OUTER, startDeg)
  const p2 = polar(CX, CY, R_OUTER, endDeg)
  const p3 = polar(CX, CY, R_INNER, endDeg)
  const p4 = polar(CX, CY, R_INNER, startDeg)
  const f = (n: number) => n.toFixed(2)
  const la = Math.abs(startDeg - endDeg) > 180 ? 1 : 0
  return [
    `M ${f(p1.x)} ${f(p1.y)}`,
    `A ${R_OUTER} ${R_OUTER} 0 ${la} 0 ${f(p2.x)} ${f(p2.y)}`,
    `L ${f(p3.x)} ${f(p3.y)}`,
    `A ${R_INNER} ${R_INNER} 0 ${la} 1 ${f(p4.x)} ${f(p4.y)}`,
    'Z',
  ].join(' ')
}

// ─── 颜色辅助 ─────────────────────────────────────────────────────────────────

function classificationColor(cls: string): string {
  const map: Record<string, string> = {
    'Extreme Fear': '#d94f3d',
    'Fear':         '#e07d3c',
    'Neutral':      '#c9a227',
    'Greed':        '#6aaa3a',
    'Extreme Greed':'#16c784',
  }
  return map[cls] ?? '#8b949e'
}

// ─── 仪表盘组件 ───────────────────────────────────────────────────────────────

const Gauge: FC<{ value: number; classification: string }> = ({ value, classification }) => {
  const angle = valueToAngle(value)
  const rad   = (angle * Math.PI) / 180
  const color = classificationColor(classification)

  // 指针（长针 + 短装饰线）
  const needleLen = R_INNER - 8
  const nx = CX + needleLen * Math.cos(rad)
  const ny = CY - needleLen * Math.sin(rad)

  // 指针底部两侧小点
  const baseR  = 6
  const perpRad = rad + Math.PI / 2
  const bx1 = CX + baseR * Math.cos(perpRad)
  const by1 = CY - baseR * Math.sin(perpRad)
  const bx2 = CX - baseR * Math.cos(perpRad)
  const by2 = CY + baseR * Math.sin(perpRad)

  return (
    <svg viewBox="0 0 320 175" className={styles.gauge}>
      {/* 底层背景弧（灰色轨道） */}
      <path
        d={arcPath(179, 1)}
        fill="#1c2128"
      />

      {/* 5个色区 */}
      {ZONES.map(z => (
        <path key={z.label} d={arcPath(z.from, z.to)} fill={z.color} opacity={0.85} />
      ))}

      {/* 区域标签（外圈） */}
      {ZONE_LABELS.map(({ angle: a, label }) => {
        const labelR = R_OUTER + 16
        const p = polar(CX, CY, labelR, a)
        const lines = label.split('\n')
        return (
          <text
            key={label}
            x={p.x}
            y={p.y}
            textAnchor="middle"
            dominantBaseline="middle"
            fontSize="8"
            fill="#6e7681"
          >
            {lines.map((l, i) => (
              <tspan key={i} x={p.x} dy={i === 0 ? (lines.length > 1 ? '-5' : '0') : '10'}>
                {l}
              </tspan>
            ))}
          </text>
        )
      })}

      {/* 指针 */}
      <polygon
        points={`${nx.toFixed(1)},${ny.toFixed(1)} ${bx1.toFixed(1)},${by1.toFixed(1)} ${bx2.toFixed(1)},${by2.toFixed(1)}`}
        fill={color}
      />
      {/* 中心圆 */}
      <circle cx={CX} cy={CY} r={10} fill="#161b22" stroke={color} strokeWidth={2} />

      {/* 数值 */}
      <text x={CX} y={CY + 26} textAnchor="middle" fontSize="28" fontWeight="700" fill={color}>
        {value}
      </text>
      <text x={CX} y={CY + 44} textAnchor="middle" fontSize="11" fill="#8b949e">
        {classification}
      </text>
    </svg>
  )
}

// ─── 历史折线图组件 ───────────────────────────────────────────────────────────

const HistoryChart: FC<{ history: FearGreedItem[] }> = ({ history }) => {
  const containerRef = useRef<HTMLDivElement>(null)
  const chartRef     = useRef<IChartApi | null>(null)
  const lineRef      = useRef<ISeriesApi<'Line'> | null>(null)

  const initChart = useCallback(() => {
    const el = containerRef.current
    if (!el || chartRef.current) return

    const chart = createChart(el, {
      layout: {
        background: { color: '#0d1117' },
        textColor:  '#9198a1',
      },
      grid: {
        vertLines: { color: '#1c2128' },
        horzLines: { color: '#1c2128' },
      },
      crosshair: { mode: CrosshairMode.Normal },
      rightPriceScale: { borderColor: '#30363d' },
      timeScale: { borderColor: '#30363d', timeVisible: true, secondsVisible: false },
      width:  el.clientWidth,
      height: el.clientHeight,
    })

    const lineSeries = chart.addLineSeries({
      color:     '#16c784',
      lineWidth: 2,
      priceFormat: { type: 'price', precision: 0, minMove: 1 },
    })

    chartRef.current = chart
    lineRef.current  = lineSeries

    const ro = new ResizeObserver(() => {
      if (containerRef.current) {
        chart.applyOptions({
          width:  containerRef.current.clientWidth,
          height: containerRef.current.clientHeight,
        })
      }
    })
    ro.observe(el)

    return () => {
      ro.disconnect()
      chart.remove()
      chartRef.current = null
      lineRef.current  = null
    }
  }, [])

  useEffect(() => {
    const cleanup = initChart()
    return cleanup
  }, [initChart])

  useEffect(() => {
    if (!lineRef.current || history.length === 0) return

    // history 是按 created_at DESC 排序，需要升序给图表
    const sorted = [...history].reverse()
    const data = sorted.map(item => ({
      time:  item.timestamp as UTCTimestamp,
      value: item.value,
    }))

    lineRef.current.setData(data)
    chartRef.current?.timeScale().fitContent()
  }, [history])

  return <div ref={containerRef} className={styles.chart} />
}

// ─── 限制选项 ─────────────────────────────────────────────────────────────────

const LIMIT_OPTIONS = [
  { label: '365天', value: 365 },
  { label: '2年', value: 730 },
  { label: '4年', value: 1460 },
  { label: '8年', value: 2920 },
  { label: '16年', value: 5840 },
]

// ─── 主页面组件 ───────────────────────────────────────────────────────────────

const FearGreedPage: FC = () => {
  const [data,    setData]    = useState<{ current: FearGreedItem | null; history: FearGreedItem[] } | null>(null)
  const [loading, setLoading] = useState(false)
  const [error,   setError]   = useState<string | null>(null)
  const [limit,   setLimit]   = useState(2920)

  const load = useCallback(() => {
    setLoading(true)
    setError(null)
    fetchFearGreedHistory(limit)
      .then(d => setData(d))
      .catch(e => setError(String(e)))
      .finally(() => setLoading(false))
  }, [limit])

  useEffect(() => { load() }, [load])

  const current = data?.current
  const color   = current ? classificationColor(current.value_classification) : '#8b949e'

  const updatedDate = current
    ? new Date(current.timestamp * 1000).toLocaleDateString('zh-CN', {
        year: 'numeric', month: 'long', day: 'numeric',
      })
    : '—'

  return (
    <div className={styles.page}>
      {/* ── 顶部：仪表盘 + 信息卡 ── */}
      <div className={styles.top}>
        {/* 仪表盘卡片 */}
        <div className={styles.card}>
          <div className={styles.cardTitle}>恐慌贪婪指数</div>
          {loading && !current && <div className={styles.placeholder}>加载中…</div>}
          {current && (
            <Gauge value={current.value} classification={current.value_classification} />
          )}
        </div>

        {/* 信息卡片 */}
        <div className={styles.card}>
          <div className={styles.cardTitle}>当前状态</div>
          {current && (
            <div className={styles.info}>
              <div className={styles.bigValue} style={{ color }}>{current.value}</div>
              <div className={styles.bigLabel} style={{ color }}>{current.value_classification}</div>
              <div className={styles.updatedAt}>数据日期：{updatedDate}</div>

              <div className={styles.divider} />

              <div className={styles.scaleTitle}>指数等级</div>
              <div className={styles.scaleList}>
                {[
                  { label: 'Extreme Fear', range: '0 – 24',   color: '#d94f3d' },
                  { label: 'Fear',         range: '25 – 44',  color: '#e07d3c' },
                  { label: 'Neutral',      range: '45 – 54',  color: '#c9a227' },
                  { label: 'Greed',        range: '55 – 74',  color: '#6aaa3a' },
                  { label: 'Extreme Greed',range: '75 – 100', color: '#16c784' },
                ].map(s => (
                  <div key={s.label} className={styles.scaleItem}>
                    <span className={styles.scaleDot} style={{ background: s.color }} />
                    <span className={styles.scaleLabel}>{s.label}</span>
                    <span className={styles.scaleRange}>{s.range}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
          {!current && !loading && !error && (
            <div className={styles.placeholder}>暂无数据</div>
          )}
        </div>
      </div>

      {/* ── 历史图表区 ── */}
      <div className={styles.chartCard}>
        <div className={styles.chartHeader}>
          <span className={styles.cardTitle}>历史趋势</span>
          <div className={styles.limitTabs}>
            {LIMIT_OPTIONS.map(opt => (
              <button
                key={opt.value}
                className={`${styles.limitTab} ${limit === opt.value ? styles.limitTabActive : ''}`}
                onClick={() => setLimit(opt.value)}
              >
                {opt.label}
              </button>
            ))}
          </div>
        </div>
        {error && (
          <div className={styles.error}>
            <span>错误：{error}</span>
            <button onClick={load}>重试</button>
          </div>
        )}
        <HistoryChart history={data?.history ?? []} />
      </div>
    </div>
  )
}

export default FearGreedPage
