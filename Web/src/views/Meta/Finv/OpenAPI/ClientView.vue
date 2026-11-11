<script setup lang="ts">
// 开放平台 → OpenAPI 调用方（需求 11173 拆分自原单页）
//
// 表：open.open_api_client（一行 = 一个接入应用）
// 接口：GET  /Meta/Finv/OpenAPI/Client/Page
//       POST /Meta/Finv/OpenAPI/Client/Save     （idn=0 新建，默认停用）
//       POST /Meta/Finv/OpenAPI/Client/Enable
//
// 凭证在「OpenAPI 凭证」页维护；调用审计在「OpenAPI 调用审计」页；鉴权开关在「OpenAPI 鉴权配置」页。
import { computed, onMounted, ref, watch } from 'vue'

import { apiGet, apiPost } from '../../../../api'
import { showToast } from '../../../../ui/toast'
import OpenAPIClientDialog from './ClientDialog.vue'
import { OPENAPI_BASE as BASE, cidrText, limitText, statusColor, statusText, timeText } from './shared'

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

const rows = ref<ClientRow[]>([])
const total = ref(0)
const keyword = ref('')
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)

const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

async function load() {
  loading.value = true
  try {
    const params = new URLSearchParams({ page: String(page.value), pageSize: String(pageSize.value) })
    if (keyword.value.trim()) params.set('keyword', keyword.value.trim())
    const data = await apiGet<{ total: number; list: ClientRow[] }>(`${BASE}/Client/Page?${params.toString()}`)
    rows.value = data.list ?? []
    total.value = data.total ?? 0
  } catch (err) {
    rows.value = []
    total.value = 0
    showToast(`查询调用方失败：${(err as Error).message}`, 'error')
  } finally {
    loading.value = false
  }
}

const dialogOpen = ref(false)
const dialogIDN = ref(0)

function openCreate() {
  dialogIDN.value = 0
  dialogOpen.value = true
}

function openEdit(row: ClientRow) {
  dialogIDN.value = row.idn
  dialogOpen.value = true
}

async function toggle(row: ClientRow) {
  const enable = row.flag_enable !== '1'
  const action = enable ? '启用' : '停用'
  if (!enable && !window.confirm(`确认停用调用方「${row.client_code}」？停用后其全部凭证立即失效。`)) return
  try {
    await apiPost(`${BASE}/Client/Enable`, { idn: row.idn, enable })
    showToast(`调用方已${action}`, 'success')
    await load()
  } catch (err) {
    showToast(`${action}失败：${(err as Error).message}`, 'error')
  }
}

watch(keyword, (value) => {
  if (value === '') {
    page.value = 1
    void load()
  }
})

onMounted(load)
</script>

<template>
  <div class="openapi-page">
    <v-card variant="outlined">
      <v-card-title class="d-flex align-center flex-wrap ga-2">
        <v-icon icon="mdi-account-key-outline" class="mr-1" />
        OpenAPI 调用方
        <v-chip size="small" variant="tonal" color="primary">open.open_api_client</v-chip>
        <v-chip size="small" variant="tonal">共 {{ total }} 个</v-chip>
      </v-card-title>
      <v-card-subtitle class="pb-2">
        一行 = 一个接入应用。新增默认<strong>停用</strong>，核对白名单与配额后再启用；停用会让其全部凭证立即失效。
      </v-card-subtitle>

      <v-card-text>
        <div class="d-flex flex-wrap align-center ga-2 mb-3">
          <v-text-field
            v-model="keyword"
            label="关键字（代号 / 名称）"
            density="compact"
            hide-details
            clearable
            style="min-width: 220px; max-width: 300px"
            @keyup.enter="page = 1; load()"
          />
          <v-btn color="primary" variant="tonal" prepend-icon="mdi-magnify" @click="page = 1; load()">查询</v-btn>
          <v-btn color="primary" prepend-icon="mdi-plus" @click="openCreate">新增调用方</v-btn>
        </div>

        <v-table density="comfortable" class="openapi-table">
          <thead>
            <tr>
              <th>idn</th>
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
            <tr v-for="row in rows" :key="row.idn">
              <td class="text-no-wrap text-caption">{{ row.idn }}</td>
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
                <v-btn size="small" variant="text" @click="openEdit(row)">编辑</v-btn>
                <v-btn
                  size="small"
                  variant="text"
                  :color="row.flag_enable === '1' ? 'warning' : 'success'"
                  @click="toggle(row)"
                >
                  {{ row.flag_enable === '1' ? '停用' : '启用' }}
                </v-btn>
              </td>
            </tr>
            <tr v-if="rows.length === 0">
              <td colspan="11" class="text-center text-medium-emphasis py-6">
                {{ loading ? '加载中…' : '暂无调用方，点「新增调用方」登记第一个' }}
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
          ① 调用方是「谁在调用」的主体，本身不携带密钥；密钥在
          <strong>开放平台 → 接入管理 → OpenAPI 凭证</strong> 页签发，一个调用方可持有多条凭证（可同时用多种认证模式）。
        </p>
        <p>
          ② <strong>调用方的启用状态是总闸</strong>：停用调用方时，其下所有凭证立即失效（无需逐条停用）；
          重新启用后凭证恢复到各自的状态。
        </p>
        <p>
          ③ IP 白名单与凭证白名单<strong>取交集</strong>：两边都填时，来源 IP 必须同时满足；任一边留空表示该侧不限。
        </p>
        <p>
          ④ 速率（次/分）与每日配额用 Redis 计数，<code>0</code> 表示不限；超限的调用返回错误码
          <code>19008</code>，并在「OpenAPI 调用审计」页留下 <code>RATE_LIMIT</code> 记录。
        </p>
        <p>⑤ 授权到期为空表示长期；到期后该调用方的全部凭证按「已过期」拒绝（<code>19003</code>）。</p>
      </v-card-text>
    </v-card>

    <OpenAPIClientDialog v-model="dialogOpen" :client-idn="dialogIDN" @saved="load" />
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
