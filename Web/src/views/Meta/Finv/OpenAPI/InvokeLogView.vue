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
import { OPENAPI_BASE as BASE, PASS_OPTIONS, REASON_OPTIONS, passColor, passText, reasonText, timeText } from './shared'

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

const rows = ref<InvokeLogRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const flagPass = ref('')
const reasonCode = ref('')
const accessKey = ref('')

const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

async function load() {
  loading.value = true
  try {
    const params = new URLSearchParams({ page: String(page.value), pageSize: String(pageSize.value) })
    if (flagPass.value) params.set('flagPass', flagPass.value)
    if (reasonCode.value) params.set('reasonCode', reasonCode.value)
    if (accessKey.value.trim()) params.set('accessKey', accessKey.value.trim())
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

onMounted(load)
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
          <v-btn color="primary" variant="tonal" prepend-icon="mdi-magnify" @click="page = 1; load()">查询</v-btn>
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
            <tr v-for="row in rows" :key="row.idn">
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
            <tr v-if="rows.length === 0">
              <td colspan="10" class="text-center text-medium-emphasis py-6">
                {{ loading ? '加载中…' : '暂无调用记录（鉴权关闭时不写审计）' }}
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

.openapi-table :deep(th),
.openapi-table :deep(td) {
  white-space: nowrap;
}
</style>
