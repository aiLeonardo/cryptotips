import {
  useEffect,
  useRef,
  useCallback,
  useState,
  type FC,
  type PointerEvent as RPointerEvent,
} from 'react'
import {
  createChart,
  CrosshairMode,
  type IChartApi,
  type ISeriesApi,
  type UTCTimestamp,
} from 'lightweight-charts'
import type { KLineItem, ReversalSignalItem, RegimeStartpointItem } from '../types/kline'
import type { FearGreedItem } from '../types/feargreed'
import styles from './KLineChart.module.css'

interface Props {
  klines: KLineItem[]
  symbol: string
  interval: string
  showVol: boolean
  showAmountPanel?: boolean
  showEmaScorePanel?: boolean
  showScoreLine?: boolean
  showReversalMarkers?: boolean
  showRegimeMarkers?: boolean
  showFearGreedPanel?: boolean
  quoteVolumeLogEma?: number[]
  quoteVolumeZ?: number[]
  reversalSignals?: ReversalSignalItem[]
  regimeStartpoints?: RegimeStartpointItem[]
  fearGreedHistory?: FearGreedItem[]
}

interface Tip {
  time: string
  open: number; high: number; low: number; close: number
  change: number; changePct: number
  vol: number; qvol: number
  up: boolean
}

// ── data adapters ──────────────────────────────────────────────────────────────

function toCandle(k: KLineItem) {
  return { time: k.time as UTCTimestamp, open: k.open, high: k.high, low: k.low, close: k.close }
}
function toVol(k: KLineItem, i: number, p?: KLineItem) {
  const up = i === 0 ? true : k.close >= (p?.close ?? k.open)
  return { time: k.time as UTCTimestamp, value: k.volume, color: up ? 'rgba(38,166,154,.65)' : 'rgba(239,83,80,.65)' }
}
function toQVol(k: KLineItem, i: number, p?: KLineItem) {
  const up = i === 0 ? true : k.close >= (p?.close ?? k.open)
  return { time: k.time as UTCTimestamp, value: k.quoteVolume, color: up ? 'rgba(38,166,154,.55)' : 'rgba(239,83,80,.55)' }
}

// ── format helpers ─────────────────────────────────────────────────────────────

function fmtTime(ts: number) {
  const d = new Date(ts * 1000)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}
function dayKey(ts: number) {
  const d = new Date(ts * 1000)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getUTCFullYear()}-${p(d.getUTCMonth() + 1)}-${p(d.getUTCDate())}`
}
function fmtAmt(v: number) {
  if (v >= 1e9) return (v / 1e9).toFixed(2) + 'B'
  if (v >= 1e6) return (v / 1e6).toFixed(2) + 'M'
  if (v >= 1e3) return (v / 1e3).toFixed(2) + 'K'
  return v.toFixed(2)
}

// ── constants ──────────────────────────────────────────────────────────────────

const DIVIDER_H = 5    // px，分隔条高度
const MIN_PX    = 60   // px，面板最小高度

const CHART_BASE = {
  layout:          { background: { color: '#0d1117' }, textColor: '#9198a1' },
  grid:            { vertLines: { color: '#1c2128' }, horzLines: { color: '#1c2128' } },
  crosshair:       { mode: CrosshairMode.Normal },
  // 统一左右价格轴宽度，避免不同面板绘图区宽度差异造成 x 轴视觉错位
  leftPriceScale:  { borderColor: '#30363d', minimumWidth: 56, visible: true },
  rightPriceScale: { borderColor: '#30363d', minimumWidth: 80, visible: true },
  handleScale:     { axisPressedMouseMove: { time: true, price: false } },
}

// ── component ──────────────────────────────────────────────────────────────────

const KLineChart: FC<Props> = ({
  klines,
  symbol,
  interval,
  showVol,
  showAmountPanel = true,
  showEmaScorePanel = true,
  showScoreLine = true,
  showReversalMarkers = false,
  showRegimeMarkers = true,
  showFearGreedPanel = false,
  quoteVolumeLogEma = [],
  quoteVolumeZ = [],
  reversalSignals = [],
  regimeStartpoints = [],
  fearGreedHistory = [],
}) => {
  // flex-grow 比例：蜡烛图 / 成交量 / 成交额
  const [ratios, setRatios] = useState([5, 2, 2, 2, 2])
  const [tip,    setTip]    = useState<Tip | null>(null)

  // DOM refs
  const areaRef  = useRef<HTMLDivElement>(null)
  const pane0Ref = useRef<HTMLDivElement>(null)   // 蜡烛图容器
  const pane1Ref = useRef<HTMLDivElement>(null)   // 成交量容器
  const pane2Ref = useRef<HTMLDivElement>(null)   // 成交额容器
  const pane3Ref = useRef<HTMLDivElement>(null)   // 信号容器（EMA/Z/Score）
  const pane4Ref = useRef<HTMLDivElement>(null)   // 贪婪指数容器（日线）

  // chart / series refs
  const charts = useRef<IChartApi[]>([])
  const sers   = useRef<ISeriesApi<any>[]>([])
  const klMap  = useRef<Map<number, KLineItem>>(new Map())
  const zMap   = useRef<Map<number, number>>(new Map())
  const emaMap = useRef<Map<number, number>>(new Map())
  const scoreMap = useRef<Map<number, number>>(new Map())
  const fgMap = useRef<Map<number, number>>(new Map())
  const fgDayMap = useRef<Map<string, number>>(new Map())
  const showFearGreedPanelRef = useRef(showFearGreedPanel)

  // ── 初始化三个独立图表 ────────────────────────────────────────────────────────

  const initCharts = useCallback(() => {
    const el0 = pane0Ref.current
    const el1 = pane1Ref.current
    const el2 = pane2Ref.current
    const el3 = pane3Ref.current
    const el4 = pane4Ref.current
    if (!el0 || !el1 || !el2 || !el3 || !el4 || charts.current.length) return

    const mk = (el: HTMLDivElement, showTime: boolean): IChartApi =>
      createChart(el, {
        ...CHART_BASE,
        timeScale: {
          borderColor: '#30363d',
          timeVisible: true,
          secondsVisible: false,
          visible: showTime,
        },
        width:  el.clientWidth,
        height: Math.max(el.clientHeight, 1),
      })

    const cc = mk(el0, false)   // 蜡烛图，不显示时间轴
    const vc = mk(el1, false)   // 成交量，不显示时间轴
    const qc = mk(el2, false)   // 成交额，不显示时间轴
    const sc = mk(el3, true)    // 信号，默认显示时间轴
    const fc = mk(el4, false)   // 贪婪指数（日线专用）

    const cs = cc.addCandlestickSeries({
      upColor:        '#26a69a', downColor:        '#ef5350',
      borderUpColor:  '#26a69a', borderDownColor:  '#ef5350',
      wickUpColor:    '#26a69a', wickDownColor:    '#ef5350',
    })
    const vs = vc.addHistogramSeries({
      priceFormat: { type: 'volume', precision: 0, minMove: 1 },
    })
    const qs = qc.addHistogramSeries({
      priceFormat: { type: 'volume', precision: 0, minMove: 1 },
      priceScaleId: 'right',
    })
    const qEma = sc.addLineSeries({
      color: '#f0b429',
      lineWidth: 2,
      priceLineVisible: false,
      lastValueVisible: false,
      priceScaleId: 'left',
    })
    const zLine = sc.addLineSeries({
      color: '#58a6ff',
      lineWidth: 1,
      lineStyle: 2,
      priceLineVisible: false,
      lastValueVisible: false,
      priceScaleId: 'right',
    })
    const scoreLine = sc.addLineSeries({
      color: '#ff7b72',
      lineWidth: 2,
      priceLineVisible: false,
      lastValueVisible: false,
      crosshairMarkerVisible: true,
      visible: showScoreLine,
      priceScaleId: 'right',
    })
    const fgLine = fc.addLineSeries({
      color: '#16c784',
      lineWidth: 2,
      priceFormat: { type: 'price', precision: 0, minMove: 1 },
      priceLineVisible: false,
      lastValueVisible: false,
    })

    // 关键：所有子图保留一致的左右价格轴占位，避免绘图区宽度不同导致 X 轴错位
    qc.applyOptions({
      leftPriceScale: {
        visible: true,
        borderColor: '#30363d',
        minimumWidth: 56,
        scaleMargins: { top: 0.1, bottom: 0.1 },
      },
      rightPriceScale: {
        visible: true,
        borderColor: '#30363d',
        minimumWidth: 80,
        scaleMargins: { top: 0.1, bottom: 0.1 },
      },
    })
    sc.applyOptions({
      leftPriceScale: {
        visible: true,
        borderColor: '#30363d',
        minimumWidth: 56,
        scaleMargins: { top: 0.1, bottom: 0.1 },
      },
      rightPriceScale: {
        visible: true,
        borderColor: '#30363d',
        minimumWidth: 80,
        scaleMargins: { top: 0.1, bottom: 0.1 },
      },
    })
    fc.applyOptions({
      leftPriceScale: { visible: true, borderColor: '#30363d', minimumWidth: 56 },
      rightPriceScale: { visible: true, borderColor: '#30363d', minimumWidth: 80 },
    })

    charts.current = [cc, vc, qc, sc, fc]
    sers.current   = [cs, vs, qs, qEma, zLine, scoreLine, fgLine]

    // ── 时间轴同步 ────────────────────────────────────────────────────────────
    let timeLock = false
    ;[cc, vc, qc, sc, fc].forEach((src, si) => {
      const rest = [cc, vc, qc, sc, fc].filter((_, i) => i !== si)
      src.timeScale().subscribeVisibleLogicalRangeChange(r => {
        if (timeLock || !r) return
        timeLock = true
        rest.forEach(o => o.timeScale().setVisibleLogicalRange(r))
        timeLock = false
      })
    })

    // ── 十字线位置同步（只负责同步位置，不操作 horzLine 可见性） ─────────────────
    const serVal = (s: ISeriesApi<any>, ts: number, k: KLineItem): number | null => {
      if (s === cs) return k.close
      if (s === vs) return k.volume
      if (s === qs) return k.quoteVolume
      if (s === qEma) return emaMap.current.get(ts) ?? null
      if (s === zLine) return zMap.current.get(ts) ?? null
      if (s === scoreLine) return scoreMap.current.get(ts) ?? null
      if (s === fgLine) {
        if (!showFearGreedPanelRef.current) return null
        const exact = fgMap.current.get(ts)
        if (exact != null) return exact
        const daily = fgDayMap.current.get(dayKey(ts))
        return daily ?? null
      }
      return null
    }

    ;[cc, vc, qc, sc, fc].forEach((src, si) => {
      const primarySeries = [cs, vs, qs, zLine, fgLine]
      const others = [cc, vc, qc, sc, fc]
        .map((c, i) => ({ c, s: primarySeries[i] }))
        .filter((_, i) => i !== si)

      src.subscribeCrosshairMove(param => {
        if (!param.point || !param.time) {
          others.forEach(o => o.c.clearCrosshairPosition())
          setTip(null)
          return
        }

        const ts = param.time as number
        const k  = klMap.current.get(ts)
        if (!k) return

        setTip({
          time: fmtTime(ts),
          open: k.open, high: k.high, low: k.low, close: k.close,
          change:    k.close - k.open,
          changePct: ((k.close - k.open) / k.open) * 100,
          vol: k.volume, qvol: k.quoteVolume,
          up: k.close >= k.open,
        })

        others.forEach(o => {
          const v = serVal(o.s, ts, k)
          if (v == null) {
            o.c.clearCrosshairPosition()
            return
          }
          o.c.setCrosshairPosition(v, param.time as UTCTimestamp, o.s)
        })
      })
    })

    // ── horzLine 隔离：用 DOM pointerenter/pointerleave 控制，与 crosshairMove 完全解耦 ──
    // 原因：setCrosshairPosition 会异步触发目标图表的 subscribeCrosshairMove，
    // 导致 lock 方案失效；DOM 事件基于真实鼠标位置，完全可靠。
    const allCharts = [cc, vc, qc, sc, fc]
    const enterHandlers: EventListener[] = []
    const leaveHandlers: EventListener[] = []

    ;[el0, el1, el2, el3, el4].forEach((el, i) => {
      const onEnter: EventListener = () => {
        allCharts.forEach((c, j) =>
          c.applyOptions({ crosshair: { horzLine: { visible: j === i } } })
        )
      }
      const onLeave: EventListener = () => {
        allCharts.forEach(c =>
          c.applyOptions({ crosshair: { horzLine: { visible: true } } })
        )
      }
      enterHandlers.push(onEnter)
      leaveHandlers.push(onLeave)
      el.addEventListener('pointerenter', onEnter)
      el.addEventListener('pointerleave', onLeave)
    })

    // ── 响应式尺寸 ────────────────────────────────────────────────────────────
    const ro = new ResizeObserver(() => {
      ;[el0, el1, el2, el3, el4].forEach((el, i) => {
        const w = el.clientWidth
        const h = el.clientHeight
        // 跳过高度为 0 的容器（成交量面板隐藏时）
        if (w > 0 && h > 0) {
          charts.current[i]?.applyOptions({ width: w, height: h })
        }
      })
    })
    ;[el0, el1, el2, el3, el4].forEach(el => ro.observe(el))

    return () => {
      ro.disconnect()
      ;[el0, el1, el2, el3, el4].forEach((el, i) => {
        el.removeEventListener('pointerenter', enterHandlers[i])
        el.removeEventListener('pointerleave', leaveHandlers[i])
      })
      ;[cc, vc, qc, sc, fc].forEach(c => c.remove())
      charts.current = []
      sers.current   = []
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    showFearGreedPanelRef.current = showFearGreedPanel
  }, [showFearGreedPanel])

  useEffect(() => {
    const cleanup = initCharts()
    return cleanup
  }, [initCharts])

  // ── 数据更新 ──────────────────────────────────────────────────────────────────

  useEffect(() => {
    const [cs, vs, qs, qEma, zLine, scoreLine, fgLine] = sers.current
    if (!cs || !vs || !qs || !qEma || !zLine || !scoreLine || !fgLine || klines.length === 0) return

    const map = new Map<number, KLineItem>()
    klines.forEach(k => map.set(k.time, k))
    klMap.current = map

    cs.setData(klines.map(toCandle))
    vs.setData(klines.map((k, i) => toVol(k,  i, klines[i - 1])))
    qs.setData(klines.map((k, i) => toQVol(k, i, klines[i - 1])))

    const emaData = klines.map((k, i) => ({
      time: k.time as UTCTimestamp,
      value: quoteVolumeLogEma[i] ?? 0,
    }))
    qEma.setData(emaData)

    const zData = klines.map((k, i) => ({
      time: k.time as UTCTimestamp,
      value: quoteVolumeZ[i] ?? 0,
    }))
    zLine.setData(zData)

    const signalMap = new Map<number, ReversalSignalItem>()
    reversalSignals.forEach(s => signalMap.set(s.time, s))
    const scoreData = klines.map(k => {
      const s = signalMap.get(k.time)
      if (!s) return { time: k.time as UTCTimestamp, value: 0 }
      const signed = s.type === 'top' ? Math.abs(s.score) : -Math.abs(s.score)
      return { time: k.time as UTCTimestamp, value: signed }
    })
    scoreLine.setData(scoreData)

    emaMap.current = new Map(emaData.map(d => [Number(d.time), d.value]))
    zMap.current = new Map(zData.map(d => [Number(d.time), d.value]))
    scoreMap.current = new Map(scoreData.map(d => [Number(d.time), d.value]))

    const rawFgDayMap = new Map(
      fearGreedHistory.map(item => [dayKey(item.timestamp), item.value] as const),
    )
    const fgData = klines.map(k => {
      const key = dayKey(k.time)
      return {
        time: k.time as UTCTimestamp,
        value: rawFgDayMap.get(key) ?? 0,
      }
    })
    fgLine.setData(fgData)
    fgMap.current = new Map(fgData.map(d => [Number(d.time), d.value]))
    fgDayMap.current = new Map(fgData.map(d => [dayKey(Number(d.time)), d.value]))

    const reversalMarkers = showReversalMarkers
      ? reversalSignals.map(s => ({
          time: s.time as UTCTimestamp,
          position: s.type === 'top' ? 'aboveBar' : 'belowBar',
          color: s.type === 'top' ? '#ef5350' : '#26a69a',
          shape: s.type === 'top' ? 'arrowDown' : 'arrowUp',
          text: `${s.type === 'top' ? 'Top' : 'Bottom'} z=${s.score.toFixed(2)}`,
        }))
      : []

    const regimeMarkers = showRegimeMarkers
      ? regimeStartpoints.map(p => {
          const isBull = p.state === 'BULL'
          const isBear = p.state === 'BEAR'
          return {
            time: p.time as UTCTimestamp,
            position: (isBull ? 'belowBar' : isBear ? 'aboveBar' : 'inBar') as 'aboveBar' | 'belowBar' | 'inBar',
            color: isBull ? '#16c784' : isBear ? '#f6465d' : '#f0b429',
            shape: (isBull ? 'arrowUp' : isBear ? 'arrowDown' : 'circle') as 'arrowUp' | 'arrowDown' | 'circle',
            text: `Regime ${p.state} ${(p.confidence * 100).toFixed(1)}%`,
          }
        })
      : []

    const markers = [...reversalMarkers, ...regimeMarkers]
      .sort((a, b) => Number(a.time) - Number(b.time))

    // lightweight-charts v4 标记接口
    ;(cs as any).setMarkers(markers)

    charts.current[0]?.timeScale().fitContent()
  }, [klines, quoteVolumeLogEma, quoteVolumeZ, reversalSignals, showReversalMarkers, regimeStartpoints, showRegimeMarkers, fearGreedHistory])

  useEffect(() => {
    const [,, , qEma, zLine, scoreLine, fgLine] = sers.current
    if (!qEma || !zLine || !scoreLine || !fgLine) return
    qEma.applyOptions({ visible: showEmaScorePanel })
    zLine.applyOptions({ visible: showEmaScorePanel })
    scoreLine.applyOptions({ visible: showEmaScorePanel && showScoreLine })
    fgLine.applyOptions({ visible: showFearGreedPanel })

    const scChart = charts.current[3]
    const fgChart = charts.current[4]
    scChart?.timeScale().applyOptions({ visible: !showFearGreedPanel })
    fgChart?.timeScale().applyOptions({ visible: showFearGreedPanel })
  }, [showEmaScorePanel, showScoreLine, showFearGreedPanel])

  // ── 分隔条拖拽（i, j 为要调整比例的面板索引） ────────────────────────────────

  const onDivDown = (i: number, j: number) => (e: RPointerEvent) => {
    e.preventDefault()
    const startY    = e.clientY
    const r0        = [...ratios]
    const total     = areaRef.current?.clientHeight ?? 0
    const visibleCount = [true, showVol, showAmountPanel, showEmaScorePanel, showFearGreedPanel].filter(Boolean).length
    const divCount  = Math.max(0, visibleCount - 1)
    const avail     = Math.max(1, total - DIVIDER_H * divCount)
    // 隐藏面板不参与占比
    const effR      = [r0[0], showVol ? r0[1] : 0, showAmountPanel ? r0[2] : 0, showEmaScorePanel ? r0[3] : 0, showFearGreedPanel ? r0[4] : 0]
    const sum       = effR.reduce((a, b) => a + b, 0)
    const upx0      = (effR[i] / sum) * avail
    const dpx0      = (effR[j] / sum) * avail

    const onMove = (ev: PointerEvent) => {
      const delta = ev.clientY - startY
      const upx   = Math.max(MIN_PX, upx0 + delta)
      const dpx   = Math.max(MIN_PX, dpx0 - delta)
      const nr    = [...r0]
      nr[i] = (upx / avail) * sum
      nr[j] = (dpx / avail) * sum
      setRatios(nr)
    }
    const onUp = () => {
      window.removeEventListener('pointermove', onMove)
      window.removeEventListener('pointerup', onUp)
    }
    window.addEventListener('pointermove', onMove)
    window.addEventListener('pointerup', onUp)
  }

  // ── 渲染 ──────────────────────────────────────────────────────────────────────

  const empty = klines.length === 0
  const clr   = tip?.up ? '#26a69a' : '#ef5350'
  const latestZ = quoteVolumeZ.length > 0 ? quoteVolumeZ[quoteVolumeZ.length - 1] : 0

  return (
    <div className={styles.wrapper}>

      {/* ── header / 图例 ── */}
      <div className={styles.header}>
        <span className={styles.pair}>{symbol}</span>
        <span className={styles.intv}>{interval}</span>

        {tip ? (
          <div className={styles.legend}>
            <span className={styles.lt}>{tip.time}</span>
            <span>O&nbsp;<b style={{ color: clr }}>{tip.open}</b></span>
            <span>H&nbsp;<b style={{ color: clr }}>{tip.high}</b></span>
            <span>L&nbsp;<b style={{ color: clr }}>{tip.low}</b></span>
            <span>C&nbsp;<b style={{ color: clr }}>{tip.close}</b></span>
            <span style={{ color: clr }}>
              {tip.change >= 0 ? '+' : ''}{tip.change.toFixed(2)}
              &nbsp;({tip.changePct >= 0 ? '+' : ''}{tip.changePct.toFixed(2)}%)
            </span>
            <span className={styles.lt}>
              Vol&nbsp;<b style={{ color: '#e6edf3' }}>{fmtAmt(tip.vol)}</b>
            </span>
            <span className={styles.lt}>
              USDT&nbsp;<b style={{ color: '#e6edf3' }}>{fmtAmt(tip.qvol)}</b>
            </span>
          </div>
        ) : (
          <>
            <span className={styles.count}>{klines.length.toLocaleString()} 根 K 线</span>
            <span className={styles.lt}>USDT放量Z分数: <b style={{ color: latestZ >= 2 ? '#ef5350' : '#58a6ff' }}>{latestZ.toFixed(2)}</b></span>
          </>
        )}
      </div>

      {/* ── 图表区域 ── */}
      <div ref={areaRef} className={styles.area}>
        {empty && <div className={styles.empty}>暂无数据，请选择 Symbol 和 Interval</div>}

        {/* 蜡烛图面板 */}
        <div className={styles.pane} style={{ flex: ratios[0] }}>
          <div className={styles.plabel}>主K线面板（价格行为：OHLC）</div>
          <div ref={pane0Ref} className={styles.inner} />
        </div>

        {/* 分隔条 1：蜡烛 ↔ 成交量（仅 showVol 时显示） */}
        {showVol && (
          <div className={styles.divider} onPointerDown={onDivDown(0, 1)} />
        )}

        {/* 成交量面板（任务1：根据 showVol 折叠/展开） */}
        <div
          className={styles.pane}
          style={showVol
            ? { flex: ratios[1] }
            : { flex: '0 0 0px', overflow: 'hidden' }
          }
        >
          <div className={styles.plabel}>成交量面板（量能强弱）</div>
          <div ref={pane1Ref} className={styles.inner} />
        </div>

        {/* 分隔条 2：连接前一可见面板与后一可见面板 */}
        {(showAmountPanel || showEmaScorePanel || showFearGreedPanel) && (
          <div
            className={styles.divider}
            onPointerDown={showVol
              ? onDivDown(1, showAmountPanel ? 2 : showEmaScorePanel ? 3 : 4)
              : onDivDown(0, showAmountPanel ? 2 : showEmaScorePanel ? 3 : 4)}
          />
        )}

        {/* 成交额面板 */}
        <div className={styles.pane} style={showAmountPanel ? { flex: ratios[2] } : { flex: '0 0 0px', overflow: 'hidden' }}>
          <div className={styles.plabel}>成交额面板（资金成交额：USDT柱形）</div>
          <div ref={pane2Ref} className={styles.inner} />
        </div>

        {/* 分隔条 3：成交额 ↔ 信号 */}
        {showAmountPanel && (showEmaScorePanel || showFearGreedPanel) && (
          <div
            className={styles.divider}
            onPointerDown={onDivDown(2, showEmaScorePanel ? 3 : 4)}
          />
        )}

        {/* 信号面板 */}
        <div className={styles.pane} style={showEmaScorePanel ? { flex: ratios[3] } : { flex: '0 0 0px', overflow: 'hidden' }}>
          <div className={styles.plabel}>成交额EMA分数面板（黄=ln成交额EMA20，蓝=放量Z，红=Top/Bottom分数）</div>
          <div ref={pane3Ref} className={styles.inner} />
        </div>

        {/* 分隔条 4：信号 ↔ 贪婪指数（日线） */}
        {showEmaScorePanel && showFearGreedPanel && (
          <div className={styles.divider} onPointerDown={onDivDown(3, 4)} />
        )}

        {/* 贪婪指数面板（仅日线） */}
        <div className={styles.pane} style={showFearGreedPanel ? { flex: ratios[4] } : { flex: '0 0 0px', overflow: 'hidden' }}>
          <div className={styles.plabel}>比特币贪婪指数面板（日线，缺失日期按0补齐）</div>
          <div ref={pane4Ref} className={styles.inner} />
        </div>
      </div>

    </div>
  )
}

export default KLineChart
