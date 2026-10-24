// 前端共享工具函数（评审：fmtDate 等格式化函数在多个页面重复实现 5 处，统一收敛于此）
// 用法：import { fmtDate, fmtNum, fmtPct, statusColor, debounce } from '../utils'

/** yyyymmdd → yyyy-MM-dd（0 或空返回 '-'） */
export function fmtDate(d: number | undefined | null): string {
  if (!d) return '-'
  const s = String(d)
  if (s.length !== 8) return s
  return `${s.slice(0, 4)}-${s.slice(4, 6)}-${s.slice(6, 8)}`
}

/** hhmmss → HH:mm:ss（null/undefined 返回 '-'；0 是合法的 00:00:00） */
export function fmtTime(t: number | undefined | null): string {
  if (t === undefined || t === null) return '-'
  const s = String(t).padStart(6, '0')
  return `${s.slice(0, 2)}:${s.slice(2, 4)}:${s.slice(4, 6)}`
}

/** 本地时区 Date → yyyy-MM-dd */
export function isoDate(d: Date): string {
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${m}-${day}`
}

/** 本地今天 yyyy-MM-dd */
export function todayISO(): string {
  return isoDate(new Date())
}

/**
 * yyyy-MM-dd 月份平移（delta 为负数表示往前），对月末做钳制：
 * 2026-03-31 -1 月 → 2026-02-28，而不是溢出到 3 月。
 */
export function shiftMonthISO(iso: string, delta: number): string {
  const [year, month, day] = iso.split('-').map(Number)
  const target = new Date(year, month - 1 + delta, 1)
  const lastDay = new Date(target.getFullYear(), target.getMonth() + 1, 0).getDate()
  target.setDate(Math.min(day, lastDay))
  return isoDate(target)
}

/** 日期 + 时间组合显示 */
export function fmtDateTime(d: number | undefined | null, t: number | undefined | null): string {
  return `${fmtDate(d)} ${fmtTime(t)}`
}

/** 数值格式化（千分位；NaN/空显示 '-'；>= profitFactorInfSentinel 显示 ∞） */
export function fmtNum(v: number | undefined | null, digits = 2): string {
  if (v === undefined || v === null || Number.isNaN(v)) return '-'
  if (Math.abs(v) >= 999999) return '∞'
  return Number(v).toLocaleString('en-US', { maximumFractionDigits: digits })
}

/** 百分比格式化 */
export function fmtPct(v: number | undefined | null): string {
  if (v === undefined || v === null || Number.isNaN(v)) return '-'
  if (Math.abs(v) >= 999999) return '∞'
  return `${Number(v).toFixed(2)}%`
}

/** ISO 8601 时间字符串 → 本地 yyyy-MM-dd HH:mm:ss（空/非法返回 '-'） */
export function fmtISO(v: string | null | undefined): string {
  if (!v) return '-'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

/** 字节数 → 人类可读（B / KB / MB / GB / TB） */
export function fmtBytes(v: number | null | undefined): string {
  if (v === null || v === undefined || Number.isNaN(v)) return '-'
  if (v < 1024) return `${v} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let value = v / 1024
  let index = 0
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index += 1
  }
  return `${value.toFixed(2)} ${units[index]}`
}

/** yyyymm 数字 → yyyy-MM（0 / 空返回 '-'） */
export function fmtMonth(v: number | null | undefined): string {
  if (!v) return '-'
  const s = String(v)
  return s.length === 6 ? `${s.slice(0, 4)}-${s.slice(4, 6)}` : s
}

/** 任务状态 → 颜色 */
export function statusColor(s: string): string {
  return { PENDING: 'grey', RUNNING: 'primary', SUCCEEDED: 'success', FAILED: 'error', CANCELLED: 'warning' }[s] ?? 'grey'
}

/** 任务状态中文名 */
export function statusName(s: string): string {
  return { PENDING: '待执行', RUNNING: '执行中', SUCCEEDED: '成功', FAILED: '失败', CANCELLED: '已取消' }[s] ?? s
}

/** 防抖（切换按钮等高频操作） */
export function debounce<T extends (...args: never[]) => void>(fn: T, wait = 300): (...args: Parameters<T>) => void {
  let timer: ReturnType<typeof setTimeout> | null = null
  return (...args: Parameters<T>) => {
    if (timer) clearTimeout(timer)
    timer = setTimeout(() => fn(...args), wait)
  }
}
