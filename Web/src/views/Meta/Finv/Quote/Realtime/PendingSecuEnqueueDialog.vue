<script setup lang="ts">
// 待录入证券 →「入队」确认对话框（需求 11153 / 11154）
//
// 两步走、人工确认：
//   1. 列表行点「入队」只打开本对话框，GET /Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Enqueue?stockId=
//      预检「将要写入 futu.finv_futu_secu_key 的字段」与「键表现状」（只读）；
//   2. 人工核对（可改正 instrument）后点「确认入队」，才 POST 同一个地址真正落库。
//
// 入队语义：把证券键登记进键表（status=PENDING / flag_enable=1 / priority=10，usc 留空）。
// 键表已有该 ftmm_symbol 时不入队，改为提示去「USC 映射规则」补 USC —— 补好后回到本页用「手动补录USC」同步。
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import { apiGet, apiPost } from '../../../../../api'
import { showToast } from '../../../../../ui/toast'

interface PendingSecuRow {
  stock_id: number
  name: string
  stock_code: string
  market_type: string
  market_label: string
  instrument_type: string
}

const props = defineProps<{
  modelValue: boolean
  row: PendingSecuRow | null
}>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  done: []
}>()

const router = useRouter()

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

// ---- 预检结果（与后端 PendingSecuEnqueuePreview 一致）----

interface EnqueuePreview {
  stock_id: number
  name: string
  ftmm_symbol: string
  market: string
  code: string
  instrument_type: string
  market_type: string
  instrument: string
  existed: boolean
  status: string
  usc: string
  priority: number
  writable: boolean
  hint: string
}

interface EnqueueResult {
  ftmm_symbol: string
  instrument: string
  enqueued: boolean
  existed: boolean
  status: string
  usc: string
  hint: string
}

/** 键表 instrument 词表（与后端 KeyInstrumentOptions 一致；下拉用「中文[值]」，DictFilterSpec） */
const INSTRUMENT_OPTIONS = [
  { title: '股票[stock]', value: 'stock' },
  { title: 'ETF[etf]', value: 'etf' },
  { title: '指数[index]', value: 'index' },
  { title: '板块[sector]', value: 'sector' },
  { title: '期货[futures]', value: 'futures' },
  { title: '加密货币[crypto]', value: 'crypto' },
  { title: '外汇[forex]', value: 'forex' },
  { title: '大盘[market]', value: 'market' },
]

const loading = ref(false)
const submitting = ref(false)
const error = ref('')
const preview = ref<EnqueuePreview | null>(null)
const instrument = ref('')

async function loadPreview() {
  preview.value = null
  instrument.value = ''
  error.value = ''
  const row = props.row
  if (!row?.stock_id) return
  loading.value = true
  try {
    const data = await apiGet<EnqueuePreview>(
      `/Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Enqueue?stockId=${row.stock_id}`,
    )
    preview.value = data
    instrument.value = data.instrument ?? ''
  } catch (err) {
    error.value = `预检失败：${(err as Error).message}`
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.modelValue, props.row?.stock_id] as const,
  ([open]) => {
    if (open) void loadPreview()
  },
  { immediate: true },
)

const canSubmit = computed(
  () => !!preview.value?.writable && !!instrument.value && !loading.value && !submitting.value,
)

/** 已入队（键表已有该键）时不再提供提交，改为引导去「USC 映射规则」 */
const alreadyQueued = computed(() => preview.value !== null && !preview.value.writable)

async function submit() {
  if (!canSubmit.value || !props.row) return
  submitting.value = true
  error.value = ''
  try {
    const result = await apiPost<EnqueueResult>(
      '/Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Enqueue',
      { stock_id: props.row.stock_id, instrument: instrument.value },
      60000,
    )
    if (result.existed) {
      showToast(`键表已存在 ${result.ftmm_symbol}，未重复入队：${result.hint}`, 'warning')
      preview.value = { ...preview.value!, existed: true, writable: false, status: result.status, usc: result.usc, hint: result.hint }
      return
    }
    showToast(`已入队 ${result.ftmm_symbol}（instrument=${result.instrument}）`, 'success')
    emit('done')
    close()
  } catch (err) {
    error.value = `入队失败：${(err as Error).message}`
    showToast(error.value, 'error')
  } finally {
    submitting.value = false
  }
}

/** 去「USC 映射规则」菜单补 USC（只在用户点按钮时跳转，不自动跳） */
function goUscMapping() {
  close()
  void router.push('/Meta/Finv/Quote/Realtime/UscMapping')
}

function close() {
  visible.value = false
  error.value = ''
}
</script>

<template>
  <v-dialog v-model="visible" max-width="880" scrollable>
    <v-card>
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-tray-arrow-up" class="mr-2" />
        入队确认
        <v-chip v-if="row" size="small" variant="tonal" color="primary" class="ml-3">
          {{ row.stock_code }} {{ row.name }}
        </v-chip>
      </v-card-title>
      <v-card-subtitle class="pb-2">
        入队 = 把该证券的键登记进 <code>futu.finv_futu_secu_key</code>（
        <code>status=PENDING</code>、<code>flag_enable=1</code>、<code>priority=10</code>，<code>usc</code> 留空等规则解析）。
        <strong>本页不会自动提交</strong>：核对下面将要写入的字段，确认无误再点「确认入队」。
      </v-card-subtitle>

      <v-divider />

      <v-card-text>
        <v-progress-linear v-if="loading" indeterminate color="primary" class="mb-3" />

        <v-alert v-if="error" type="error" density="compact" variant="tonal" class="mb-3">
          {{ error }}
        </v-alert>

        <!-- 快照摘要 -->
        <v-sheet v-if="row" class="pa-3 rounded mb-3" color="surface-variant">
          <div class="d-flex flex-wrap ga-4 text-caption">
            <span>证券编号 <strong>{{ row.stock_id }}</strong></span>
            <span>代码 <strong>{{ row.stock_code || '—' }}</strong></span>
            <span>名称 <strong>{{ row.name || '—' }}</strong></span>
            <span>市场 <strong>{{ row.market_label || '—' }}</strong></span>
            <span>instrument_type <strong>{{ row.instrument_type || '—' }}</strong></span>
            <span>market_type <strong>{{ row.market_type || '—' }}</strong></span>
          </div>
        </v-sheet>

        <!-- 将要写入键表的字段（服务端派生，只读）：一个字段一行，不合并展示 -->
        <div class="text-subtitle-2 mb-1">将要写入键表的字段</div>
        <v-table v-if="preview" density="compact" class="mb-3 finv-enqueue-fields">
          <thead>
            <tr>
              <th class="text-no-wrap">字段</th>
              <th class="text-no-wrap">取值</th>
              <th class="text-no-wrap">来源 / 说明</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td class="text-no-wrap"><code>ftmm_symbol</code></td>
              <td class="text-no-wrap"><code>{{ preview.ftmm_symbol }}</code></td>
              <td>主键：<code>upper(市场标签.代码)</code>，派生后只读</td>
            </tr>
            <tr>
              <td class="text-no-wrap"><code>market</code></td>
              <td class="text-no-wrap">{{ preview.market }}</td>
              <td>取自快照 <code>market_label</code></td>
            </tr>
            <tr>
              <td class="text-no-wrap"><code>code</code></td>
              <td class="text-no-wrap">{{ preview.code }}</td>
              <td>取自快照 <code>stock_code</code></td>
            </tr>
            <tr>
              <td class="text-no-wrap"><code>stock_id</code></td>
              <td class="text-no-wrap">{{ preview.stock_id }}</td>
              <td>取自快照证券编号</td>
            </tr>
            <tr>
              <td class="text-no-wrap"><code>instrument</code></td>
              <td class="text-no-wrap">{{ instrument || '—' }}</td>
              <td>见下方下拉，可人工改正</td>
            </tr>
            <tr>
              <td class="text-no-wrap"><code>status</code></td>
              <td class="text-no-wrap">PENDING</td>
              <td>键表固定取值（入队即待处理）</td>
            </tr>
            <tr>
              <td class="text-no-wrap"><code>flag_enable</code></td>
              <td class="text-no-wrap">1</td>
              <td>键表固定取值（启用）</td>
            </tr>
            <tr>
              <td class="text-no-wrap"><code>priority</code></td>
              <td class="text-no-wrap">10</td>
              <td>键表固定取值（最低优先级）</td>
            </tr>
            <tr>
              <td class="text-no-wrap"><code>usc</code></td>
              <td class="text-no-wrap text-medium-emphasis">留空</td>
              <td>入队后由「USC 映射规则」解析</td>
            </tr>
          </tbody>
        </v-table>

        <!-- instrument：唯一可人工改正的字段 -->
        <div class="d-flex flex-wrap align-center ga-2 mb-3">
          <v-select
            v-model="instrument"
            :items="INSTRUMENT_OPTIONS"
            item-title="title"
            item-value="value"
            :return-object="false"
            label="instrument（按品种码推导，可改正）"
            :hint="`由 instrument_type=${preview?.instrument_type || '—'} / market_type=${preview?.market_type || '—'} 推导；词表与键表一致（板块是 sector，不是采集表的 sectors）`"
            persistent-hint
            density="compact"
            :disabled="alreadyQueued"
            style="min-width: 320px; max-width: 420px"
          />
        </div>

        <!-- 键表现状 -->
        <v-alert v-if="alreadyQueued" type="warning" density="compact" variant="tonal" class="mb-2">
          <div class="mb-1"><strong>键表已存在该记录</strong>（status={{ preview?.status || '—' }}、usc={{ preview?.usc || '空' }}、priority={{ preview?.priority }}），本次不能重复入队。</div>
          <div>{{ preview?.hint }}</div>
        </v-alert>
        <v-alert v-else-if="preview" type="info" density="compact" variant="tonal" class="mb-2">
          {{ preview.hint }}
        </v-alert>
      </v-card-text>

      <v-divider />

      <v-card-actions>
        <span class="text-caption text-medium-emphasis ml-2">
          {{ alreadyQueued ? '已有该键，需先补 USC 后才能同步本页记录' : '核对无误后才会写库' }}
        </span>
        <v-spacer />
        <v-btn variant="text" :disabled="submitting" @click="close">取消</v-btn>
        <v-btn
          v-if="alreadyQueued"
          color="primary"
          variant="tonal"
          prepend-icon="mdi-transit-connection-variant"
          @click="goUscMapping"
        >
          去「USC 映射规则」
        </v-btn>
        <v-btn
          v-else
          color="primary"
          prepend-icon="mdi-tray-arrow-up"
          :disabled="!canSubmit"
          :loading="submitting"
          @click="submit"
        >
          确认入队
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
/* 「将要写入键表的字段」表：字段名与取值等宽，说明列允许折行 */
.finv-enqueue-fields :deep(td),
.finv-enqueue-fields :deep(th) {
  padding-inline: 12px;
}

.finv-enqueue-fields :deep(td:nth-child(1)),
.finv-enqueue-fields :deep(td:nth-child(2)) {
  white-space: nowrap;
}
</style>
