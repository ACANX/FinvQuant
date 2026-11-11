// 「开放平台」各页面共用的枚举字典与展示辅助（需求 11173）。
//
// 为什么要抽出来：调用方 / 凭证 / 调用审计 / 鉴权配置拆成四个独立页面后，
// 枚举中文、时间格式、状态配色必须共用一份，避免四份各写各的导致口径漂移。
//
// 枚举显示口径（Docs/DevSpec/DictFilterSpec.md、DictColumnSpec.md）：
//   下拉 → 「中文[值]」；表格 → 只显示中文。

/** 鉴权模式（sys_config 19001） */
export const MODE_OPTIONS = [
  { title: '关闭（全部放行）[0]', value: '0' },
  { title: '启用（强制鉴权）[1]', value: '1' },
  { title: '宽松（校验并记日志，仍放行）[2]', value: '2' },
]
export const MODE_LABELS: Record<string, string> = {
  '0': '关闭',
  '1': '启用',
  '2': '宽松',
}

/** 凭证认证模式（open.open_api_credential.auth_mode） */
export const AUTH_MODE_LABELS: Record<string, string> = {
  API_KEY: '静态 Key',
  AK_SK: 'AK/SK 签名',
  BEARER: 'Bearer JWT（预留）',
  MTLS: '双向 TLS（预留）',
  BASIC: '用户名口令（预留）',
  FALLBACK: '兼容兜底',
}

/** 调用结果（open.open_api_invoke_log.flag_pass） */
export const PASS_OPTIONS = [
  { title: '全部结果', value: '' },
  { title: '放行[1]', value: '1' },
  { title: '拒绝[0]', value: '0' },
]
export const PASS_LABELS: Record<string, string> = { '1': '放行', '0': '拒绝' }

/** 拒绝原因（open.open_api_invoke_log.reason_code，与错误码 190xx 一一对应） */
export const REASON_LABELS: Record<string, string> = {
  NO_KEY: '未携带凭证',
  BAD_KEY: '凭证无效',
  REVOKE: '已吊销',
  EXPIRE: '已停用/过期',
  CIDR_DENY: 'IP 不在白名单',
  SIGN_MISMATCH: '签名不匹配',
  TS_SKEW: '时间戳超窗',
  NONCE_REPLAY: 'nonce 重放',
  SCOPE_DENY: '超出授权范围',
  RATE_LIMIT: '超速率/配额',
  NOT_READY: '鉴权未就绪',
}
export const REASON_OPTIONS = [
  { title: '全部原因', value: '' },
  ...Object.keys(REASON_LABELS).map((value) => ({ title: REASON_LABELS[value], value })),
]

/** 认证模式中文 */
export function authModeText(value: string | null | undefined): string {
  const key = String(value ?? '').trim()
  if (!key) return '—'
  return AUTH_MODE_LABELS[key] ?? key
}

/** 放行/拒绝中文 */
export function passText(value: string): string {
  return PASS_LABELS[value] ?? value
}

/** 放行/拒绝配色 */
export function passColor(value: string): string {
  return value === '1' ? 'success' : 'error'
}

/** 拒绝原因中文 */
export function reasonText(value: string | null | undefined): string {
  const key = String(value ?? '').trim()
  if (!key) return '—'
  return REASON_LABELS[key] ?? key
}

/** 时间统一展示为「yyyy-MM-dd HH:mm:ss」（后端给 RFC3339 或空串） */
export function timeText(value: string | null | undefined): string {
  const raw = String(value ?? '').trim()
  if (!raw) return '—'
  const parsed = new Date(raw)
  if (Number.isNaN(parsed.getTime())) return raw
  const pad = (n: number) => String(n).padStart(2, '0')
  return (
    `${parsed.getFullYear()}-${pad(parsed.getMonth() + 1)}-${pad(parsed.getDate())} ` +
    `${pad(parsed.getHours())}:${pad(parsed.getMinutes())}:${pad(parsed.getSeconds())}`
  )
}

/** 调用方 / 凭证状态配色：已吊销 → error，启用 → success，停用 → warning */
export function statusColor(row: { flag_enable: string; flag_revoke?: string }): string {
  if (row.flag_revoke === '1') return 'error'
  return row.flag_enable === '1' ? 'success' : 'warning'
}

/** 调用方 / 凭证状态文案 */
export function statusText(row: { flag_enable: string; flag_revoke?: string }): string {
  if (row.flag_revoke === '1') return '已吊销'
  return row.flag_enable === '1' ? '启用' : '停用'
}

/** IP 白名单展示（空 = 不限） */
export function cidrText(items: string[] | null | undefined): string {
  if (!items || items.length === 0) return '不限'
  return items.join(', ')
}

/** 速率/配额展示（0 = 不限） */
export function limitText(value: number): string {
  return value > 0 ? String(value) : '不限'
}

/** 调用方下拉选项 */
export interface ClientOption {
  idn: number
  client_code: string
  client_name: string
}

export const OPENAPI_BASE = '/Meta/Finv/OpenAPI'
