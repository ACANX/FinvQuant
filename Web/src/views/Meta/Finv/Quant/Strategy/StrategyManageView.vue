<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { apiGet, apiPost } from '../../../../../api'

interface StrategyRow {
  strategy_id: string
  strategy_code: string
  strategy_name: string
  strategy_type: string
  description: string
  definition: Record<string, unknown>
  definition_version: number
  data_period: string
  secu_code: string
  template_id?: string | null
  allow_backtest: string
  status: string
  created_by: string
}

const rows = ref<StrategyRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const loading = ref(false)
const error = ref('')
const message = ref('')

const dialog = ref(false)
const editing = ref(false)

// 策略表单（与 StrategyRow 同构，避免 Record<string, unknown> 的 v-model 类型问题）
interface StrategyForm {
  strategy_id: string
  strategy_code: string
  strategy_name: string
  strategy_type: string
  description: string
  definition_version: number
  data_period: string
  secu_code: string
  template_id: string | null
  allow_backtest: string
  status: string
  created_by: string
}

const form = ref<StrategyForm>({
  strategy_id: '',
  strategy_code: '',
  strategy_name: '',
  strategy_type: 'RULE_BASED',
  description: '',
  definition_version: 1,
  data_period: 'Min',
  secu_code: '',
  template_id: null,
  allow_backtest: '1',
  status: 'ENABLED',
  created_by: '',
})
const defText = ref('')
const defError = ref('')
const templateKey = ref('')
const apiTemplates = ref<{ template_id: string; template_code: string; template_name: string; content: Record<string, unknown> }[]>([])

// 内置策略模板（本地兜底，与种子模板 TPL-STRAT-* 内容一致；
// 评审：原实现 4 个模板全部前端硬编码且从不调用 Template/List，现改为优先从 API 加载）
const localTemplates: Record<string, { name: string; json: string }> = {
  'dual-ma': {
    name: '双均线交叉（GCMain 示例）',
    json: JSON.stringify({
      version: '1',
      strategy_type: 'RULE_BASED',
      description: '双均线交叉策略：MA5 上穿 MA20 买入，下穿卖出，3% 止损；分批建仓 + 分批减仓',
      universe: { securities: ['GCMain'] },
      data: { period: 'Min', price_field: 'close', warmup_bars: 30, fill_mode: 'NEXT_BAR_OPEN' },
      indicators: [
        { id: 'ma_fast', type: 'MA', params: { window: 5, field: 'close' } },
        { id: 'ma_slow', type: 'MA', params: { window: 20, field: 'close' } },
      ],
      signals: { buy: 'cross_up(ma_fast, ma_slow)', sell: 'cross_down(ma_fast, ma_slow)' },
      rules: {
        buy: { action: 'BUY', quantity_type: 'ALL_IN', quantity: 0, max_per_day: 0, max_per_run: 0, allowed_times: [], allow: true },
        sell: { action: 'SELL', quantity_type: 'ALL', quantity: 0, max_per_day: 0, max_per_run: 0, allowed_times: [], allow: true },
      },
      risk: {
        stop_loss_pct: 3, take_profit_pct: 0, max_position_pct: 100, max_positions: 1,
        max_trades_per_day: 0, min_interval_bars: 0,
        builder: { enabled: true, target_position_pct: 80, tranches: 4, tranche_interval_bars: 240 },
        reduce_tranches: 3,
        max_trades_per_week: 0, max_trades_per_month: 0, max_fee_per_window: 0, fee_window_days: 0,
      },
      cost: { commission_rate: 0.0003, slippage_pct: 0.0001 },
    }, null, 2),
  },
  rsi: {
    name: 'RSI 超买超卖',
    json: JSON.stringify({
      version: '1',
      strategy_type: 'RULE_BASED',
      description: 'RSI(14) 低于 30 买入，高于 70 卖出',
      universe: { securities: ['GCMain'] },
      data: { period: 'Min', price_field: 'close', warmup_bars: 30, fill_mode: 'NEXT_BAR_OPEN' },
      indicators: [{ id: 'rsi14', type: 'RSI', params: { window: 14, field: 'close' } }],
      signals: { buy: 'rsi14 < 30', sell: 'rsi14 > 70' },
      rules: {
        buy: { action: 'BUY', quantity_type: 'ALL_IN', quantity: 0, max_per_day: 0, max_per_run: 0, allowed_times: [], allow: true },
        sell: { action: 'SELL', quantity_type: 'ALL', quantity: 0, max_per_day: 0, max_per_run: 0, allowed_times: [], allow: true },
      },
      risk: { stop_loss_pct: 0, take_profit_pct: 0, max_position_pct: 100, max_positions: 1, max_trades_per_day: 0, min_interval_bars: 0 },
      cost: { commission_rate: 0.0003, slippage_pct: 0.0001 },
    }, null, 2),
  },
  boll: {
    name: '布林带突破',
    json: JSON.stringify({
      version: '1',
      strategy_type: 'RULE_BASED',
      description: '收盘价突破布林上轨买入，跌破中轨卖出',
      universe: { securities: ['GCMain'] },
      data: { period: 'Min', price_field: 'close', warmup_bars: 30, fill_mode: 'NEXT_BAR_OPEN' },
      indicators: [
        { id: 'boll_up', type: 'BOLL', params: { window: 20, k: 2, field: 'close', output: 'upper' } },
        { id: 'boll_mid', type: 'BOLL', params: { window: 20, k: 2, field: 'close', output: 'mid' } },
      ],
      signals: { buy: 'cross_up(close, boll_up)', sell: 'cross_down(close, boll_mid)' },
      rules: {
        buy: { action: 'BUY', quantity_type: 'ALL_IN', quantity: 0, max_per_day: 0, max_per_run: 0, allowed_times: [], allow: true },
        sell: { action: 'SELL', quantity_type: 'ALL', quantity: 0, max_per_day: 0, max_per_run: 0, allowed_times: [], allow: true },
      },
      risk: { stop_loss_pct: 0, take_profit_pct: 0, max_position_pct: 100, max_positions: 1, max_trades_per_day: 0, min_interval_bars: 0 },
      cost: { commission_rate: 0.0003, slippage_pct: 0.0001 },
    }, null, 2),
  },
  macd: {
    name: 'MACD 金叉死叉',
    json: JSON.stringify({
      version: '1',
      strategy_type: 'RULE_BASED',
      description: 'MACD DIF 上穿 DEA（金叉）买入，下穿（死叉）卖出',
      universe: { securities: ['GCMain'] },
      data: { period: 'Min', price_field: 'close', warmup_bars: 60, fill_mode: 'NEXT_BAR_OPEN' },
      indicators: [
        { id: 'dif', type: 'MACD', params: { fast: 12, slow: 26, signal: 9, field: 'close', output: 'dif' } },
        { id: 'dea', type: 'MACD', params: { fast: 12, slow: 26, signal: 9, field: 'close', output: 'dea' } },
      ],
      signals: { buy: 'cross_up(dif, dea)', sell: 'cross_down(dif, dea)' },
      rules: {
        buy: { action: 'BUY', quantity_type: 'ALL_IN', quantity: 0, max_per_day: 0, max_per_run: 0, allowed_times: [], allow: true },
        sell: { action: 'SELL', quantity_type: 'ALL', quantity: 0, max_per_day: 0, max_per_run: 0, allowed_times: [], allow: true },
      },
      risk: { stop_loss_pct: 0, take_profit_pct: 0, max_position_pct: 100, max_positions: 1, max_trades_per_day: 0, min_interval_bars: 0 },
      cost: { commission_rate: 0.0003, slippage_pct: 0.0001 },
    }, null, 2),
  },
}

const templateOptions = computed(() => {
  const apiOpts = apiTemplates.value.map((t) => ({ title: `[模板] ${t.template_name}`, value: t.template_id }))
  const localOpts = Object.entries(localTemplates).map(([k, v]) => ({ title: `[内置] ${v.name}`, value: `local:${k}` }))
  return [...apiOpts, ...localOpts]
})

async function loadTemplates() {
  try {
    const data = await apiGet<{ list: { template_id: string; template_code: string; template_name: string; content: Record<string, unknown> }[] }>(
      '/Meta/Finv/Quant/Backtest/Template/List?page=1&pageSize=100&templateType=STRATEGY',
    )
    apiTemplates.value = data.list ?? []
  } catch {
    // 模板接口不可用时回退本地内置模板
    apiTemplates.value = []
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet<{ total: number; list: StrategyRow[] }>(
      `/Meta/Finv/Quant/Backtest/Strategy/List?page=${page.value}&pageSize=${pageSize.value}&keyword=${encodeURIComponent(keyword.value)}`,
    )
    rows.value = data.list ?? []
    total.value = data.total ?? 0
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

function applyTemplate() {
  defError.value = ''
  const key = templateKey.value
  if (!key) return
  if (key.startsWith('local:')) {
    defText.value = localTemplates[key.slice(6)]?.json ?? ''
    form.value.template_id = null
    return
  }
  const t = apiTemplates.value.find((x) => x.template_id === key)
  if (t) {
    defText.value = JSON.stringify(t.content ?? {}, null, 2)
    form.value.template_id = t.template_id
  }
}

function openCreate() {
  editing.value = false
  form.value = {
    strategy_id: '', strategy_code: '', strategy_name: '', strategy_type: 'RULE_BASED',
    description: '', definition_version: 1, data_period: 'Min', secu_code: '',
    template_id: null, allow_backtest: '1', status: 'ENABLED', created_by: '',
  }
  templateKey.value = ''
  applyTemplate()
  dialog.value = true
}

function openEdit(row: StrategyRow) {
  editing.value = true
  form.value = {
    ...row,
    template_id: (row as unknown as { template_id?: string | null }).template_id ?? null,
  }
  defText.value = JSON.stringify(row.definition ?? {}, null, 2)
  defError.value = ''
  templateKey.value = ''
  dialog.value = true
}

async function save() {
  error.value = ''
  // 校验 JSON 定义
  let definition: Record<string, unknown>
  try {
    definition = JSON.parse(defText.value)
  } catch (e) {
    defError.value = '策略定义 JSON 格式错误：' + (e as Error).message
    return
  }
  defError.value = ''
  try {
    await apiPost('/Meta/Finv/Quant/Backtest/Strategy/Save', { ...form.value, definition })
    message.value = '保存成功'
    dialog.value = false
    await load()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function toggle(row: StrategyRow) {
  try {
    await apiPost('/Meta/Finv/Quant/Backtest/Strategy/Toggle', {
      strategy_id: row.strategy_id,
      allow_backtest: row.allow_backtest === '1' ? '0' : '1',
    })
    await load()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function remove(row: StrategyRow) {
  if (!confirm(`确认删除策略「${row.strategy_name}」？`)) return
  try {
    await apiPost('/Meta/Finv/Quant/Backtest/Strategy/Delete', { strategy_id: row.strategy_id })
    message.value = '删除成功'
    await load()
  } catch (e) {
    error.value = (e as Error).message
  }
}

onMounted(async () => {
  await Promise.all([load(), loadTemplates()])
})
</script>

<template>
  <v-container fluid>
    <v-card>
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-sitemap-outline" class="mr-2" color="primary" />
        策略管理（结构化策略定义）
        <v-spacer />
        <v-text-field v-model="keyword" density="compact" label="搜索编码/名称/标的" hide-details
          class="mx-4" style="max-width: 260px" @keyup.enter="load" />
        <v-btn color="primary" variant="tonal" class="mr-2" @click="load">查询</v-btn>
        <v-btn color="primary" @click="openCreate">新建策略</v-btn>
      </v-card-title>

      <v-alert v-if="error" type="error" dismissible class="mx-4 mb-2">{{ error }}</v-alert>
      <v-alert v-if="message" type="success" dismissible class="mx-4 mb-2">{{ message }}</v-alert>

      <v-data-table-server v-model:page="page" v-model:items-per-page="pageSize" :headers="[
        { title: '编码', key: 'strategy_code', width: 150 },
        { title: '名称', key: 'strategy_name' },
        { title: '类型', key: 'strategy_type', width: 110 },
        { title: '标的', key: 'secu_code', width: 110 },
        { title: '周期', key: 'data_period', width: 80 },
        { title: '版本', key: 'definition_version', width: 70 },
        { title: '回测开关', key: 'allow_backtest', width: 100 },
        { title: '操作', key: 'actions', width: 180, sortable: false },
      ]" :items="rows" :loading="loading" :items-length="total" item-value="strategy_id"
        @update:options="load">
        <template #item.strategy_type="{ item }">
          <v-chip size="small" color="primary" variant="tonal">{{ item.strategy_type }}</v-chip>
        </template>
        <template #item.data_period="{ item }">
          {{ item.data_period }}
        </template>
        <template #item.allow_backtest="{ item }">
          <v-switch :model-value="item.allow_backtest === '1'" density="compact" hide-details
            :color="item.allow_backtest === '1' ? 'green' : 'grey'" @update:model-value="toggle(item)" />
        </template>
        <template #item.actions="{ item }">
          <v-btn size="small" variant="text" @click="openEdit(item)">编辑</v-btn>
          <v-btn size="small" variant="text" color="error" @click="remove(item)">删除</v-btn>
        </template>
      </v-data-table-server>
    </v-card>

    <!-- 新增/编辑对话框 -->
    <v-dialog v-model="dialog" max-width="880">
      <v-card>
        <v-card-title>{{ editing ? '编辑策略' : '新建策略' }}</v-card-title>
        <v-card-text>
          <v-row>
            <v-col cols="6">
              <v-text-field v-model="form.strategy_code" label="策略编码 *" hint="全局唯一，如 STRAT-DUALMA-GC" />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model="form.strategy_name" label="策略名称 *" />
            </v-col>
          </v-row>
          <v-row>
            <v-col cols="4">
              <v-select v-model="form.data_period" :items="['Min', 'Hour', 'Day']" label="默认数据周期" />
            </v-col>
            <v-col cols="4">
              <v-text-field v-model="form.secu_code" label="默认标的证券代码" hint="如 GCMain / 518880 / NVDA" />
            </v-col>
            <v-col cols="4">
              <v-switch v-model="form.allow_backtest" true-value="1" false-value="0" label="回测开关"
                color="primary" hide-details class="mt-3" />
            </v-col>
          </v-row>
          <v-text-field v-model="form.description" label="策略说明" />

          <v-divider class="my-3" />
          <v-row align="center" class="mb-2">
            <v-col cols="auto">
              <v-icon icon="mdi-code-json" class="mr-1" color="primary" />
              <strong>结构化策略定义（JSON）</strong>
            </v-col>
            <v-spacer />
            <v-select v-model="templateKey" :items="templateOptions" label="策略模板（内置/自定义）"
              density="compact" hide-details style="max-width: 320px" class="mr-2"
              clearable @update:model-value="applyTemplate" />
            <v-btn size="small" variant="tonal" @click="applyTemplate">载入模板</v-btn>
          </v-row>
          <v-textarea v-model="defText" :error="!!defError" :error-messages="defError"
            rows="16" class="font-mono" style="font-family: monospace" spellcheck="false" />
          <v-alert type="info" variant="tonal" density="compact" class="mt-1">
            定义模型：universe（标的池）/ data（周期·撮合模式）/ indicators（指标）/ signals（信号表达式）/
            rules（规则·数量·限制）/ risk（风控）/ cost（成本）。表达式支持
            cross_up / cross_down / ref / highest / lowest / abs 与比较、AND/OR/NOT。
          </v-alert>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="dialog = false">取消</v-btn>
          <v-btn color="primary" @click="save">保存</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </v-container>
</template>
