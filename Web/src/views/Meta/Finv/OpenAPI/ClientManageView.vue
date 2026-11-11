<script setup lang="ts">
// OpenAPI 调用方管理（需求 11169）
//
// 三张表（迁移 V300183）：
//   open.open_api_client       调用方（一行 = 一个接入应用）
//   open.open_api_credential   凭证（一行 = 一种认证模式的密钥）
//   open.open_api_invoke_log   调用审计（只增不改，按 sys_config 19003 保留天数清理）
//
// 接口（前缀 /Meta/Finv/OpenAPI）：
//   GET  /Settings             读鉴权配置（19001～19003 + 密钥是否已配置）
//   PUT  /Settings             保存 19001～19003
//   GET  /Client/Page          调用方分页
//   POST /Client/Save          新建/编辑调用方（idn=0 新建，默认停用）
//   POST /Client/Enable        启用/停用调用方
//   GET  /Credential/Page      凭证分页
//   POST /Credential/Create    签发凭证（密钥明文只返回一次）
//   POST /Credential/Enable    启用/停用凭证
//   POST /Credential/Revoke    吊销凭证（原因必填，不可恢复）
//   POST /Credential/Rotate    轮换密钥（新密钥只返回一次）
//   GET  /InvokeLog/Page       调用审计分页
//
// 口径：密钥只显示一次（库里只有哈希/密文），任何列表都不回显 secret_*。
import { computed, onMounted, ref, watch } from 'vue'

import { apiGet, apiPost, apiPut } from '../../../../api'
import { showToast } from '../../../../ui/toast'
import OpenAPIClientDialog from './ClientDialog.vue'
import OpenAPICredentialDialog from './CredentialDialog.vue'

const BASE = '/Meta/Finv/OpenAPI'

// ---------- 枚举（下拉用「中文[值]」，表格只显示中文） ----------

const MODE_OPTIONS = [
  { title: '关闭（全部放行）[0]', value: '0' },
  { title: '启用（强制鉴权）[1]', value: '1' },
  { title: '宽松（校验并记日志，仍放行）[2]', value: '2' },
]
const MODE_LABELS: Record<string, string> = {
  '0': '关闭',
  '1': '启用',
  '2': '宽松',
}

const AUTH_MODE_LABELS: Record<string, string> = {
  API_KEY: '静态 Key',
  AK_SK: 'AK/SK 签名',
  BEARER: 'Bearer JWT（预留）',
  MTLS: '双向 TLS（预留）',
  BASIC: '用户名口令（预留）',
  FALLBACK: '兼容兜底',
}

const PASS_OPTIONS = [
  { title: '全部结果', value: '' },
  { title: '放行[1]', value: '1' },
  { title: '拒绝[0]', value: '0' },
]
const PASS_LABELS: Record<string, string> = { '1': '放行', '0': '拒绝' }

const REASON_LABELS: Record<string, string> = {
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
const REASON_OPTIONS = [
  { title: '全部原因', value: '' },
  ...Object.keys(REASON_LABELS).map((value) => ({ title: REASON_LABELS[value], value })),
]

// ---------- 鉴权配置 ----------

interface Settings {
  mode: string
  sign_time_window: number
  log_retain_days: number
  secret_kid: string
  secret_key_ready: boolean
  fallback_key_ready: boolean
}

const settings = ref<Settings>({
  mode: '0',
  sign_time_window: 300,
  log_retain_days: 90,
  secret_kid: 'k1',
  secret_key_ready: false,
  fallback_key_ready: false,
})
const settingsMode = ref('0')
const settingsWindow = ref(300)
const settingsRetain = ref(90)
const savingSettings = ref(false)

async function loadSettings() {
  try {
    const data = await apiGet<Settings>(`${BASE}/Settings`)
    settings.value = data
    settingsMode.value = data.mode
    settingsWindow.value = data.sign_time_window
    settingsRetain.value = data.log_retain_days
  } catch (err) {
    showToast(`读取鉴权配置失败：${(err as Error).message}`, 'error')
  }
}

async function saveSettings() {
  savingSettings.value = true
  try {
    const data = await apiPut<Settings>(`${BASE}/Settings`, {
      mode: settingsMode.value,
      sign_time_window: Number(settingsWindow.value),
      log_retain_days: Number(settingsRetain.value),
    })
    settings.value = data
    showToast('鉴权配置已保存（10 秒内生效）', 'success')
  } catch (err) {
    showToast(`保存失败：${(err as Error).message}`, 'error')
  } finally {
    savingSettings.value = false
  }
}

const modeHint = computed(() => {
  switch (settingsMode.value) {
    case '1':
      return '强制鉴权：未通过校验的调用一律返回 190xx'
    case '2':
      return '宽松模式：照常校验并写审计，但失败仍放行（联调用）'
    default:
      return '鉴权关闭：全部放行，行为与早期过渡实现一致（不写审计）'
  }
})

// ---------- Tab 1：调用方 ----------

interface ClientRow {
  idn: number
  client_code: string
  client_name: string
  owner_uid: string
  contact_email: string
  contract_no: string
  allow_cidr: string[] | null
  rate_limit_rpm: number
  quota_day: number
  dt_expire: string
  flag_enable: string
  remark: string
  gmt_update: string
  count_credential: number
  count_credential_active: number
}

const clients = ref<ClientRow[]>([])
const clientTotal = ref(0)
const clientKeyword = ref('')
const clientPage = ref(1)
const clientPageSize = ref(20)
const clientLoading = ref(false)

const clientPageCount = computed(() => Math.max(1, Math.ceil(clientTotal.value / clientPageSize.value)))

async function loadClients() {
  clientLoading.value = true
  try {
    const params = new URLSearchParams({
      page: String(clientPage.value),
      pageSize: String(clientPageSize.value),
    })
    if (clientKeyword.value.trim()) params.set('keyword', clientKeyword.value.trim())
    const data = await apiGet<{ total: number; list: ClientRow[] }>(`${BASE}/Client/Page?${params.toString()}`)
    clients.value = data.list ?? []
    clientTotal.value = data.total ?? 0
  } catch (err) {
    clients.value = []
    clientTotal.value = 0
    showToast(`查询调用方失败：${(err as Error).message}`, 'error')
  } finally {
    clientLoading.value = false
  }
}

const clientDialogOpen = ref(false)
const clientDialogIDN = ref(0)

function openClientCreate() {
  clientDialogIDN.value = 0
  clientDialogOpen.value = true
}

function openClientEdit(row: ClientRow) {
  clientDialogIDN.value = row.idn
  clientDialogOpen.value = true
}

async function toggleClient(row: ClientRow) {
  const enable = row.flag_enable !== '1'
  const action = enable ? '启用' : '停用'
  if (!enable && !window.confirm(`确认停用调用方「${row.client_code}」？停用后其全部凭证立即失效。`)) return
  try {
    await apiPost(`${BASE}/Client/Enable`, { idn: row.idn, enable })
    showToast(`调用方已${action}`, 'success')
    await loadClients()
  } catch (err) {
    showToast(`${action}失败：${(err as Error).message}`, 'error')
  }
}

// ---------- Tab 2：凭证 ----------

interface CredentialRow {
  idn: number
  client_idn: number
  client_code: string
  credential_no: string
  auth_mode: string
  access_key_id: string
  key_prefix: string
  has_secret: boolean
  sign_algorithm: string
  scope: string[] | null
  allow_cidr: string[] | null
  dt_expire: string
  dt_last_use: string
  count_use: number
  flag_enable: string
  flag_revoke: string
  dt_revoke: string
  reason_revoke: string
  remark: string
  gmt_update: string
}

const credentials = ref<CredentialRow[]>([])
const credentialTotal = ref(0)
const credentialKeyword = ref('')
const credentialClientIDN = ref(0)
const credentialPage = ref(1)
const credentialPageSize = ref(20)
const credentialLoading = ref(false)

const credentialPageCount = computed(() =>
  Math.max(1, Math.ceil(credentialTotal.value / credentialPageSize.value)),
)

const credentialClientOptions = computed(() => [
  { title: '全部调用方', value: 0 },
  ...clients.value.map((row) => ({ title: `${row.client_code}（${row.client_name}）`, value: row.idn })),
])

async function loadCredentials() {
  credentialLoading.value = true
  try {
    const params = new URLSearchParams({
      page: String(credentialPage.value),
      pageSize: String(credentialPageSize.value),
    })
    if (credentialClientIDN.value > 0) params.set('clientId', String(credentialClientIDN.value))
    if (credentialKeyword.value.trim()) params.set('keyword', credentialKeyword.value.trim())
    const data = await apiGet<{ total: number; list: CredentialRow[] }>(
      `${BASE}/Credential/Page?${params.toString()}`,
    )
    credentials.value = data.list ?? []
    credentialTotal.value = data.total ?? 0
  } catch (err) {
    credentials.value = []
    credentialTotal.value = 0
    showToast(`查询凭证失败：${(err as Error).message}`, 'error')
  } finally {
    credentialLoading.value = false
  }
}

// 签发 / 轮换：同一个对话框，结果里展示一次性密钥
const credentialDialogOpen = ref(false)
const credentialDialogMode = ref<'create' | 'rotate'>('create')
const credentialDialogClientIDN = ref(0)
const credentialDialogIDN = ref(0)

function openCredentialCreate() {
  if (clients.value.length === 0) {
    showToast('请先登记调用方，再签发凭证', 'warning')
    return
  }
  credentialDialogMode.value = 'create'
  credentialDialogClientIDN.value = credentialClientIDN.value || clients.value[0].idn
  credentialDialogIDN.value = 0
  credentialDialogOpen.value = true
}

function openCredentialRotate(row: CredentialRow) {
  if (!window.confirm(`确认轮换凭证「${row.credential_no}」的密钥？旧密钥立即失效（AK/SK 旧 SK 仅窗口内可验签）。`)) {
    return
  }
  credentialDialogMode.value = 'rotate'
  credentialDialogClientIDN.value = row.client_idn
  credentialDialogIDN.value = row.idn
  credentialDialogOpen.value = true
}

async function toggleCredential(row: CredentialRow) {
  const enable = row.flag_enable !== '1'
  const action = enable ? '启用' : '停用'
  try {
    await apiPost(`${BASE}/Credential/Enable`, { idn: row.idn, enable })
    showToast(`凭证已${action}`, 'success')
    await loadCredentials()
  } catch (err) {
    showToast(`${action}失败：${(err as Error).message}`, 'error')
  }
}

const revokeDialogOpen = ref(false)
const revokeTarget = ref<CredentialRow | null>(null)
const revokeReason = ref('')
const revoking = ref(false)

function openRevoke(row: CredentialRow) {
  revokeTarget.value = row
  revokeReason.value = ''
  revokeDialogOpen.value = true
}

async function confirmRevoke() {
  if (!revokeTarget.value) return
  if (!revokeReason.value.trim()) {
    showToast('吊销必须填写原因', 'warning')
    return
  }
  revoking.value = true
  try {
    await apiPost(`${BASE}/Credential/Revoke`, {
      idn: revokeTarget.value.idn,
      reason: revokeReason.value.trim(),
    })
    showToast('凭证已吊销（不可恢复）', 'success')
    revokeDialogOpen.value = false
    await loadCredentials()
  } catch (err) {
    showToast(`吊销失败：${(err as Error).message}`, 'error')
  } finally {
    revoking.value = false
  }
}

// ---------- Tab 3：调用审计 ----------

interface InvokeLogRow {
  idn: number
  client_idn: number
  credential_idn: number
  access_key_id: string
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

const logs = ref<InvokeLogRow[]>([])
const logTotal = ref(0)
const logPage = ref(1)
const logPageSize = ref(20)
const logLoading = ref(false)
const logFlagPass = ref('')
const logReason = ref('')
const logAccessKey = ref('')

const logPageCount = computed(() => Math.max(1, Math.ceil(logTotal.value / logPageSize.value)))

async function loadLogs() {
  logLoading.value = true
  try {
    const params = new URLSearchParams({
      page: String(logPage.value),
      pageSize: String(logPageSize.value),
    })
    if (logFlagPass.value) params.set('flagPass', logFlagPass.value)
    if (logReason.value) params.set('reasonCode', logReason.value)
    if (logAccessKey.value.trim()) params.set('accessKey', logAccessKey.value.trim())
    const data = await apiGet<{ total: number; list: InvokeLogRow[] }>(
      `${BASE}/InvokeLog/Page?${params.toString()}`,
    )
    logs.value = data.list ?? []
    logTotal.value = data.total ?? 0
  } catch (err) {
    logs.value = []
    logTotal.value = 0
    showToast(`查询调用审计失败：${(err as Error).message}`, 'error')
  } finally {
    logLoading.value = false
  }
}

// ---------- 展示辅助 ----------

function authModeText(value: string | null | undefined): string {
  const key = String(value ?? '').trim()
  if (!key) return '—'
  return AUTH_MODE_LABELS[key] ?? key
}

function passText(value: string): string {
  return PASS_LABELS[value] ?? value
}

function passColor(value: string): string {
  return value === '1' ? 'success' : 'error'
}

function reasonText(value: string | null | undefined): string {
  const key = String(value ?? '').trim()
  if (!key) return '—'
  return REASON_LABELS[key] ?? key
}

/** 时间统一展示为「yyyy-MM-dd HH:mm:ss」（后端给 RFC3339 或空串） */
function timeText(value: string | null | undefined): string {
  const raw = String(value ?? '').trim()
  if (!raw) return '—'
  const parsed = new Date(raw)
  if (Number.isNaN(parsed.getTime())) return raw
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${parsed.getFullYear()}-${pad(parsed.getMonth() + 1)}-${pad(parsed.getDate())} ` +
    `${pad(parsed.getHours())}:${pad(parsed.getMinutes())}:${pad(parsed.getSeconds())}`
}

function statusColor(row: ClientRow | CredentialRow): string {
  if ('flag_revoke' in row && row.flag_revoke === '1') return 'error'
  return row.flag_enable === '1' ? 'success' : 'warning'
}

function statusText(row: ClientRow | CredentialRow): string {
  if ('flag_revoke' in row && row.flag_revoke === '1') return '已吊销'
  return row.flag_enable === '1' ? '启用' : '停用'
}

function cidrText(items: string[] | null | undefined): string {
  if (!items || items.length === 0) return '不限'
  return items.join(', ')
}

function limitText(value: number): string {
  return value > 0 ? String(value) : '不限'
}

// ---------- Tab 切换与刷新 ----------

const tab = ref('client')

watch(tab, (value) => {
  if (value === 'credential' && credentials.value.length === 0) void loadCredentials()
  if (value === 'log' && logs.value.length === 0) void loadLogs()
})

watch(clientKeyword, (value) => {
  if (value === '') {
    clientPage.value = 1
    void loadClients()
  }
})

onMounted(async () => {
  await loadSettings()
  await loadClients()
})
</script>

<template>
  <div class="openapi-admin">
    <v-card class="mb-4" variant="outlined">
      <v-card-title class="d-flex align-center flex-wrap ga-2">
        <v-icon icon="mdi-key-chain" class="mr-1" />
        OpenAPI 调用方管理
        <v-chip size="small" variant="tonal" color="primary">/OpenAPI/V1</v-chip>
        <v-chip size="small" variant="tonal" :color="settings.mode === '1' ? 'success' : 'warning'">
          当前鉴权：{{ MODE_LABELS[settings.mode] ?? settings.mode }}
        </v-chip>
      </v-card-title>
      <v-card-subtitle class="pb-2">
        管理对外 OpenAPI 的调用方与密钥，并查看调用审计。密钥只在签发/轮换时显示一次，之后任何界面都不再回显。
      </v-card-subtitle>

      <v-card-text>
        <div class="d-flex flex-wrap align-center ga-3">
          <v-select
            v-model="settingsMode"
            :items="MODE_OPTIONS"
            item-title="title"
            item-value="value"
            label="鉴权模式"
            density="compact"
            hide-details
            style="min-width: 260px; max-width: 320px"
            @update:model-value="settingsMode = String($event)"
          />
          <v-text-field
            v-model.number="settingsWindow"
            type="number"
            label="签名时间窗（秒）"
            density="compact"
            hide-details
            min="30"
            max="3600"
            style="max-width: 170px"
          />
          <v-text-field
            v-model.number="settingsRetain"
            type="number"
            label="日志保留天数"
            density="compact"
            hide-details
            min="0"
            max="3650"
            style="max-width: 170px"
          />
          <v-btn color="primary" prepend-icon="mdi-content-save" :loading="savingSettings" @click="saveSettings">
            保存配置
          </v-btn>
          <v-chip size="small" variant="tonal">密钥版本 KID：{{ settings.secret_kid }}</v-chip>
          <v-chip size="small" variant="tonal" :color="settings.secret_key_ready ? 'success' : 'warning'">
            19005 加密密钥：{{ settings.secret_key_ready ? '已配置' : '未配置' }}
          </v-chip>
          <v-chip size="small" variant="tonal" :color="settings.fallback_key_ready ? 'success' : 'warning'">
            19006 兜底密钥：{{ settings.fallback_key_ready ? '已配置' : '未配置' }}
          </v-chip>
        </div>
        <div class="text-caption text-medium-emphasis mt-2">{{ modeHint }}（配置保存后 10 秒内生效，无需重启）</div>
        <v-alert
          v-if="!settings.secret_key_ready"
          class="mt-3"
          type="info"
          density="compact"
          variant="tonal"
          text="19005 加密密钥未配置：静态 API Key 可正常签发与使用；AK/SK 模式会被拒绝（19010），签发前请先在系统配置里填写 32 字节 Base64 密钥。"
        />
      </v-card-text>
    </v-card>

    <v-card variant="outlined">
      <v-tabs v-model="tab" density="compact" class="px-3 pt-2">
        <v-tab value="client">
          <v-icon icon="mdi-account-key-outline" start />
          调用方
        </v-tab>
        <v-tab value="credential">
          <v-icon icon="mdi-key-chain" start />
          凭证
        </v-tab>
        <v-tab value="log">
          <v-icon icon="mdi-text-box-search-outline" start />
          调用审计
        </v-tab>
      </v-tabs>

      <v-window v-model="tab">
        <!-- ============ Tab 1 调用方 ============ -->
        <v-window-item value="client">
          <v-card-text>
            <div class="d-flex flex-wrap align-center ga-2 mb-3">
              <v-text-field
                v-model="clientKeyword"
                label="关键字（代号 / 名称）"
                density="compact"
                hide-details
                clearable
                style="min-width: 220px; max-width: 300px"
                @keyup.enter="clientPage = 1; loadClients()"
              />
              <v-btn color="primary" variant="tonal" prepend-icon="mdi-magnify" @click="clientPage = 1; loadClients()">
                查询
              </v-btn>
              <v-btn color="primary" prepend-icon="mdi-plus" @click="openClientCreate">新增调用方</v-btn>
              <v-spacer />
              <span class="text-caption text-medium-emphasis">共 {{ clientTotal }} 个调用方</span>
            </div>

            <v-table density="comfortable" class="openapi-table">
              <thead>
                <tr>
                  <th>代号</th>
                  <th>名称</th>
                  <th>状态</th>
                  <th>凭证（生效/全部）</th>
                  <th>速率（次/分）</th>
                  <th>每日配额</th>
                  <th>IP 白名单</th>
                  <th>到期</th>
                  <th>最近更新</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in clients" :key="row.idn">
                  <td class="font-weight-medium">{{ row.client_code }}</td>
                  <td>{{ row.client_name }}</td>
                  <td>
                    <v-chip size="small" variant="tonal" :color="statusColor(row)">{{ statusText(row) }}</v-chip>
                  </td>
                  <td>{{ row.count_credential_active }} / {{ row.count_credential }}</td>
                  <td>{{ limitText(row.rate_limit_rpm) }}</td>
                  <td>{{ limitText(row.quota_day) }}</td>
                  <td class="text-truncate" style="max-width: 180px">{{ cidrText(row.allow_cidr) }}</td>
                  <td>{{ timeText(row.dt_expire) }}</td>
                  <td>{{ timeText(row.gmt_update) }}</td>
                  <td class="text-no-wrap">
                    <v-btn size="small" variant="text" @click="openClientEdit(row)">编辑</v-btn>
                    <v-btn
                      size="small"
                      variant="text"
                      :color="row.flag_enable === '1' ? 'warning' : 'success'"
                      @click="toggleClient(row)"
                    >
                      {{ row.flag_enable === '1' ? '停用' : '启用' }}
                    </v-btn>
                  </td>
                </tr>
                <tr v-if="clients.length === 0">
                  <td colspan="10" class="text-center text-medium-emphasis py-6">
                    {{ clientLoading ? '加载中…' : '暂无调用方，点「新增调用方」登记第一个' }}
                  </td>
                </tr>
              </tbody>
            </v-table>

            <div class="d-flex align-center justify-end ga-2 mt-3">
              <v-pagination
                v-model="clientPage"
                :length="clientPageCount"
                density="compact"
                total-visible="7"
                @update:model-value="loadClients"
              />
            </div>
          </v-card-text>
        </v-window-item>

        <!-- ============ Tab 2 凭证 ============ -->
        <v-window-item value="credential">
          <v-card-text>
            <div class="d-flex flex-wrap align-center ga-2 mb-3">
              <v-select
                v-model="credentialClientIDN"
                :items="credentialClientOptions"
                item-title="title"
                item-value="value"
                label="调用方"
                density="compact"
                hide-details
                style="min-width: 220px; max-width: 280px"
                @update:model-value="credentialClientIDN = Number($event); credentialPage = 1; loadCredentials()"
              />
              <v-text-field
                v-model="credentialKeyword"
                label="关键字（凭证号 / 公开标识）"
                density="compact"
                hide-details
                clearable
                style="min-width: 200px; max-width: 280px"
                @keyup.enter="credentialPage = 1; loadCredentials()"
              />
              <v-btn
                color="primary"
                variant="tonal"
                prepend-icon="mdi-magnify"
                @click="credentialPage = 1; loadCredentials()"
              >
                查询
              </v-btn>
              <v-btn color="primary" prepend-icon="mdi-key-plus" @click="openCredentialCreate">签发凭证</v-btn>
              <v-spacer />
              <span class="text-caption text-medium-emphasis">共 {{ credentialTotal }} 条凭证</span>
            </div>

            <v-table density="comfortable" class="openapi-table">
              <thead>
                <tr>
                  <th>凭证号</th>
                  <th>调用方</th>
                  <th>认证模式</th>
                  <th>公开标识</th>
                  <th>授权范围</th>
                  <th>状态</th>
                  <th>最近使用</th>
                  <th>累计调用</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in credentials" :key="row.idn">
                  <td class="font-weight-medium">{{ row.credential_no }}</td>
                  <td>{{ row.client_code }}</td>
                  <td>{{ authModeText(row.auth_mode) }}</td>
                  <td>
                    <code>{{ row.key_prefix }}…</code>
                    <v-tooltip location="bottom" max-width="420">
                      <template #activator="{ props }">
                        <v-icon v-bind="props" icon="mdi-information-outline" size="small" class="ml-1" />
                      </template>
                      <div>公开标识（完整值）：<code>{{ row.access_key_id }}</code></div>
                      <div class="mt-1">密钥材料：{{ row.has_secret ? '已写入（不可回显）' : '缺失' }}</div>
                    </v-tooltip>
                  </td>
                  <td>
                    <span v-if="!row.scope || row.scope.length === 0" class="text-medium-emphasis">全部接口</span>
                    <template v-else>
                      <span>{{ row.scope.length }} 条</span>
                      <v-tooltip location="bottom" max-width="480">
                        <template #activator="{ props }">
                          <v-icon v-bind="props" icon="mdi-eye-outline" size="small" class="ml-1" />
                        </template>
                        <div v-for="item in row.scope" :key="item"><code>{{ item }}</code></div>
                      </v-tooltip>
                    </template>
                  </td>
                  <td>
                    <v-chip size="small" variant="tonal" :color="statusColor(row)">{{ statusText(row) }}</v-chip>
                    <v-tooltip v-if="row.flag_revoke === '1'" location="bottom" max-width="360">
                      <template #activator="{ props }">
                        <v-icon v-bind="props" icon="mdi-information-outline" size="small" class="ml-1" />
                      </template>
                      <div>吊销时间：{{ timeText(row.dt_revoke) }}</div>
                      <div>原因：{{ row.reason_revoke || '—' }}</div>
                    </v-tooltip>
                  </td>
                  <td>{{ timeText(row.dt_last_use) }}</td>
                  <td>{{ row.count_use }}</td>
                  <td class="text-no-wrap">
                    <template v-if="row.flag_revoke !== '1'">
                      <v-btn size="small" variant="text" @click="toggleCredential(row)">
                        {{ row.flag_enable === '1' ? '停用' : '启用' }}
                      </v-btn>
                      <v-btn size="small" variant="text" color="primary" @click="openCredentialRotate(row)">
                        轮换
                      </v-btn>
                      <v-btn size="small" variant="text" color="error" @click="openRevoke(row)">吊销</v-btn>
                    </template>
                    <span v-else class="text-medium-emphasis text-caption">已吊销，只能新建</span>
                  </td>
                </tr>
                <tr v-if="credentials.length === 0">
                  <td colspan="9" class="text-center text-medium-emphasis py-6">
                    {{ credentialLoading ? '加载中…' : '暂无凭证，点「签发凭证」生成密钥' }}
                  </td>
                </tr>
              </tbody>
            </v-table>

            <div class="d-flex align-center justify-end ga-2 mt-3">
              <v-pagination
                v-model="credentialPage"
                :length="credentialPageCount"
                density="compact"
                total-visible="7"
                @update:model-value="loadCredentials"
              />
            </div>
          </v-card-text>
        </v-window-item>

        <!-- ============ Tab 3 调用审计 ============ -->
        <v-window-item value="log">
          <v-card-text>
            <div class="d-flex flex-wrap align-center ga-2 mb-3">
              <v-select
                v-model="logFlagPass"
                :items="PASS_OPTIONS"
                item-title="title"
                item-value="value"
                label="结果"
                density="compact"
                hide-details
                style="max-width: 150px"
                @update:model-value="logFlagPass = String($event); logPage = 1; loadLogs()"
              />
              <v-select
                v-model="logReason"
                :items="REASON_OPTIONS"
                item-title="title"
                item-value="value"
                label="拒绝原因"
                density="compact"
                hide-details
                style="max-width: 190px"
                @update:model-value="logReason = String($event); logPage = 1; loadLogs()"
              />
              <v-text-field
                v-model="logAccessKey"
                label="公开标识（模糊）"
                density="compact"
                hide-details
                clearable
                style="min-width: 180px; max-width: 240px"
                @keyup.enter="logPage = 1; loadLogs()"
              />
              <v-btn color="primary" variant="tonal" prepend-icon="mdi-magnify" @click="logPage = 1; loadLogs()">
                查询
              </v-btn>
              <v-spacer />
              <span class="text-caption text-medium-emphasis">共 {{ logTotal }} 条记录</span>
            </div>

            <v-table density="compact" class="openapi-table">
              <thead>
                <tr>
                  <th>请求时间</th>
                  <th>方法</th>
                  <th>路径</th>
                  <th>结果</th>
                  <th>原因</th>
                  <th>HTTP</th>
                  <th>耗时（ms）</th>
                  <th>公开标识</th>
                  <th>来源 IP</th>
                  <th>请求 ID</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in logs" :key="row.idn">
                  <td class="text-no-wrap">{{ timeText(row.dt_request) }}</td>
                  <td>{{ row.method || '—' }}</td>
                  <td class="text-truncate" style="max-width: 260px">{{ row.path || '—' }}</td>
                  <td>
                    <v-chip size="x-small" variant="tonal" :color="passColor(row.flag_pass)">
                      {{ passText(row.flag_pass) }}
                    </v-chip>
                  </td>
                  <td>{{ reasonText(row.reason_code) }}</td>
                  <td>{{ row.status_code }}</td>
                  <td>{{ row.latency_ms }}</td>
                  <td class="text-truncate" style="max-width: 180px">{{ row.access_key_id || '—' }}</td>
                  <td>{{ row.ip || '—' }}</td>
                  <td class="text-truncate" style="max-width: 150px">{{ row.request_id || '—' }}</td>
                </tr>
                <tr v-if="logs.length === 0">
                  <td colspan="10" class="text-center text-medium-emphasis py-6">
                    {{ logLoading ? '加载中…' : '暂无调用记录（鉴权关闭时不会写审计）' }}
                  </td>
                </tr>
              </tbody>
            </v-table>

            <div class="d-flex align-center justify-end ga-2 mt-3">
              <v-pagination
                v-model="logPage"
                :length="logPageCount"
                density="compact"
                total-visible="7"
                @update:model-value="loadLogs"
              />
            </div>
          </v-card-text>
        </v-window-item>
      </v-window>
    </v-card>

    <!-- 页面底部唯一说明卡片（ExplainCardSpec） -->
    <v-card class="mt-4" variant="outlined">
      <v-card-title class="text-subtitle-1">
        <v-icon icon="mdi-help-circle-outline" class="mr-1" />
        说明
      </v-card-title>
      <v-card-text class="text-body-2">
        <p>
          ① <strong>鉴权模式</strong>（系统配置 19001）：关闭 = 全部放行、不写审计（默认）；
          启用 = 未通过校验即返回 <code>190xx</code>；宽松 = 照常校验并写审计但失败仍放行，用于旧调用方灰度改造。
          保存后 10 秒内生效，无需重启。
        </p>
        <p>
          ② <strong>模式一 静态 Key</strong>：请求带 <code>X-API-Key</code> 或 <code>Authorization: Bearer</code>；
          库里只存 <code>sha256</code> 哈希，签发时显示一次，之后无法取回。
        </p>
        <p>
          ③ <strong>模式二 AK/SK</strong>：请求带 <code>Authorization: FinvAK &lt;AK&gt;:&lt;签名&gt;</code> +
          <code>X-Finv-Timestamp</code> + <code>X-Finv-Nonce</code>；SK 以 AES-256-GCM 密文存储，
          加密密钥取系统配置 <code>19005</code>（未配置时 AK/SK 一律拒绝，返回 <code>19010</code>）。
        </p>
        <p>
          ④ <strong>状态语义</strong>：<em>停用</em>可逆（不改密钥）；<em>吊销</em>不可逆，必须填写原因并留痕，之后只能新建凭证。
          调用方停用时其全部凭证立即失效。
        </p>
        <p>
          ⑤ <strong>调用审计</strong>（<code>open.open_api_invoke_log</code>）记录放行与被拒两类请求，
          只存请求体摘要与长度、不存原文；按系统配置 <code>19003</code>（默认 90 天）清理。
          鉴权关闭期间不写审计。
        </p>
        <p>
          ⑥ <strong>兼容兜底</strong>：环境变量 <code>FINV_OPENAPI_KEY</code> 与系统配置 <code>19006</code>
          任一命中即按旧口径放行（审计里标记为「兼容兜底」），过渡结束后清空这两个值即可关闭。
        </p>
      </v-card-text>
    </v-card>

    <OpenAPIClientDialog
      v-model="clientDialogOpen"
      :client-idn="clientDialogIDN"
      @saved="loadClients"
    />

    <OpenAPICredentialDialog
      v-model="credentialDialogOpen"
      :mode="credentialDialogMode"
      :client-idn="credentialDialogClientIDN"
      :credential-idn="credentialDialogIDN"
      :clients="clients"
      @saved="loadCredentials"
    />

    <v-dialog v-model="revokeDialogOpen" max-width="520">
      <v-card>
        <v-card-title class="text-subtitle-1">吊销凭证</v-card-title>
        <v-card-subtitle>
          {{ revokeTarget?.credential_no }}（{{ revokeTarget?.client_code }}）吊销后不可恢复，只能新建凭证。
        </v-card-subtitle>
        <v-card-text>
          <v-textarea
            v-model="revokeReason"
            label="吊销原因（必填）"
            placeholder="如：调用方已下线、密钥疑似泄露、合同到期"
            variant="outlined"
            density="compact"
            rows="3"
            :error="revokeReason.trim() === ''"
            :error-messages="revokeReason.trim() === '' ? '吊销必须填写原因' : ''"
          />
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="revokeDialogOpen = false">取消</v-btn>
          <v-btn color="error" :loading="revoking" :disabled="revokeReason.trim() === ''" @click="confirmRevoke">
            确认吊销
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<style scoped>
.openapi-admin {
  padding: 16px;
}

.openapi-table :deep(th),
.openapi-table :deep(td) {
  white-space: nowrap;
}
</style>
