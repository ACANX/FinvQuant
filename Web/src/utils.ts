// 前端共享工具函数（评审：fmtDate 等格式化函数在多个页面重复实现 5 处，统一收敛于此）
// 用法：import { fmtDate, fmtNum, fmtPct, statusColor, debounce } from '../utils'

/** yyyymmdd → yyyy-MM-dd（0 或空返回 '-'） */
export function fmtDate(d: number | undefined | null): string {
  if (!d) return '-'
  const s = String(d)
  if (s.length !== 8) return s
  return `${s.slice(0, 4)}-${s.slice(4, 6)}-${s.slice(6, 8)}`
}

/** hhmmss → HH:mm:ss（0 或空返回 '-'） */
export function fmtTime(t: number | undefined | null): string {
  if (!t) return '-'
  const s = String(t).padStart(6, '0')
  return `${s.slice(0, 2)}:${s.slice(2, 4)}:${s.slice(4, 6)}`
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
