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
import type { KLineItem } from '../types/kline'
import styles from './KLineChart.module.css'

interface Props { klines: KLineItem[]; symbol: string; interval: string; showVol: boolean }

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
  // 任务2：统一最小宽度，避免各图表价格轴宽度不同导致 x 坐标错位
  rightPriceScale: { borderColor: '#30363d', minimumWidth: 80 },
  handleScale:     { axisPressedMouseMove: { time: true, price: false } },
}

// ── component ──────────────────────────────────────────────────────────────────

const KLineChart: FC<Props> = ({ klines, symbol, interval, showVol }) => {
  // flex-grow 比例：蜡烛图 / 成交量 / 成交额
  const [ratios, setRatios] = useState([6, 2, 2])
  const [tip,    setTip]    = useState<Tip | null>(null)

  // DOM refs
  const areaRef  = useRef<HTMLDivElement>(null)
  const pane0Ref = useRef<HTMLDivElement>(null)   // 蜡烛图容器
  const pane1Ref = useRef<HTMLDivElement>(null)   // 成交量容器
  const pane2Ref = useRef<HTMLDivElement>(null)   // 成交额容器

  // chart / series refs
  const charts = useRef<IChartApi[]>([])
  const sers   = useRef<ISeriesApi<any>[]>([])
  const klMap  = useRef<Map<number, KLineItem>>(new Map())

  // ── 初始化三个独立图表 ────────────────────────────────────────────────────────

  const initCharts = useCallback(() => {
    const el0 = pane0Ref.current
    const el1 = pane1Ref.current
    const el2 = pane2Ref.current
    if (!el0 || !el1 || !el2 || charts.current.length) return

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
    const qc = mk(el2, true)    // 成交额，显示时间轴（最底部）

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
    })

    charts.current = [cc, vc, qc]
    sers.current   = [cs, vs, qs]

    // ── 时间轴同步 ────────────────────────────────────────────────────────────
    let timeLock = false
    ;[cc, vc, qc].forEach((src, si) => {
      const rest = [cc, vc, qc].filter((_, i) => i !== si)
      src.timeScale().subscribeVisibleLogicalRangeChange(r => {
        if (timeLock || !r) return
        timeLock = true
        rest.forEach(o => o.timeScale().setVisibleLogicalRange(r))
        timeLock = false
      })
    })

    // ── 十字线位置同步（只负责同步位置，不操作 horzLine 可见性） ─────────────────
    const serVal = (s: ISeriesApi<any>, k: KLineItem): number =>
      s === cs ? k.close : s === vs ? k.volume : k.quoteVolume

    ;[cc, vc, qc].forEach((src, si) => {
      const others = [cc, vc, qc]
        .map((c, i) => ({ c, s: sers.current[i] }))
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

        others.forEach(o =>
          o.c.setCrosshairPosition(serVal(o.s, k), param.time as UTCTimestamp, o.s)
        )
      })
    })

    // ── horzLine 隔离：用 DOM pointerenter/pointerleave 控制，与 crosshairMove 完全解耦 ──
    // 原因：setCrosshairPosition 会异步触发目标图表的 subscribeCrosshairMove，
    // 导致 lock 方案失效；DOM 事件基于真实鼠标位置，完全可靠。
    const allCharts = [cc, vc, qc]
    const enterHandlers: EventListener[] = []
    const leaveHandlers: EventListener[] = []

    ;[el0, el1, el2].forEach((el, i) => {
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
      ;[el0, el1, el2].forEach((el, i) => {
        const w = el.clientWidth
        const h = el.clientHeight
        // 跳过高度为 0 的容器（成交量面板隐藏时）
        if (w > 0 && h > 0) {
          charts.current[i]?.applyOptions({ width: w, height: h })
        }
      })
    })
    ;[el0, el1, el2].forEach(el => ro.observe(el))

    return () => {
      ro.disconnect()
      ;[el0, el1, el2].forEach((el, i) => {
        el.removeEventListener('pointerenter', enterHandlers[i])
        el.removeEventListener('pointerleave', leaveHandlers[i])
      })
      ;[cc, vc, qc].forEach(c => c.remove())
      charts.current = []
      sers.current   = []
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    const cleanup = initCharts()
    return cleanup
  }, [initCharts])

  // ── 数据更新 ──────────────────────────────────────────────────────────────────

  useEffect(() => {
    const [cs, vs, qs] = sers.current
    if (!cs || !vs || !qs || klines.length === 0) return

    const map = new Map<number, KLineItem>()
    klines.forEach(k => map.set(k.time, k))
    klMap.current = map

    cs.setData(klines.map(toCandle))
    vs.setData(klines.map((k, i) => toVol(k,  i, klines[i - 1])))
    qs.setData(klines.map((k, i) => toQVol(k, i, klines[i - 1])))
    charts.current[0]?.timeScale().fitContent()
  }, [klines])

  // ── 分隔条拖拽（i, j 为要调整比例的面板索引） ────────────────────────────────

  const onDivDown = (i: number, j: number) => (e: RPointerEvent) => {
    e.preventDefault()
    const startY    = e.clientY
    const r0        = [...ratios]
    const total     = areaRef.current?.clientHeight ?? 0
    const divCount  = showVol ? 2 : 1
    const avail     = Math.max(1, total - DIVIDER_H * divCount)
    // 当成交量隐藏时，忽略其占比，只用蜡烛图和成交额的比例
    const effR      = showVol ? r0 : [r0[0], 0, r0[2]]
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
          <span className={styles.count}>{klines.length.toLocaleString()} 根 K 线</span>
        )}
      </div>

      {/* ── 图表区域 ── */}
      <div ref={areaRef} className={styles.area}>
        {empty && <div className={styles.empty}>暂无数据，请选择 Symbol 和 Interval</div>}

        {/* 蜡烛图面板 */}
        <div className={styles.pane} style={{ flex: ratios[0] }}>
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
          <div className={styles.plabel}>成交量 Vol</div>
          <div ref={pane1Ref} className={styles.inner} />
        </div>

        {/* 分隔条 2：当 showVol 时控制成交量↔成交额，否则控制蜡烛↔成交额 */}
        <div
          className={styles.divider}
          onPointerDown={showVol ? onDivDown(1, 2) : onDivDown(0, 2)}
        />

        {/* 成交额面板 */}
        <div className={styles.pane} style={{ flex: ratios[2] }}>
          <div className={styles.plabel}>成交额 USDT</div>
          <div ref={pane2Ref} className={styles.inner} />
        </div>
      </div>

    </div>
  )
}

export default KLineChart
