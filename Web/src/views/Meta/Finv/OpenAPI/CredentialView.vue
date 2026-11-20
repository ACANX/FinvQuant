<script setup lang="ts">
// 开放平台 → OpenAPI 凭证（需求 11173 拆分自原单页）
//
// 表：open.open_api_credential（一行 = 一种认证模式的密钥）
// 接口：GET  /Meta/Finv/OpenAPI/Credential/Page
//       POST /Meta/Finv/OpenAPI/Credential/Create   签发（密钥明文只返回一次）
//       POST /Meta/Finv/OpenAPI/Credential/Enable
//       POST /Meta/Finv/OpenAPI/Credential/Revoke   吊销（原因必填，不可恢复）
//       POST /Meta/Finv/OpenAPI/Credential/Rotate   轮换（新密钥只返回一次）
//       GET  /Meta/Finv/OpenAPI/Client/Page          （仅用于「调用方」下拉与筛选）
//
// 敏感列 secret_hash / secret_cipher 永不返回，列表只展示 has_secret。
import { computed, onMounted, ref, watch } from 'vue'

import { apiGet, apiPost } from '../../../../api'
import TextPopover from '../../../../components/TextPopover.vue'
import { showToast } from '../../../../ui/toast'
import OpenAPICredentialDialog from './CredentialDialog.vue'
import {
  OPENAPI_BASE as BASE,
  authModeText,
  expireText,
  statusColor,
  statusText,
  timeText,
  type ClientOption,
} from './shared'

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

const rows = ref<CredentialRow[]>([])
const total = ref(0)
const keyword = ref('')
const clientIDN = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const clients = ref<ClientOption[]>([])

const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))
const clientOptions = computed(() => [
  { title: '全部调用方', value: 0 },
  ...clients.value.map((row) => ({ title: `${row.client_code}（${row.client_name}）`, value: row.idn })),
])

async function loadClients() {
  try {
    const data = await apiGet<{ total: number; list: ClientOption[] }>(
      `${BASE}/Client/Page?page=1&pageSize=200`,
    )
    clients.value = data.list ?? []
  } catch (err) {
    clients.value = []
    showToast(`读取调用方失败：${(err as Error).message}`, 'error')
  }
}

// 状态过滤（需求 11217）：默认只列可用凭证（未吊销/未停用/未过期）
const status = ref('active')
const STATUS_OPTIONS = [
  { title: '仅有效（未吊销/未停用/未过期）[active]', value: 'active' },
  { title: '全部（含吊销/停用/过期）[all]', value: 'all' },
]

async function load() {
  loading.value = true
  try {
    const params = new URLSearchParams({ page: String(page.value), pageSize: String(pageSize.value) })
    if (clientIDN.value > 0) params.set('clientId', String(clientIDN.value))
    if (keyword.value.trim()) params.set('keyword', keyword.value.trim())
    params.set('status', status.value)
    const data = await apiGet<{ total: number; list: CredentialRow[] }>(
      `${BASE}/Credential/Page?${params.toString()}`,
    )
    rows.value = data.list ?? []
    total.value = data.total ?? 0
  } catch (err) {
    rows.value = []
    total.value = 0
    showToast(`查询凭证失败：${(err as Error).message}`, 'error')
  } finally {
    loading.value = false
  }
}

// 签发 / 轮换共用同一个对话框（结果里展示一次性密钥）
const dialogOpen = ref(false)
const dialogMode = ref<'create' | 'rotate'>('create')
const dialogClientIDN = ref(0)
const dialogCredentialIDN = ref(0)
const dialogCredentialNo = ref('')
const dialogAuthMode = ref('')

function openCreate() {
  if (clients.value.length === 0) {
    showToast('请先在「OpenAPI 调用方」页登记调用方，再签发凭证', 'warning')
    return
  }
  dialogMode.value = 'create'
  dialogClientIDN.value = clientIDN.value || clients.value[0].idn
  dialogCredentialIDN.value = 0
  dialogCredentialNo.value = ''
  dialogAuthMode.value = ''
  dialogOpen.value = true
}

function openRotate(row: CredentialRow) {
  if (!window.confirm(`确认轮换凭证「${row.credential_no}」的密钥？旧密钥立即失效（AK/SK 旧 SK 仅窗口内可验签）。`)) return
  dialogMode.value = 'rotate'
  dialogClientIDN.value = row.client_idn
  dialogCredentialIDN.value = row.idn
  dialogCredentialNo.value = row.credential_no
  dialogAuthMode.value = row.auth_mode
  dialogOpen.value = true
}

async function toggle(row: CredentialRow) {
  const enable = row.flag_enable !== '1'
  const action = enable ? '启用' : '停用'
  try {
    await apiPost(`${BASE}/Credential/Enable`, { idn: row.idn, enable })
    showToast(`凭证已${action}`, 'success')
    await load()
  } catch (err) {
    showToast(`${action}失败：${(err as Error).message}`, 'error')
  }
}

// 改授权范围（需求 11230）：只改 scope，不动密钥/白名单/有效期
interface EndpointOption {
  idn: number
  api_code: string
  http_method: string
  path: string
  api_name: string
}

const scopeOpen = ref(false)
const scopeTarget = ref<CredentialRow | null>(null)
const scopePicked = ref<string[]>([])
const scopeExtra = ref('')
const scopeSaving = ref(false)
const endpointOptions = ref<{ title: string; value: string }[]>([])

// 把凭证现有 scope（"GET /path"）拆成「能对上下拉选项的」与「对不上的（通配/历史值）」两部分
function splitScope(items: string[] | null): { picked: string[]; extra: string } {
  const known = new Set(endpointOptions.value.map((item) => item.value))
  const picked: string[] = []
  const extra: string[] = []
  for (const item of items ?? []) {
    if (known.has(item)) picked.push(item)
    else extra.push(item)
  }
  return { picked, extra: extra.join('\n') }
}

async function openScope(row: CredentialRow) {
  scopeTarget.value = row
  scopeExtra.value = ''
  scopePicked.value = []
  if (endpointOptions.value.length === 0) {
    try {
      const data = await apiGet<{ list: EndpointOption[] }>(`${BASE}/Endpoint/Options`)
      endpointOptions.value = (data.list ?? []).map((item) => ({
        title: `${item.api_name}［${item.http_method} ${item.api_code}］`,
        value: `${item.http_method} ${item.path}`,
      }))
    } catch (err) {
      endpointOptions.value = []
      showToast(`读取接口清单失败（仍可手输路径）：${(err as Error).message}`, 'warning')
    }
  }
  const split = splitScope(row.scope)
  scopePicked.value = split.picked
  scopeExtra.value = split.extra
  scopeOpen.value = true
}

// toList 把 v-select 的取值统一成数组：单选模式（静态 Key，multiple=false）下
// Vuetify 会写字符串进来，直接展开会被拆成单个字符（选中 PageUni 变成 "i"）。
function toList(value: unknown): string[] {
  if (Array.isArray(value)) return value.filter((item): item is string => typeof item === 'string')
  return typeof value === 'string' && value !== '' ? [value] : []
}

// 静态 Key 只能授权 1 个接口：单选写回字符串时归一为数组，多选时只保留最后一个
watch(
  () => scopePicked.value,
  (raw) => {
    const picked = toList(raw)
    if (scopeTarget.value?.auth_mode !== 'APIKEY') return
    if (picked.length > 1) {
      scopePicked.value = [picked[picked.length - 1]]
      showToast('静态 Key 只能授权 1 个接口，已保留最后一次选择', 'warning')
    } else if (!Array.isArray(raw)) {
      scopePicked.value = picked
    }
  },
)

const scopeFinal = computed(() => {
  const merged: string[] = []
  const seen = new Set<string>()
  for (const item of [...toList(scopePicked.value), ...scopeExtra.value.split('\n')]) {
    const value = item.trim()
    if (!value || seen.has(value)) continue
    seen.add(value)
    merged.push(value)
  }
  return merged
})

const scopeHint = computed(() => {
  const mode = scopeTarget.value?.auth_mode ?? ''
  if (mode === 'APIKEY') {
    return `静态 Key 只能授 1 个具体接口（不允通配）；当前 ${scopeFinal.value.length} 条`
  }
  return `留空 = 全部接口；当前 ${scopeFinal.value.length} 条`
})

async function confirmScope() {
  if (!scopeTarget.value) return
  scopeSaving.value = true
  try {
    await apiPost(`${BASE}/Credential/Scope`, {
      idn: scopeTarget.value.idn,
      scope: scopeFinal.value,
    })
    showToast('授权范围已更新（立即生效）', 'success')
    scopeOpen.value = false
    await load()
  } catch (err) {
    showToast(`修改授权范围失败：${(err as Error).message}`, 'error')
  } finally {
    scopeSaving.value = false
  }
}

// 改有效期（需求 11246）：只改 dt_expire，不动密钥/授权范围/白名单
const expireOpen = ref(false)
const expireTarget = ref<CredentialRow | null>(null)
const expireValue = ref('')
const expireSaving = ref(false)

function openExpire(row: CredentialRow) {
  expireTarget.value = row
  expireValue.value = (row.dt_expire ?? '').slice(0, 10)
  expireOpen.value = true
}

const expireInvalid = computed(() => {
  if (expireTarget.value?.auth_mode !== 'APIKEY') return ''
  if (!expireValue.value) return '静态 Key 必须设置到期日（不允许长期）'
  const limit = new Date()
  limit.setDate(limit.getDate() + 90)
  if (expireValue.value > limit.toISOString().slice(0, 10)) return '静态 Key 有效期最长 90 天'
  return ''
})

async function confirmExpire() {
  if (!expireTarget.value || expireInvalid.value) return
  expireSaving.value = true
  try {
    await apiPost(`${BASE}/Credential/Expire`, {
      idn: expireTarget.value.idn,
      dt_expire: expireValue.value ? `${expireValue.value}T00:00:00+08:00` : '',
    })
    showToast('有效期已更新（立即生效）', 'success')
    expireOpen.value = false
    await load()
  } catch (err) {
    showToast(`修改有效期失败：${(err as Error).message}`, 'error')
  } finally {
    expireSaving.value = false
  }
}

const revokeOpen = ref(false)
const revokeTarget = ref<CredentialRow | null>(null)
const revokeReason = ref('')
const revoking = ref(false)

function openRevoke(row: CredentialRow) {
  revokeTarget.value = row
  revokeReason.value = ''
  revokeOpen.value = true
}

async function confirmRevoke() {
  if (!revokeTarget.value || !revokeReason.value.trim()) return
  revoking.value = true
  try {
    await apiPost(`${BASE}/Credential/Revoke`, {
      idn: revokeTarget.value.idn,
      reason: revokeReason.value.trim(),
    })
    showToast('凭证已吊销（不可恢复）', 'success')
    revokeOpen.value = false
    await load()
  } catch (err) {
    showToast(`吊销失败：${(err as Error).message}`, 'error')
  } finally {
    revoking.value = false
  }
}

watch(keyword, (value) => {
  if (value === '') {
    page.value = 1
    void load()
  }
})

onMounted(async () => {
  await loadClients()
  await load()
})
</script>

<template>
  <div class="openapi-page">
    <v-card variant="outlined">
      <v-card-title class="d-flex align-center flex-wrap ga-2">
        <v-icon icon="mdi-key-chain" class="mr-1" />
        OpenAPI 凭证
        <v-chip size="small" variant="tonal" color="primary">open.open_api_credential</v-chip>
        <v-chip size="small" variant="tonal">共 {{ total }} 条</v-chip>
      </v-card-title>
      <v-card-subtitle class="pb-2">
        一行 = 一种认证模式的密钥。密钥只在<strong>签发 / 轮换</strong>时显示一次，之后任何界面都不回显。
      </v-card-subtitle>

      <v-card-text>
        <div class="d-flex flex-wrap align-center ga-2 mb-3">
          <v-select
            v-model="clientIDN"
            :items="clientOptions"
            item-title="title"
            item-value="value"
            label="调用方"
            density="compact"
            hide-details
            style="min-width: 220px; max-width: 280px"
            @update:model-value="clientIDN = Number($event); page = 1; load()"
          />
          <v-text-field
            v-model="keyword"
            label="关键字（凭证号 / 公开标识）"
            density="compact"
            hide-details
            clearable
            style="min-width: 200px; max-width: 280px"
            @keyup.enter="page = 1; load()"
          />
          <v-select
            v-model="status"
            :items="STATUS_OPTIONS"
            item-title="title"
            item-value="value"
            :return-object="false"
            label="状态"
            density="compact"
            hide-details
            style="min-width: 260px; max-width: 320px"
            @update:model-value="status = String($event); page = 1; load()"
          />
          <v-btn color="primary" variant="tonal" prepend-icon="mdi-magnify" @click="page = 1; load()">查询</v-btn>
          <v-btn color="primary" prepend-icon="mdi-key-plus" @click="openCreate">签发凭证</v-btn>
        </div>

        <v-table density="comfortable" class="openapi-table">
          <thead>
            <tr>
              <th>IDN</th>
              <th>凭证号</th>
              <th>调用方</th>
              <th>认证模式</th>
              <th>公开标识</th>
              <th>授权范围</th>
              <th>状态</th>
              <th>到期日期</th>
              <th>最近使用</th>
              <th>累计调用</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.idn">
              <td class="text-no-wrap text-caption">{{ row.idn }}</td>
              <td class="font-weight-medium">{{ row.credential_no }}</td>
              <td>{{ row.client_code }}</td>
              <td>{{ authModeText(row.auth_mode) }}</td>
              <td>
                <code>{{ row.key_prefix }}…</code>
                <TextPopover
                  title="公开标识（完整值）"
                  :lines="[
                    row.access_key_id,
                    `密钥材料：${row.has_secret ? '已写入（不可回显）' : '缺失'}`,
                    `签名算法：${row.sign_algorithm}`,
                  ]"
                />
              </td>
              <td>
                <span v-if="!row.scope || row.scope.length === 0" class="text-medium-emphasis">全部接口</span>
                <template v-else>
                  <span>{{ row.scope.length }} 条</span>
                  <TextPopover title="授权范围（完整）" :lines="row.scope" />
                </template>
              </td>
              <td>
                <v-chip size="small" variant="tonal" :color="statusColor(row)">{{ statusText(row) }}</v-chip>
                <TextPopover
                  v-if="row.flag_revoke === '1'"
                  title="吊销信息"
                  :lines="[`吊销时间：${timeText(row.dt_revoke)}`, `原因：${row.reason_revoke || '—'}`]"
                />
              </td>
              <td class="text-no-wrap">
                <span v-if="!row.dt_expire" class="text-medium-emphasis">长期有效</span>
                <span v-else>{{ expireText(row.dt_expire) }}</span>
              </td>
              <td>{{ timeText(row.dt_last_use) }}</td>
              <td>{{ row.count_use }}</td>
              <td class="text-no-wrap">
                <template v-if="row.flag_revoke !== '1'">
                  <v-btn size="small" variant="text" @click="toggle(row)">
                    {{ row.flag_enable === '1' ? '停用' : '启用' }}
                  </v-btn>
                  <v-btn size="small" variant="text" @click="openScope(row)">授权范围</v-btn>
                  <v-btn size="small" variant="text" @click="openExpire(row)">有效期</v-btn>
                  <v-btn size="small" variant="text" color="primary" @click="openRotate(row)">轮换</v-btn>
                  <v-btn size="small" variant="text" color="error" @click="openRevoke(row)">吊销</v-btn>
                </template>
                <span v-else class="text-medium-emphasis text-caption">已吊销，只能新建</span>
              </td>
            </tr>
            <tr v-if="rows.length === 0">
              <td colspan="11" class="text-center text-medium-emphasis py-6">
                {{ loading ? '加载中…' : '暂无凭证，点「签发凭证」生成密钥' }}
              </td>
            </tr>
          </tbody>
        </v-table>

        <div class="d-flex align-center justify-end ga-2 mt-3">
          <v-pagination v-model="page" :length="pageCount" density="compact" total-visible="7" @update:model-value="load" />
        </div>
      </v-card-text>
    </v-card>

    <v-card class="mt-4" variant="outlined">
      <v-card-title class="text-subtitle-1">
        <v-icon icon="mdi-help-circle-outline" class="mr-1" />
        说明
      </v-card-title>
      <v-card-text class="text-body-2">
        <p>
          ① <strong>凭证号 / AK / SK / 静态 Key 的英文字母一律大写</strong>（便于人工抄写核对），
          形如 <code>CRED-CDEU-01</code>、<code>FK_LIVE_…</code>、<code>AK_LIVE_…</code>、<code>SK_LIVE_…</code>；
          调用方手写小写也能匹配（服务端会按大写归一后比对）。
        </p>
        <p>
          ② <strong>模式一 静态 Key</strong>：密钥形如 <code>FK_LIVE_</code> + 32 位随机串，请求带
          <code>X-API-Key</code>（或 <code>Authorization: Bearer</code>）；库里只存 <code>sha256</code> 哈希。
        </p>
        <p>
          ③ <strong>模式二 AK/SK</strong>：AK 形如 <code>AK_LIVE_</code> + 32 位随机串（公开标识），
          SK 形如 <code>SK_LIVE_</code> + 48 位随机串；请求带 <code>Authorization: FinvAK &lt;AK&gt;:&lt;签名&gt;</code> +
          <code>X-Finv-Timestamp</code> + <code>X-Finv-Nonce</code>。SK 以 AES-256-GCM 密文存储，
          加密密钥取系统配置 <code>19005</code>（未配置时签发会返回 503）。
        </p>
        <p>
          ④ <strong>状态语义</strong>：停用可逆（不改密钥）；吊销不可逆，必须填写原因并留痕，
          之后只能新建凭证；调用方停用时其全部凭证一并失效。
        </p>
        <p>
          ⑤ <strong>轮换</strong>：静态 Key 的公开标识会改变（旧 Key 立即失效）；AK/SK 的旧 SK 在签名时间窗内仍可验签，
          窗口过后失效，便于调用方无感切换。
        </p>
        <p>⑤ 授权范围留空 = 该路由组全部接口；每条写成「方法 路径」，路径支持尾部 <code>/*</code> 前缀匹配。</p>
      </v-card-text>
    </v-card>

    <OpenAPICredentialDialog
      v-model="dialogOpen"
      :mode="dialogMode"
      :client-idn="dialogClientIDN"
      :credential-idn="dialogCredentialIDN"
      :credential-no="dialogCredentialNo"
      :auth-mode="dialogAuthMode"
      :clients="clients"
      @saved="load"
    />

    <!-- 改授权范围（需求 11230）：只改 scope，提交后立即生效 -->
    <v-dialog v-model="scopeOpen" max-width="720">
      <v-card>
        <v-card-title class="text-subtitle-1">修改授权范围</v-card-title>
        <v-card-subtitle>
          {{ scopeTarget?.credential_no }}（{{ authModeText(scopeTarget?.auth_mode ?? '') }}）——
          只改授权范围，不动密钥、IP 白名单与有效期；提交后立即生效（鉴权缓存同步失效）。
        </v-card-subtitle>
        <v-card-text>
          <v-select
            v-model="scopePicked"
            :items="endpointOptions"
            item-title="title"
            item-value="value"
            :return-object="false"
            :multiple="scopeTarget?.auth_mode !== 'APIKEY'"
            :chips="scopeTarget?.auth_mode !== 'APIKEY'"
            :closable-chips="scopeTarget?.auth_mode !== 'APIKEY'"
            label="授权接口（选项来自「OpenAPI 接口清单」）"
            persistent-hint
            :hint="scopeHint"
            density="compact"
            variant="outlined"
            class="mb-3"
          />
          <v-textarea
            v-model="scopeExtra"
            label="补充规则（每行一条「方法 路径」，通配用尾部 /*，可留空）"
            placeholder="GET /OpenAPI/V1/Meta/Finv/Quote/Cdeu/*"
            density="compact"
            variant="outlined"
            rows="2"
            persistent-hint
            hint="通配规则无法枚举，只能手输；下拉选不到的现有规则会自动填到这里"
          />
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="scopeOpen = false">取消</v-btn>
          <v-btn color="primary" :loading="scopeSaving" @click="confirmScope">保存</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- 改有效期（需求 11246）：只改 dt_expire，提交后立即生效 -->
    <v-dialog v-model="expireOpen" max-width="520">
      <v-card>
        <v-card-title class="text-subtitle-1">修改有效期</v-card-title>
        <v-card-subtitle>
          {{ expireTarget?.credential_no }}（{{ authModeText(expireTarget?.auth_mode ?? '') }}）——
          只改有效期，不动密钥、授权范围与 IP 白名单；提交后立即生效（鉴权缓存同步失效）。
        </v-card-subtitle>
        <v-card-text>
          <v-text-field
            v-model="expireValue"
            type="date"
            :label="expireTarget?.auth_mode === 'APIKEY'
              ? '到期日（必填，最长 90 天）'
              : '到期日（留空 = 长期）'"
            :error-messages="expireInvalid ? [expireInvalid] : []"
            density="compact"
            variant="outlined"
            persistent-hint
            hint="当前值取自列表的「到期」列；保存后列表刷新"
          />
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="expireOpen = false">取消</v-btn>
          <v-btn color="primary" :loading="expireSaving" :disabled="expireInvalid !== ''" @click="confirmExpire">
            保存
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="revokeOpen" max-width="520">
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
          <v-btn variant="text" @click="revokeOpen = false">取消</v-btn>
          <v-btn color="error" :loading="revoking" :disabled="revokeReason.trim() === ''" @click="confirmRevoke">
            确认吊销
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<style scoped>
.openapi-page {
  padding: 16px;
}

.openapi-table :deep(th),
.openapi-table :deep(td) {
  white-space: nowrap;
}
</style>
