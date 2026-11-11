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
import { showToast } from '../../../../ui/toast'
import OpenAPICredentialDialog from './CredentialDialog.vue'
import {
  OPENAPI_BASE as BASE,
  authModeText,
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

async function load() {
  loading.value = true
  try {
    const params = new URLSearchParams({ page: String(page.value), pageSize: String(pageSize.value) })
    if (clientIDN.value > 0) params.set('clientId', String(clientIDN.value))
    if (keyword.value.trim()) params.set('keyword', keyword.value.trim())
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

function openCreate() {
  if (clients.value.length === 0) {
    showToast('请先在「OpenAPI 调用方」页登记调用方，再签发凭证', 'warning')
    return
  }
  dialogMode.value = 'create'
  dialogClientIDN.value = clientIDN.value || clients.value[0].idn
  dialogCredentialIDN.value = 0
  dialogOpen.value = true
}

function openRotate(row: CredentialRow) {
  if (!window.confirm(`确认轮换凭证「${row.credential_no}」的密钥？旧密钥立即失效（AK/SK 旧 SK 仅窗口内可验签）。`)) return
  dialogMode.value = 'rotate'
  dialogClientIDN.value = row.client_idn
  dialogCredentialIDN.value = row.idn
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
          <v-btn color="primary" variant="tonal" prepend-icon="mdi-magnify" @click="page = 1; load()">查询</v-btn>
          <v-btn color="primary" prepend-icon="mdi-key-plus" @click="openCreate">签发凭证</v-btn>
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
            <tr v-for="row in rows" :key="row.idn">
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
                  <div>签名算法：{{ row.sign_algorithm }}</div>
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
                  <v-btn size="small" variant="text" @click="toggle(row)">
                    {{ row.flag_enable === '1' ? '停用' : '启用' }}
                  </v-btn>
                  <v-btn size="small" variant="text" color="primary" @click="openRotate(row)">轮换</v-btn>
                  <v-btn size="small" variant="text" color="error" @click="openRevoke(row)">吊销</v-btn>
                </template>
                <span v-else class="text-medium-emphasis text-caption">已吊销，只能新建</span>
              </td>
            </tr>
            <tr v-if="rows.length === 0">
              <td colspan="9" class="text-center text-medium-emphasis py-6">
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
      :clients="clients"
      @saved="load"
    />

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
