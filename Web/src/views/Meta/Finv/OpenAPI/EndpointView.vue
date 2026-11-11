<script setup lang="ts">
// 开放平台 → 安全设置 → 接口清单（需求 11223）
//
// 表：open.open_api_endpoint（一行 = 一个可调接口的目录信息）
// 接口：GET  /Meta/Finv/OpenAPI/Endpoint/Page     分页查询
//       POST /Meta/Finv/OpenAPI/Endpoint/Flags    切换 flag_public / flag_enable
//
// **本页只读路径与方法**：允许调哪些接口由后端 router.go 的路由注册决定，
// 本表是「目录 + 元数据」——给授权范围下拉提供可选项、给审计报表提供 api_code 锚点。
// 所以页面上不提供「新增接口 / 改路径」的操作，避免出现「表里有、路由没有」的幽灵接口。
import { computed, onMounted, ref, watch } from 'vue'

import { apiGet, apiPost } from '../../../../api'
import TextPopover from '../../../../components/TextPopover.vue'
import { showToast } from '../../../../ui/toast'
import { OPENAPI_BASE as BASE, timeText } from './shared'

interface EndpointRow {
  idn: number
  api_code: string
  catalog: string
  func: string
  http_method: string
  path: string
  api_name: string
  api_group: string
  flag_write: string
  flag_public: string
  flag_enable: string
  dt_deprecate: string
  remark: string
  gmt_update: string
  count_grant: number
}

const rows = ref<EndpointRow[]>([])
const total = ref(0)
const keyword = ref('')
const catalog = ref('')
const catalogOptions = ref<{ title: string; value: string }[]>([{ title: '全部目录', value: '' }])
const apiGroup = ref('')
const flagPublic = ref('')
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)

const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

// 分组下拉从当前结果里归纳（接口数量很少，不额外加「分组列表」接口）
const groupOptions = ref<{ title: string; value: string }[]>([{ title: '全部分组', value: '' }])

const PUBLIC_OPTIONS = [
  { title: '全部（含内部）', value: '' },
  { title: '对外公开[1]', value: '1' },
  { title: '内部保留[0]', value: '0' },
]

async function load() {
  loading.value = true
  try {
    const params = new URLSearchParams({ page: String(page.value), pageSize: String(pageSize.value) })
    if (keyword.value.trim()) params.set('keyword', keyword.value.trim())
    if (catalog.value) params.set('catalog', catalog.value)
    if (apiGroup.value) params.set('apiGroup', apiGroup.value)
    if (flagPublic.value) params.set('flagPublic', flagPublic.value)
    const data = await apiGet<{ total: number; list: EndpointRow[] }>(
      `${BASE}/Endpoint/Page?${params.toString()}`,
    )
    rows.value = data.list ?? []
    total.value = data.total ?? 0
    const catalogs = new Set((data.list ?? []).map((row) => row.catalog).filter(Boolean))
    if (catalogs.size > 0) {
      catalogOptions.value = [
        { title: '全部目录', value: '' },
        ...[...catalogs].map((item) => ({ title: item, value: item })),
      ]
    }
    const groups = new Set((data.list ?? []).map((row) => row.api_group).filter(Boolean))
    if (groups.size > 0) {
      groupOptions.value = [
        { title: '全部分组', value: '' },
        ...[...groups].map((group) => ({ title: group, value: group })),
      ]
    }
  } catch (err) {
    rows.value = []
    total.value = 0
    showToast(`查询接口清单失败：${(err as Error).message}`, 'error')
  } finally {
    loading.value = false
  }
}

// ---- 列宽可调（拖拽表头右边缘 / 双击自适应 / 记住宽度）----
//
// 「路径」与「中文名」这两列内容长度差异很大（路径可达 60+ 字符，中文名 6~8 字），
// 自动布局会把它们挤在一起、互相压字，所以改成固定布局 + 手动可调（需求 11229）。
interface Column {
  key: string
  title: string
  width: number // 默认宽度（px）
  min: number   // 最小宽度（px）
}

const COLUMNS: Column[] = [
  { key: 'idn', title: 'IDN', width: 70, min: 50 },
  { key: 'api_code', title: '接口编码', width: 230, min: 140 },
  { key: 'catalog', title: '目录', width: 180, min: 120 },
  { key: 'func', title: '功能', width: 130, min: 90 },
  { key: 'http_method', title: '方法', width: 80, min: 60 },
  { key: 'path', title: '路径', width: 360, min: 160 },
  { key: 'api_name', title: '中文名', width: 160, min: 110 },
  { key: 'api_group', title: '分组', width: 110, min: 80 },
  { key: 'flag_write', title: '读写', width: 70, min: 56 },
  { key: 'count_grant', title: '已授权凭证', width: 110, min: 80 },
  { key: 'flag_public', title: '公开', width: 92, min: 72 },
  { key: 'flag_enable', title: '启用', width: 92, min: 72 },
  { key: 'gmt_update', title: '更新时间', width: 180, min: 140 },
]

const WIDTHS_KEY = 'finv.openapi.endpoint.widths'
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
  resizing = { key: col.key, startX: event.clientX, startWidth: colWidths.value[col.key] }
  document.addEventListener('mousemove', onResizing)
  document.addEventListener('mouseup', stopResize)
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'
}

function onResizing(event: MouseEvent) {
  if (!resizing) return
  const col = COLUMNS.find((item) => item.key === resizing!.key)
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
    widest = Math.max(widest, ctx.measureText(cellText(row, col)).width + 28)
  }
  colWidths.value[col.key] = Math.max(col.min, Math.min(1200, Math.ceil(widest)))
  saveWidths()
}

function cellText(row: EndpointRow, col: Column): string {
  switch (col.key) {
    case 'flag_write': return row.flag_write === '1' ? '写' : '读'
    case 'count_grant': return String(row.count_grant)
    case 'flag_public': return row.flag_public === '1' ? '开启' : '关闭'
    case 'flag_enable': return row.flag_enable === '1' ? '启用' : '停用'
    case 'gmt_update': return timeText(row.gmt_update)
    default: return String((row as unknown as Record<string, unknown>)[col.key] ?? '')
  }
}

// 两列开关：写的是同一张表，就一个接口（PATCH 语义用 POST 表达，与其它页面一致）
async function saveFlag(row: EndpointRow, field: 'flag_public' | 'flag_enable', next: boolean) {
  const value = next ? '1' : '0'
  try {
    await apiPost(`${BASE}/Endpoint/Flags`, { idn: row.idn, [field]: value })
    row[field] = value
    const label = field === 'flag_public' ? '公开' : '启用'
    showToast(`已${next ? '开启' : '关闭'}「${row.api_code}」的${label}标志`, 'success')
  } catch (err) {
    showToast(`更新失败：${(err as Error).message}`, 'error')
    await load()
  }
}

watch(keyword, (value) => {
  if (value === '') {
    page.value = 1
    void load()
  }
})

onMounted(() => {
  loadWidths()
  void load()
})
</script>

<template>
  <div class="openapi-page">
    <v-card variant="outlined">
      <v-card-title class="d-flex align-center flex-wrap ga-2">
        <v-icon icon="mdi-api" class="mr-1" />
        OpenAPI 接口清单
        <v-chip size="small" variant="tonal" color="primary">open.open_api_endpoint</v-chip>
        <v-chip size="small" variant="tonal">共 {{ total }} 条</v-chip>
      </v-card-title>
      <v-card-subtitle class="pb-2">
        一行 = 一个可调接口的目录信息（编码 / 方法 / 路径 / 读写 / 公开与启用标志）。
        <strong>本页不决定「能调什么」</strong>——那由后端路由注册决定，这里只给授权范围下拉做选项、给审计报表提供稳定的
        <code>api_code</code> 锚点。
      </v-card-subtitle>

      <v-card-text>
        <div class="d-flex flex-wrap align-center ga-2 mb-3">
          <v-text-field
            v-model="keyword"
            label="关键字（编码 / 路径 / 中文名）"
            density="compact"
            hide-details
            clearable
            style="min-width: 220px; max-width: 300px"
            @keyup.enter="page = 1; load()"
          />
          <v-select
            v-model="catalog"
            :items="catalogOptions"
            item-title="title"
            item-value="value"
            :return-object="false"
            label="目录"
            density="compact"
            hide-details
            style="min-width: 190px; max-width: 240px"
            @update:model-value="catalog = String($event); page = 1; load()"
          />
          <v-select
            v-model="apiGroup"
            :items="groupOptions"
            item-title="title"
            item-value="value"
            :return-object="false"
            label="分组"
            density="compact"
            hide-details
            style="min-width: 160px; max-width: 200px"
            @update:model-value="apiGroup = String($event); page = 1; load()"
          />
          <v-select
            v-model="flagPublic"
            :items="PUBLIC_OPTIONS"
            item-title="title"
            item-value="value"
            :return-object="false"
            label="公开"
            density="compact"
            hide-details
            style="min-width: 170px; max-width: 210px"
            @update:model-value="flagPublic = String($event); page = 1; load()"
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
          <v-table density="comfortable" class="openapi-table openapi-table-fixed">
            <colgroup>
              <col v-for="col in COLUMNS" :key="col.key" :style="{ width: colWidths[col.key] + 'px' }" />
            </colgroup>
            <thead>
              <tr>
                <th v-for="col in COLUMNS" :key="col.key" class="openapi-th">
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
            <!-- tbody 的 td 顺序必须与上面 COLUMNS 一一对应（colgroup 按列宽生效） -->
            <tbody>
              <tr v-for="row in rows" :key="row.idn">
                <td class="cell-nowrap text-caption">{{ row.idn }}</td>
                <td class="cell-ellipsis">
                  <code>{{ row.api_code }}</code>
                  <TextPopover
                    v-if="row.remark || row.dt_deprecate"
                    title="备注 / 下线信息"
                    :lines="[
                      `备注：${row.remark || '—'}`,
                      `下线时间：${row.dt_deprecate ? timeText(row.dt_deprecate) : '在服务'}`,
                    ]"
                  />
                </td>
                <td class="cell-ellipsis" :title="row.catalog"><code>{{ row.catalog }}</code></td>
                <td class="cell-ellipsis" :title="row.func"><code>{{ row.func }}</code></td>
                <td class="cell-nowrap">
                  <v-chip size="x-small" variant="tonal" :color="row.flag_write === '1' ? 'warning' : 'info'">
                    {{ row.http_method }}
                  </v-chip>
                </td>
                <td class="cell-ellipsis">
                  <span class="mono">{{ row.path }}</span>
                  <TextPopover v-if="row.path" title="完整路径" :text="row.path" />
                </td>
                <td class="cell-ellipsis" :title="row.api_name">{{ row.api_name }}</td>
                <td class="cell-nowrap">{{ row.api_group || '—' }}</td>
                <td class="cell-nowrap">
                  <v-chip v-if="row.flag_write === '1'" size="x-small" variant="tonal" color="warning">写</v-chip>
                  <v-chip v-else size="x-small" variant="tonal" color="info">读</v-chip>
                </td>
                <td class="cell-nowrap">
                  <span :class="row.count_grant > 0 ? 'font-weight-medium' : 'text-medium-emphasis'">
                    {{ row.count_grant }}
                  </span>
                </td>
                <td class="cell-nowrap">
                  <div class="d-flex justify-center">
                    <v-switch
                      :model-value="row.flag_public === '1'"
                      color="primary"
                      density="compact"
                      hide-details
                      :disabled="!!row.dt_deprecate"
                      @update:model-value="saveFlag(row, 'flag_public', Boolean($event))"
                    />
                  </div>
                </td>
                <td class="cell-nowrap">
                  <div class="d-flex justify-center">
                    <v-switch
                      :model-value="row.flag_enable === '1'"
                      color="success"
                      density="compact"
                      hide-details
                      @update:model-value="saveFlag(row, 'flag_enable', Boolean($event))"
                    />
                  </div>
                </td>
                <td class="cell-nowrap">{{ timeText(row.gmt_update) }}</td>
              </tr>
              <tr v-if="rows.length === 0">
                <td :colspan="COLUMNS.length" class="text-center text-medium-emphasis py-6">
                  {{ loading ? '加载中…' : '暂无接口（清单由迁移 V300193 登记，新增接口需先注册路由再补种子）' }}
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

    <v-card class="mt-4" variant="outlined">
      <v-card-title class="text-subtitle-1">
        <v-icon icon="mdi-help-circle-outline" class="mr-1" />
        说明
      </v-card-title>
      <v-card-text class="text-body-2">
        <p>
          ① <strong>这不是白名单</strong>：请求能不能调某个接口，取决于该接口是否在后端路由里注册；本表只描述已注册的接口，
          用来给「授权范围」下拉提供选项、给审计报表把路径翻译成稳定的 <code>api_code</code>。
        </p>
        <p>
          ② <strong>公开</strong>（<code>flag_public</code>）：开启后才会出现在凭证的「授权范围」下拉里；
          内部接口关掉它可以避免被误授权。
        </p>
        <p>
          ③ <strong>启用</strong>（<code>flag_enable</code>）：停用只是「不再推荐授权」，
          <strong>已经授权过的凭证仍然可以调用</strong>——要真正收回权限必须改凭证的授权范围或吊销凭证。
        </p>
        <p>
          ④ <strong>读写</strong>（<code>flag_write</code>）：写接口用静态 Key 调用时必须带查询参数
          <code>_</code>（毫秒时间戳，±60 秒），否则 <code>19006</code>。
        </p>
        <p>
          ⑤ <strong>目录 / 功能</strong>：<code>catalog</code> 取 <code>api_code</code> <strong>最后一个点之前</strong>的部分
          （如 <code>finv.quote.cdeu.min</code>）；<code>func</code> 取 <code>path</code> <strong>最后一个斜杠之后</strong>的部分
          （大驼峰，如 <code>PageUni</code> / <code>FTStockUpsert</code>）。
          两者都是生成列、<strong>不可手工写入</strong>：catalog 由编码派生、func 由路径派生，且 func 有
          <code>^[A-Z][A-Za-z0-9]*$</code> 约束——路径末段不是大驼峰会直接写入失败。
        </p>
        <p>
          ⑥ <strong>已授权凭证</strong>：当前有多少个凭证的授权范围里包含这个接口（来自
          <code>open.open_api_credential_scope</code>；该表尚未接入读写，数值仅供参考）。
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

/* 单元格：不换行；宽度不够时才出省略号（可拖宽或点 ⓘ 看全文） */
.cell-nowrap {
  white-space: nowrap;
}

.cell-ellipsis {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* 路径用等宽字体，便于逐字符核对 */
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
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
</style>
