<script setup lang="ts">
// 开放平台 → OpenAPI 调用审计（需求 11173 拆分自原单页）
//
// 表：open.open_api_invoke_log（只增不改，按 sys_config 19003 保留天数清理）
// 接口：GET /Meta/Finv/OpenAPI/InvokeLog/Page
//
// 注意：鉴权关闭（19001=0）期间中间件不写审计，页面空态文案已说明。
import { computed, onMounted, ref } from 'vue'

import { apiGet } from '../../../../api'
import { showToast } from '../../../../ui/toast'
import TextPopover from '../../../../components/TextPopover.vue'
import { OPENAPI_BASE as BASE, PASS_OPTIONS, REASON_OPTIONS, authModeText, passColor, passText, reasonText, timeText } from './shared'

interface InvokeLogRow {
  idn: number
  client_idn: number
  credential_idn: number
  credential_no: string
  parent_idn: number
  parent_credential_no: string
  access_key_id: string
  kfp: string
  tfp: string
  auth_mode: string
  request_id: string
  dt_request: string
  method: string
  path: string
  query_str: string
  status_code: number
  biz_code: number
  flag_pass: string
  reason_code: string
  ip: string
  user_agent: string
  body_sha256: string
  count_body: number
  latency_ms: number
}

const rows = ref<InvokeLogRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const flagPass = ref('')
const reasonCode = ref('')
const accessKey = ref('')
const kfp = ref('')

const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

// 详情抽屉：一行一个字段，取值不截断（FE18：字段 / 取值 / 说明 三列）
const detail = ref<InvokeLogRow | null>(null)
const detailOpen = computed({
  get: () => detail.value !== null,
  set: (value: boolean) => { if (!value) detail.value = null },
})

interface DetailField { label: string; value: string; hint: string }

function detailFields(row: InvokeLogRow): DetailField[] {
  return [
    { label: 'IDN', value: String(row.idn), hint: '审计主键（自增）' },
    { label: '请求时间', value: timeText(row.dt_request), hint: '服务端收到请求的时间（dt_request）' },
    { label: '方法', value: row.method || '—', hint: 'HTTP 方法' },
    { label: '路径', value: row.path || '—', hint: '请求路径（不含查询串）' },
    { label: '查询串', value: row.query_str || '—', hint: '原始查询串；静态 Key 写接口需带 `_` 毫秒时间戳' },
    { label: '结果', value: passText(row.flag_pass), hint: 'flag_pass：1 放行 / 0 拒绝' },
    { label: '拒绝原因', value: row.reason_code ? `${reasonText(row.reason_code)}（${row.reason_code}）` : '—', hint: '与 190xx 错误码一一对应；放行为空' },
    { label: 'HTTP', value: String(row.status_code), hint: '本次响应的 HTTP 状态码' },
    { label: '业务码', value: String(row.biz_code), hint: '响应信封里的 code' },
    { label: '耗时（ms）', value: String(row.latency_ms), hint: '鉴权+业务处理耗时' },
    { label: '认证模式', value: authModeText(row.auth_mode), hint: 'APIKEY 静态 Key / AKSK 签名 / STS 临时凭据 / 空=未识别' },
    { label: '公开标识', value: row.access_key_id || '—', hint: 'AK 明文；静态 Key 为脱敏形态（前缀 + 前 8 + 等量星号 + 后 8）' },
    { label: '密钥指纹 kfp', value: row.kfp || '—', hint: 'sha256(完整公开标识) 前 16 位大写十六进制；APIKEY 记静态 Key 指纹、STS 记临时 AK 指纹，可按单把密钥聚合' },
    { label: '令牌指纹 tfp', value: row.tfp || '—', hint: 'sha256(SecurityToken) 前 16 位，仅 STS 模式；可按一次临时会话聚合' },
    { label: '调用方 IDN', value: String(row.client_idn), hint: '0 = 未识别到调用方' },
    { label: '本次凭证', value: row.credential_no || (row.credential_idn > 0 ? `idn=${row.credential_idn}` : '—'), hint: '本次调用使用的凭证号（STS 行是 STS 自己的凭证号，需求 11202）' },
    { label: '父凭证（STS 来源）', value: row.parent_credential_no || (row.auth_mode === 'STS' ? '无（页面直发）' : '—'), hint: 'STS 记录派生它的长期凭证；页面直发的 STS 没有父凭证' },
    { label: '凭证 IDN', value: String(row.credential_idn), hint: '0 = 未识别到凭证' },
    { label: '请求 ID', value: row.request_id || '—', hint: '网关生成的 32 位大写十六进制，与响应头 X-Request-Id 同值' },
    { label: '来源 IP', value: row.ip || '—', hint: '客户端出口 IP（可能被反代改写）' },
    { label: 'User-Agent', value: row.user_agent || '—', hint: '原始 UA，超过 512 字符截断入库' },
    { label: '请求体摘要', value: row.body_sha256 || '—', hint: 'sha256(body) 大写十六进制；不存原文' },
    { label: '请求体长度', value: String(row.count_body), hint: '字节数' },
  ]
}

// ---- 列宽可调（拖拽表头右边缘 / 双击自适应 / 记住宽度）----
interface Column {
  key: keyof InvokeLogRow | 'actions'
  title: string
  width: number   // 默认宽度（px）
  min: number     // 最小宽度（px）
  align?: string
}

const COLUMNS: Column[] = [
  { key: 'dt_request', title: '请求时间', width: 180, min: 140 },
  { key: 'method', title: '方法', width: 80, min: 60 },
  { key: 'path', title: '路径', width: 340, min: 160 },
  { key: 'flag_pass', title: '结果', width: 90, min: 70 },
  { key: 'reason_code', title: '原因', width: 120, min: 90 },
  { key: 'status_code', title: 'HTTP', width: 80, min: 60 },
  { key: 'biz_code', title: '业务码', width: 90, min: 70 },
  { key: 'latency_ms', title: '耗时（ms）', width: 100, min: 80 },
  { key: 'auth_mode', title: '认证模式', width: 130, min: 100 },
  { key: 'credential_no', title: '本次凭证', width: 190, min: 140 },
  { key: 'parent_credential_no', title: '父凭证（STS 来源）', width: 200, min: 140 },
  { key: 'access_key_id', title: '公开标识', width: 320, min: 140 },
  { key: 'kfp', title: '密钥指纹', width: 160, min: 120 },
  { key: 'ip', title: '来源 IP', width: 140, min: 110 },
  { key: 'request_id', title: '请求 ID', width: 320, min: 140 },
  { key: 'actions', title: '操作', width: 96, min: 80, align: 'text-right' },
]

const WIDTHS_KEY = 'finv.openapi.invokelog.widths'
const colWidths = ref<Record<string, number>>({})

function loadWidths() {
  const defaults: Record<string, number> = {}
  for (const col of COLUMNS) defaults[col.key] = col.width
  try {
    const saved = JSON.parse(localStorage.getItem(WIDTHS_KEY) || '{}') as Record<string, number>
    for (const col of COLUMNS) {
      const value = Number(saved[col.key])
      if (Number.isFinite(value) && value >= col.min) defaults[col.key] = value
    }
  } catch {
    // 本地缓存坏了就用默认宽度，不影响页面
  }
  colWidths.value = defaults
}

function saveWidths() {
  try {
    localStorage.setItem(WIDTHS_KEY, JSON.stringify(colWidths.value))
  } catch {
    // 隐私模式禁止 localStorage 时忽略
  }
}

function resetWidths() {
  for (const col of COLUMNS) colWidths.value[col.key] = col.width
  saveWidths()
  showToast('列宽已重置为默认', 'success')
}

// 拖拽调宽：按住表头右侧手柄左右移动
let resizing: { key: string; startX: number; startWidth: number } | null = null

function startResize(event: MouseEvent, col: Column) {
  event.preventDefault()
  event.stopPropagation()
  resizing = { key: String(col.key), startX: event.clientX, startWidth: colWidths.value[col.key] }
  document.addEventListener('mousemove', onResizing)
  document.addEventListener('mouseup', stopResize)
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'
}

function onResizing(event: MouseEvent) {
  if (!resizing) return
  const col = COLUMNS.find((item) => String(item.key) === resizing!.key)
  if (!col) return
  const next = Math.max(col.min, Math.min(1200, resizing.startWidth + (event.clientX - resizing.startX)))
  colWidths.value[resizing.key] = next
}

function stopResize() {
  if (resizing) saveWidths()
  resizing = null
  document.removeEventListener('mousemove', onResizing)
  document.removeEventListener('mouseup', stopResize)
  document.body.style.cursor = ''
  document.body.style.userSelect = ''
}

// 双击手柄：按该列内容自适应宽度（不必手动拖）
function autoFit(col: Column) {
  const canvas = document.createElement('canvas')
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  ctx.font = '12px ui-monospace, SFMono-Regular, Menlo, Consolas, monospace'
  let widest = ctx.measureText(col.title).width + 40
  for (const row of rows.value) {
    const text = cellText(row, col)
    widest = Math.max(widest, ctx.measureText(text).width + 28)
  }
  colWidths.value[col.key] = Math.max(col.min, Math.min(1200, Math.ceil(widest)))
  saveWidths()
}

function cellText(row: InvokeLogRow, col: Column): string {
  switch (col.key) {
    case 'dt_request': return timeText(row.dt_request)
    case 'flag_pass': return passText(row.flag_pass)
    case 'auth_mode': return authModeText(row.auth_mode)
    case 'credential_no': return row.credential_no || (row.credential_idn > 0 ? `idn=${row.credential_idn}` : '—')
    case 'parent_credential_no': return row.parent_credential_no || (row.auth_mode === 'STS' ? '无（页面直发）' : '—')
    case 'reason_code': return reasonText(row.reason_code)
    case 'actions': return '详情'
    default: return String((row as unknown as Record<string, unknown>)[col.key] ?? '')
  }
}

async function copyValue(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    showToast('已复制', 'success')
  } catch {
    showToast('复制失败（浏览器不允许）', 'error')
  }
}

async function load() {
  loading.value = true
  try {
    const params = new URLSearchParams({ page: String(page.value), pageSize: String(pageSize.value) })
    if (flagPass.value) params.set('flagPass', flagPass.value)
    if (reasonCode.value) params.set('reasonCode', reasonCode.value)
    if (accessKey.value.trim()) params.set('accessKey', accessKey.value.trim())
    if (kfp.value.trim()) params.set('kfp', kfp.value.trim())
    const data = await apiGet<{ total: number; list: InvokeLogRow[] }>(
      `${BASE}/InvokeLog/Page?${params.toString()}`,
    )
    rows.value = data.list ?? []
    total.value = data.total ?? 0
  } catch (err) {
    rows.value = []
    total.value = 0
    showToast(`查询调用审计失败：${(err as Error).message}`, 'error')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadWidths()
  load()
})
</script>

<template>
  <div class="openapi-page">
    <v-card variant="outlined">
      <v-card-title class="d-flex align-center flex-wrap ga-2">
        <v-icon icon="mdi-text-box-search-outline" class="mr-1" />
        OpenAPI 调用审计
        <v-chip size="small" variant="tonal" color="primary">open.open_api_invoke_log</v-chip>
        <v-chip size="small" variant="tonal">共 {{ total }} 条</v-chip>
      </v-card-title>
      <v-card-subtitle class="pb-2">
        放行与被拒的调用都会留痕；只存请求体摘要与长度，不存原文。按系统配置 19003 保留天数清理。
      </v-card-subtitle>

      <v-card-text>
        <div class="d-flex flex-wrap align-center ga-2 mb-3">
          <v-select
            v-model="flagPass"
            :items="PASS_OPTIONS"
            item-title="title"
            item-value="value"
            label="结果"
            density="compact"
            hide-details
            style="max-width: 150px"
            @update:model-value="flagPass = String($event); page = 1; load()"
          />
          <v-select
            v-model="reasonCode"
            :items="REASON_OPTIONS"
            item-title="title"
            item-value="value"
            label="拒绝原因"
            density="compact"
            hide-details
            style="max-width: 200px"
            @update:model-value="reasonCode = String($event); page = 1; load()"
          />
          <v-text-field
            v-model="accessKey"
            label="公开标识（模糊）"
            density="compact"
            hide-details
            clearable
            style="min-width: 180px; max-width: 240px"
            @keyup.enter="page = 1; load()"
          />
          <v-text-field
            v-model="kfp"
            label="密钥指纹（精确）"
            density="compact"
            hide-details
            clearable
            style="min-width: 170px; max-width: 210px"
            @keyup.enter="page = 1; load()"
          />
          <v-btn color="primary" variant="tonal" prepend-icon="mdi-magnify" @click="page = 1; load()">查询</v-btn>
        </div>

        <div class="d-flex align-center flex-wrap ga-2 mb-2">
          <span class="text-caption text-medium-emphasis">
            列宽可拖动：把鼠标放到表头右边缘拖动；双击边缘按内容自适应；
          </span>
          <v-btn size="x-small" variant="text" prepend-icon="mdi-restore" @click="resetWidths">重置列宽</v-btn>
        </div>

        <div class="openapi-table-scroll">
          <v-table density="compact" class="openapi-table openapi-table-fixed">
            <colgroup>
              <col v-for="col in COLUMNS" :key="col.key" :style="{ width: colWidths[col.key] + 'px' }" />
            </colgroup>
            <thead>
              <tr>
                <th v-for="col in COLUMNS" :key="col.key" :class="col.align" class="openapi-th">
                  <span class="th-label">{{ col.title }}</span>
                  <span
                    class="col-resizer"
                    title="拖动调整列宽；双击按内容自适应"
                    @mousedown="startResize($event, col)"
                    @dblclick="autoFit(col)"
                  />
                </th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in rows" :key="row.idn">
                <td class="cell-nowrap">
                  <span>{{ timeText(row.dt_request) }}</span>
                </td>
                <td class="cell-nowrap">{{ row.method || '—' }}</td>
                <td class="cell-ellipsis">
                  <span>{{ row.path || '—' }}</span>
                  <TextPopover v-if="row.path" title="完整路径" :text="row.path" />
                </td>
                <td>
                  <v-chip size="x-small" variant="tonal" :color="passColor(row.flag_pass)">
                    {{ passText(row.flag_pass) }}
                  </v-chip>
                </td>
                <td class="cell-nowrap">{{ reasonText(row.reason_code) }}</td>
                <td>{{ row.status_code }}</td>
                <td>{{ row.biz_code }}</td>
                <td>{{ row.latency_ms }}</td>
                <td class="cell-nowrap">{{ authModeText(row.auth_mode) }}</td>
                <td class="cell-ellipsis">
                  <span>{{ row.credential_no || (row.credential_idn > 0 ? `idn=${row.credential_idn}` : '—') }}</span>
                  <TextPopover
                    v-if="row.credential_no"
                    title="凭证"
                    :lines="[`凭证号：${row.credential_no}`, `IDN：${row.credential_idn}`]"
                  />
                </td>
                <td class="cell-ellipsis">{{ row.parent_credential_no || (row.auth_mode === 'STS' ? '无（页面直发）' : '—') }}</td>
                <td class="cell-ellipsis">
                  <span>{{ row.access_key_id || '—' }}</span>
                  <TextPopover v-if="row.access_key_id" title="公开标识（完整值）" :text="row.access_key_id" />
                </td>
                <td class="cell-nowrap text-caption">{{ row.kfp || '—' }}</td>
                <td class="cell-nowrap">{{ row.ip || '—' }}</td>
                <td class="cell-ellipsis">
                  <span>{{ row.request_id || '—' }}</span>
                  <TextPopover v-if="row.request_id" title="请求 ID" :text="row.request_id" />
                </td>
                <td class="text-right">
                  <v-btn size="x-small" variant="tonal" prepend-icon="mdi-eye-outline" @click="detail = row">详情</v-btn>
                </td>
              </tr>
              <tr v-if="rows.length === 0">
                <td :colspan="COLUMNS.length" class="text-center text-medium-emphasis py-6">
                  {{ loading ? '加载中…' : '暂无调用记录（鉴权关闭时不写审计）' }}
                </td>
              </tr>
            </tbody>
          </v-table>
        </div>

        <div class="d-flex align-center justify-end ga-2 mt-3">
          <v-pagination v-model="page" :length="pageCount" density="compact" total-visible="7" @update:model-value="load" />
        </div>
      </v-card-text>
    </v-card>

    <!-- 详情：一行一个字段，取值不截断（FE18：字段 / 取值 / 说明） -->
    <v-dialog v-model="detailOpen" max-width="900" scrollable>
      <v-card v-if="detail">
        <v-card-title class="d-flex align-center flex-wrap ga-2">
          <v-icon icon="mdi-text-box-search-outline" class="mr-1" />
          调用详情 #{{ detail.idn }}
          <v-chip size="small" variant="tonal" :color="passColor(detail.flag_pass)">
            {{ passText(detail.flag_pass) }}
          </v-chip>
          <v-chip size="small" variant="tonal">{{ detail.method }} {{ detail.status_code }}</v-chip>
          <v-spacer />
          <v-btn size="small" variant="text" prepend-icon="mdi-close" @click="detail = null">关闭</v-btn>
        </v-card-title>
        <v-divider />
        <v-card-text>
          <v-table density="compact" class="openapi-detail-table">
            <thead>
              <tr>
                <th style="width: 150px">字段</th>
                <th>取值</th>
                <th style="width: 260px">说明</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="field in detailFields(detail)" :key="field.label">
                <td class="text-no-wrap font-weight-medium">{{ field.label }}</td>
                <td class="detail-value">
                  <span>{{ field.value }}</span>
                  <v-btn
                    v-if="field.value && field.value !== '—'"
                    icon="mdi-content-copy"
                    size="x-small"
                    variant="text"
                    class="ml-1"
                    @click="copyValue(field.value)"
                  />
                </td>
                <td class="text-caption text-medium-emphasis">{{ field.hint }}</td>
              </tr>
            </tbody>
          </v-table>
        </v-card-text>
      </v-card>
    </v-dialog>

    <v-card class="mt-4" variant="outlined">
      <v-card-title class="text-subtitle-1">
        <v-icon icon="mdi-help-circle-outline" class="mr-1" />
        说明
      </v-card-title>
      <v-card-text class="text-body-2">
        <p>
          ① 写入时机：`/OpenAPI/V1/*` 的每次鉴权校验（放行与拒绝都算）。鉴权模式为
          <strong>关闭</strong>时不校验也不写审计，因此这段时间没有记录。
        </p>
        <p>
          ② <strong>拒绝原因</strong>与错误码一一对应：未携带凭证 <code>19001</code>、凭证无效 <code>19002</code>、
          停用/吊销/过期 <code>19003</code>、IP 不在白名单 <code>19004</code>、签名不符 <code>19005</code>、
          时间戳超窗 <code>19006</code>、nonce 重放 <code>19007</code>、超限流 <code>19008</code>、
          超出授权范围 <code>19009</code>、鉴权未就绪 <code>19010</code>。
        </p>
        <p>
          ③ <strong>公开标识</strong>：AK/SK 模式记 AK（公开）；静态 Key 模式只记脱敏形态（前 8 后 4），
          密钥本身不落审计表。
        </p>
        <p>
          ④ 只存 <code>body_sha256</code> 与请求体长度，不存请求原文；排障时用它比对两端是否同一份载荷。
        </p>
      </v-card-text>
    </v-card>
  </div>
</template>

<style scoped>
.openapi-page {
  padding: 16px;
}

.openapi-table-scroll {
  overflow-x: auto;
}

/* 固定布局让 colgroup 的像素宽度生效，列宽才真正可调 */
.openapi-table-fixed :deep(table) {
  table-layout: fixed;
  width: max-content;
}

.openapi-table :deep(th),
.openapi-table :deep(td) {
  white-space: nowrap;
  overflow: hidden;
}

/* 单元格：不换行；宽度不够时才出省略号（可拖宽或看 ⓘ） */
.cell-nowrap {
  white-space: nowrap;
}

.cell-ellipsis {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* 表头单元格 + 右侧拖拽手柄 */
.openapi-th {
  position: relative;
  white-space: nowrap;
  user-select: none;
}

.th-label {
  display: inline-block;
  padding-right: 10px;
}

.col-resizer {
  position: absolute;
  top: 0;
  right: 0;
  width: 8px;
  height: 100%;
  cursor: col-resize;
  user-select: none;
}

.col-resizer:hover {
  background: rgba(var(--v-theme-primary), 0.25);
}

.openapi-detail-table :deep(td) {
  vertical-align: top;
}

/* 详情里的取值不截断：长串自动换行，不出现省略号 */
.detail-value {
  white-space: normal;
  word-break: break-all;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.5;
}
</style>
