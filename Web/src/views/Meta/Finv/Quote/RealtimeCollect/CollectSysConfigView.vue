<script setup lang="ts">
// 行情采集系统配置（需求 11343）
//
// 统一维护 sys.sys_config 里**与实时行情采集相关**的 5 项配置（只查 + 改，不新增/删除）：
//   20013  Supabase 数据库连接配置（敏感：列表掩码，👁 查看明文）
//   20014  Supabase 证券键源表名
//   20015  USC 映射规则每日同步 cron（6 段）
//   20016  moomoo OpenAPI 凭据（敏感：列表掩码，👁 查看明文）
//   20018  moomoo 网页接口代理配置
//
// 20017 已在 V300228 合并进 20016 后删除；20012（登录会话）与 20002–20011（归档上传 /
// CDEU 运行清理）刻意不在本页——它们属于认证域与行情数据归档域。
//
// 接口（前缀 /Meta/Finv/Quote/Collect/Config）：
//   GET  /List     列表（敏感项只回掩码 + has_value）
//   POST /Save     保存单条（20016 私钥留空 = 保留原值，__CLEAR__ = 只清私钥）
//   GET  /Secret   取敏感项明文（后端返回 base64，前端解码后弹窗展示）
//
// 旧入口保留双入口：行情快照同步页的 moomoo 凭据卡片、证券参数同步页的源表名、
// USC 映射规则页的 cron 卡片仍可改同一批配置项，改完这里刷新即可看到。
import { computed, onMounted, ref } from 'vue'

import { apiGet, apiPost } from '../../../../../api'
import { showToast } from '../../../../../ui/toast'

const BASE = '/Meta/Finv/Quote/Collect/Config'

interface ConfigItem {
  config_no: number
  name: string
  module: string
  sensitive: boolean
  value_format: string
  description: string
  empty_hint: string
  value: string
  masked: boolean
  has_value: boolean
  gmt_modify: string | null
}

const items = ref<ConfigItem[]>([])
const loading = ref(false)
const loadError = ref('')

// 编辑弹窗
const editorOpen = ref(false)
const editorTarget = ref<ConfigItem | null>(null)
const editorValue = ref('')
const saving = ref(false)

// 明文查看弹窗（敏感项）
const secretOpen = ref(false)
const secretTarget = ref<ConfigItem | null>(null)
const secretText = ref('')
const secretLoading = ref(false)
const secretVisible = ref(false)

const modules = computed(() => {
  const out: { name: string; rows: ConfigItem[] }[] = []
  for (const item of items.value) {
    let group = out.find((g) => g.name === item.module)
    if (!group) {
      group = { name: item.module, rows: [] }
      out.push(group)
    }
    group.rows.push(item)
  }
  return out
})

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    const data = await apiGet<{ items: ConfigItem[] }>(`${BASE}/List`)
    items.value = data.items ?? []
  } catch (err) {
    loadError.value = (err as Error).message
    showToast(`读取行情采集系统配置失败：${(err as Error).message}`, 'error')
  } finally {
    loading.value = false
  }
}

function openEditor(item: ConfigItem) {
  editorTarget.value = item
  // 敏感项编辑框留空：不把掩码带进输入框，避免误保存
  editorValue.value = item.sensitive ? '' : item.value
  editorOpen.value = true
}

function editorHint(item: ConfigItem | null): string {
  if (!item) return ''
  const hint = `${item.value_format}；${item.description}`
  return item.empty_hint ? `${hint}（${item.empty_hint}）` : hint
}

async function save() {
  const item = editorTarget.value
  if (!item) return
  saving.value = true
  try {
    await apiPost(`${BASE}/Save`, { config_no: item.config_no, value: editorValue.value })
    showToast(`${item.name} 已保存`, 'success')
    editorOpen.value = false
    await load()
  } catch (err) {
    showToast(`保存失败：${(err as Error).message}`, 'error')
  } finally {
    saving.value = false
  }
}

async function openSecret(item: ConfigItem) {
  secretTarget.value = item
  secretText.value = ''
  secretVisible.value = false
  secretOpen.value = true
  secretLoading.value = true
  try {
    const data = await apiGet<{ value_base64: string; length: number }>(
      `${BASE}/Secret?configNo=${item.config_no}`,
    )
    secretText.value = decodeBase64Utf8(data.value_base64)
    if (!shownOnce(item.config_no)) {
      showToast('明文仅本次展示，请勿截图或粘贴到公开位置', 'warning')
    }
  } catch (err) {
    showToast(`读取明文失败：${(err as Error).message}`, 'error')
    secretOpen.value = false
  } finally {
    secretLoading.value = false
  }
}

// 每次会话对同一配置项只提示一次（避免反复点 👁 反复弹警告）
const warned = new Set<number>()
function shownOnce(configNo: number): boolean {
  if (warned.has(configNo)) return true
  warned.add(configNo)
  return false
}

/** base64(UTF-8) → 字符串（后端返回 base64，避免明文直接出现在响应体里被日志留存）。 */
function decodeBase64Utf8(raw: string): string {
  try {
    const binary = atob(raw)
    const bytes = Uint8Array.from(binary, (ch) => ch.charCodeAt(0))
    return new TextDecoder('utf-8').decode(bytes)
  } catch {
    return raw
  }
}

function formatTime(value: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

function valueCell(item: ConfigItem): string {
  if (!item.has_value) return '（未配置）'
  return item.sensitive ? item.value || '****' : item.value
}

onMounted(load)
</script>

<template>
  <v-card>
    <v-card-title class="d-flex align-center flex-wrap ga-2">
      <v-icon icon="mdi-cog-sync-outline" class="mr-2" />
      行情采集系统配置
      <v-chip size="small" variant="tonal" prepend-icon="mdi-format-list-numbered">
        {{ items.length }} 项
      </v-chip>
      <v-chip size="small" variant="tonal" prepend-icon="mdi-lock-outline">
        敏感项 {{ items.filter((i) => i.sensitive).length }}
      </v-chip>
      <v-spacer />
      <v-btn variant="text" prepend-icon="mdi-refresh" :loading="loading" @click="load">刷新</v-btn>
    </v-card-title>

    <v-card-text>
      <v-alert v-if="loadError" type="error" density="compact" variant="tonal" class="mb-3">
        读取失败：{{ loadError }}
      </v-alert>

      <v-alert type="info" density="compact" variant="tonal" class="mb-4">
        本页只维护 <strong>实时行情采集相关</strong>的系统配置（20013–20018，含敏感项的掩码与明文查看），
        只支持<strong>查询与修改</strong>，不新增、不删除配置项。
        旧入口（行情快照同步页的 moomoo 凭据、证券参数同步页的源表名、USC 映射规则页的 cron）
        本期保留双入口，改完刷新本页即可看到同一份值。
      </v-alert>

      <div v-for="group in modules" :key="group.name" class="mb-4">
        <div class="text-subtitle-2 mb-2">
          <v-icon icon="mdi-folder-cog-outline" size="small" class="mr-1" />{{ group.name }}
        </div>
        <v-table density="compact" class="text-caption">
          <thead>
            <tr>
              <th class="text-no-wrap" style="width: 70px">编号</th>
              <th class="text-no-wrap" style="width: 210px">配置项</th>
              <th style="min-width: 260px">当前值</th>
              <th style="min-width: 320px">值格式 / 说明</th>
              <th class="text-no-wrap" style="width: 160px">最近修改</th>
              <th class="text-no-wrap" style="width: 150px">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in group.rows" :key="item.config_no">
              <td class="text-no-wrap">{{ item.config_no }}</td>
              <td>
                <div class="text-no-wrap">{{ item.name }}</div>
                <v-chip
                  v-if="item.sensitive"
                  size="x-small"
                  variant="tonal"
                  color="warning"
                  prepend-icon="mdi-lock-outline"
                  class="mt-1"
                >
                  敏感
                </v-chip>
              </td>
              <td>
                <code class="finv-value">{{ valueCell(item) }}</code>
                <div v-if="item.sensitive && !item.has_value" class="text-medium-emphasis">
                  未配置
                </div>
              </td>
              <td>
                <div><code>{{ item.value_format }}</code></div>
                <div class="text-medium-emphasis">{{ item.description }}</div>
                <div v-if="item.empty_hint" class="text-medium-emphasis">
                  留空：{{ item.empty_hint }}
                </div>
              </td>
              <td class="text-no-wrap">{{ formatTime(item.gmt_modify) }}</td>
              <td>
                <v-btn
                  v-if="item.sensitive"
                  size="small"
                  variant="text"
                  prepend-icon="mdi-eye-outline"
                  :disabled="!item.has_value"
                  @click="openSecret(item)"
                >
                  查看
                </v-btn>
                <v-btn
                  size="small"
                  variant="text"
                  prepend-icon="mdi-pencil"
                  @click="openEditor(item)"
                >
                  修改
                </v-btn>
              </td>
            </tr>
          </tbody>
        </v-table>
      </div>

      <div v-if="!loading && !items.length" class="text-medium-emphasis py-4 text-center">
        暂无配置项（请确认已执行 20013–20018 的种子迁移）
      </div>
    </v-card-text>
  </v-card>

  <!-- 修改弹窗 -->
  <v-dialog v-model="editorOpen" max-width="720" persistent>
    <v-card>
      <v-card-title class="text-subtitle-1">
        修改配置 {{ editorTarget?.config_no }} · {{ editorTarget?.name }}
      </v-card-title>
      <v-card-text>
        <v-alert
          v-if="editorTarget?.sensitive"
          type="warning"
          density="compact"
          variant="tonal"
          class="mb-3"
        >
          敏感项：留空表示不修改（填写才覆盖）。明文不会回显在输入框里，需要核对请用「查看」。
          <template v-if="editorTarget?.config_no === 20016">
            私钥留空 = 保留原私钥；填 <code>__CLEAR__</code> = 只清私钥（保留 AppKey）。
          </template>
        </v-alert>
        <div class="text-caption text-medium-emphasis mb-2">{{ editorHint(editorTarget) }}</div>
        <v-textarea
          v-model="editorValue"
          variant="outlined"
          density="compact"
          rows="3"
          hide-details
          class="finv-editor"
          :placeholder="editorTarget?.value_format"
        />
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" :disabled="saving" @click="editorOpen = false">取消</v-btn>
        <v-btn color="primary" variant="flat" :loading="saving" @click="save">保存</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- 明文查看弹窗（敏感项，后端 base64 返回） -->
  <v-dialog v-model="secretOpen" max-width="760">
    <v-card>
      <v-card-title class="text-subtitle-1">
        明文 {{ secretTarget?.config_no }} · {{ secretTarget?.name }}
      </v-card-title>
      <v-card-text>
        <v-alert type="warning" density="compact" variant="tonal" class="mb-3">
          含凭据/密钥，仅在排障时查看；请勿截图、勿粘贴到公开位置。后端以 base64 返回，前端本地解码。
        </v-alert>
        <v-progress-linear v-if="secretLoading" indeterminate class="mb-3" />
        <template v-else>
          <div class="d-flex align-center ga-2 mb-2">
            <v-btn
              size="small"
              variant="tonal"
              :prepend-icon="secretVisible ? 'mdi-eye-off-outline' : 'mdi-eye-outline'"
              @click="secretVisible = !secretVisible"
            >
              {{ secretVisible ? '隐藏' : '显示' }}
            </v-btn>
            <span class="text-caption text-medium-emphasis">
              长度 {{ secretText.length }} 字符
            </span>
          </div>
          <v-textarea
            v-if="secretVisible"
            :model-value="secretText"
            variant="outlined"
            density="compact"
            rows="6"
            readonly
            hide-details
            class="finv-secret"
          />
          <div v-else class="text-medium-emphasis">已隐藏（点击「显示」查看）</div>
        </template>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="secretOpen = false">关闭</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.finv-value {
  white-space: nowrap;
}

.finv-editor :deep(textarea),
.finv-secret :deep(textarea) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 0.78rem;
}
</style>
