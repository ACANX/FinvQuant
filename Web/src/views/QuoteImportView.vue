<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { apiGet } from '../api'

// ---------------------------------------------------------------------------
// 历史行情数据导入：证券选择双策略
//   策略 1「先选证券再选文件」：下拉选证券代码 → 自动带出市场代码/证券信息核对
//     → 选择文件后前端解析文件头，与所选证券双向核对，确认同一证券才允许上传
//   策略 2「先选文件自动匹配」：先选 MVSV 文件 → 前端解析文件头代码 →
//     自动匹配证券字典并补全证券代码/市场代码 → 补全成功才允许上传
// 上传时后端仍做文件头一致性校验兜底（见 internal/api/handler/import.go）。
// ---------------------------------------------------------------------------

interface SecurityOption {
  usc: string
  security_name_cn: string
}

// finv_exchange 交易所字典（用于数字后展示中文名称，如 31 [纳斯达克]）
interface ExchangeDict {
  exchange_code: number
  exchange_name: string
  exchange_abbr_cn: string
}

// finv_market 市场字典（用于数字后展示中文名称，如 1315 [债券市场]）
interface MarketDict {
  market_code: number
  market_name: string
}

interface SecurityDetail {
  usc: string
  exchange_code: number
  market_code: number
  security_type: string
  security_code: string
  security_name: string
  security_name_cn: string
  security_name_full: string | null
  currency_type: string
  init_date: number
  timezone: string | null
  tz: string | null
  flag_enable: string
}

/** MVSV 文件头解析结果 */
interface FileHeader {
  code: string // # Code : xxx
  marketCode: string // # MarketCode : xxx
}

// 导入参数（上传时提交）
const marketCode = ref('')
const secuCode = ref('')
const source = ref('')
const upsertMode = ref('FIELD')
const remark = ref('')
const confirmChecked = ref(false)
const importing = ref(false)
const result = ref('')
const error = ref('')

// 策略切换：'first' = 先选证券；'file' = 先选文件
const strategy = ref<'first' | 'file'>('first')

// 证券下拉字典
const securityOptions = ref<SecurityOption[]>([])
// 交易所/市场字典映射（code → 中文名称，加载自 List 接口）
const exchangeDict = ref<Map<number, ExchangeDict>>(new Map())
const marketDict = ref<Map<number, MarketDict>>(new Map())

/** 交易所数字 → “31 [纳斯达克]”；未命中时仅返回数字 */
function fmtExchange(code: number): string {
  const hit = exchangeDict.value.get(code)
  if (!hit) return String(code)
  const name = hit.exchange_abbr_cn || hit.exchange_name
  return name ? `${code} [${name}]` : String(code)
}

/** 市场数字 → “1315 [债券市场]”；未命中时仅返回数字 */
function fmtMarket(code: number): string {
  const hit = marketDict.value.get(code)
  if (!hit) return String(code)
  return hit.market_name ? `${code} [${hit.market_name}]` : String(code)
}
const secuItems = computed(() =>
  securityOptions.value.map((o) => ({
    title: `${o.usc}:${o.security_name_cn}`,
    value: o.usc,
  })),
)

// 选中证券后的详情（策略 1）
const pickedSec = ref<SecurityDetail | null>(null)
const picking = ref(false)
const pickError = ref('')

// 文件头解析（两种策略共用）
const file = ref<File | null>(null)
const fileHeader = ref<FileHeader | null>(null)
const parsing = ref(false)

// 核对状态（策略 1 的双向核对结果）
const checkStatus = ref<'idle' | 'ok' | 'mismatch'>('idle')
const checkDetail = ref('')

// 策略 2：文件匹配到的证券
const matchedSec = ref<SecurityDetail | null>(null)
const matching = ref(false)
const matchError = ref('')

onMounted(async () => {
  try {
    const data = await apiGet<{ list: SecurityOption[] }>('/Meta/FinvQuant/Metadata/Security/Options')
    securityOptions.value = data.list ?? []
  } catch {
    securityOptions.value = []
  }
  // 加载交易所/市场字典（含禁用，供数字后中文名称展示）
  try {
    const [ex, mk] = await Promise.all([
      apiGet<{ list: ExchangeDict[] }>('/Meta/FinvQuant/Metadata/Exchange/List?page=1&page_size=500'),
      apiGet<{ list: MarketDict[] }>('/Meta/FinvQuant/Metadata/Market/List?page=1&page_size=500'),
    ])
    exchangeDict.value = new Map((ex.list ?? []).map((e) => [e.exchange_code, e]))
    marketDict.value = new Map((mk.list ?? []).map((m) => [m.market_code, m]))
  } catch {
    // 字典加载失败不影响主流程，名称展示降级为仅数字
  }
})

// ---------------------------------------------------------------------------
// 策略 1：选中证券代码 → 自动带出证券信息（含市场代码/交易所），供核对
// ---------------------------------------------------------------------------
async function onPickSecu(val: string | null) {
  pickError.value = ''
  pickedSec.value = null
  checkStatus.value = 'idle'
  checkDetail.value = ''
  const code = (val ?? '').trim()
  if (!code) return
  picking.value = true
  try {
    const data = await apiGet<{ found: boolean; security?: SecurityDetail }>(
      `/Meta/FinvQuant/Metadata/Security/Lookup?code=${encodeURIComponent(code)}`,
    )
    if (!data.found || !data.security) {
      pickError.value = `证券 ${code} 未在字典中登记，请先在「规范证券信息维护」中维护`
      return
    }
    pickedSec.value = data.security
    secuCode.value = data.security.usc
    // 自动带出市场代码：优先取字典 market_code（有值则用之），
    // 未维护（0）时回退到交易所代码 exchange_code；选文件后以文件头 MarketCode 二次核对
    marketCode.value = data.security.market_code > 0
      ? String(data.security.market_code)
      : String(data.security.exchange_code)
  } catch (e) {
    pickError.value = (e as Error).message
  } finally {
    picking.value = false
  }
}

// ---------------------------------------------------------------------------
// 文件头解析（MVSV 文本格式：# Key : Value，空行分隔头部与数据区）
// ---------------------------------------------------------------------------
async function parseFileHeader(f: File): Promise<FileHeader> {
  const text = await f.slice(0, 4096).text()
  const head = text.split(/\r?\n/).slice(0, 40)
  const get = (key: string) => {
    const hit = head.find((l) => l.trim().toLowerCase().startsWith(`# ${key.toLowerCase()}`))
    if (!hit) return ''
    // 与后端 internal/mvsv/parser.go buildHeader 保持一致：去掉值首尾的双引号
    // （MVSV 头部值为带引号字符串，如 # Code : "NVDA"）
    const val = hit.split(':').slice(1).join(':').trim()
    return val.replace(/^"+|"+$/g, '')
  }
  const code = get('Code')
  const marketCodeVal = get('MarketCode')
  if (!code || !marketCodeVal) {
    throw new Error('文件头缺少 Code 或 MarketCode 字段，无法解析证券信息')
  }
  return { code, marketCode: marketCodeVal }
}

async function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const f = input.files?.[0] ?? null
  file.value = f
  fileHeader.value = null
  matchedSec.value = null
  matchError.value = ''
  checkStatus.value = 'idle'
  checkDetail.value = ''
  if (!f) return

  parsing.value = true
  try {
    const h = await parseFileHeader(f)
    fileHeader.value = h
    // 策略 1：选文件后做双向核对（文件头 vs 已选证券）
    if (strategy.value === 'first') {
      verifyStrategy1(h)
    } else {
      // 策略 2：先选文件 → 自动匹配证券并补全
      await autoMatch(h)
    }
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    parsing.value = false
  }
}

// ---------------------------------------------------------------------------
// 策略 1 双向核对：文件头 Code/MarketCode 与所选证券（字典）逐项比对
// ---------------------------------------------------------------------------
function verifyStrategy1(h: FileHeader) {
  const sec = pickedSec.value
  if (!sec) {
    checkStatus.value = 'mismatch'
    checkDetail.value = '请先在「证券代码」中选择证券，再选择文件进行核对'
    return
  }
  const codeOk = h.code === sec.usc || h.code === sec.security_code
  // 市场代码核对：字典 market_code 已维护（>0）时与文件头比对；未维护时仅展示提示，不强校验
  const marketRef = sec.market_code > 0 ? String(sec.market_code) : ''
  const marketOk = marketRef ? h.marketCode === marketRef : true
  const parts: string[] = []
  parts.push(`文件证券代码 ${h.code} ${codeOk ? '✓ 与' : '✗ 不匹配'} 字典 ${sec.usc}(${sec.security_code})`)
  parts.push(
    marketRef
      ? `文件市场代码 ${h.marketCode} ${marketOk ? '✓ 与' : '✗ 不匹配'} 字典 ${marketRef}`
      : `文件市场代码 ${h.marketCode}（字典未维护 market_code，跳过市场代码强校验）`,
  )
  if (codeOk && marketOk) {
    checkStatus.value = 'ok'
    checkDetail.value = `核对通过：${parts.join('；')}`
    // 文件头为准，回填导入参数
    secuCode.value = sec.usc
    marketCode.value = h.marketCode
  } else {
    checkStatus.value = 'mismatch'
    checkDetail.value = `核对未通过：${parts.join('；')}。请确认文件与所选证券属于同一证券。`
  }
}

// ---------------------------------------------------------------------------
// 策略 2：按文件头代码自动匹配证券并补全
// ---------------------------------------------------------------------------
async function autoMatch(h: FileHeader) {
  matching.value = true
  matchError.value = ''
  try {
    const data = await apiGet<{ found: boolean; security?: SecurityDetail }>(
      `/Meta/FinvQuant/Metadata/Security/Lookup?code=${encodeURIComponent(h.code)}`,
    )
    if (!data.found || !data.security) {
      matchError.value = `文件证券代码 ${h.code} 未匹配到字典证券，请先在「规范证券信息维护」中登记后再导入`
      return
    }
    matchedSec.value = data.security
    // 补全证券代码 + 市场代码（文件头 MarketCode 为准）
    secuCode.value = data.security.usc
    marketCode.value = h.marketCode
  } catch (e) {
    matchError.value = (e as Error).message
  } finally {
    matching.value = false
  }
}

// ---------------------------------------------------------------------------
// 上传
// ---------------------------------------------------------------------------
const canUpload = computed(() => {
  if (!file.value) return false
  if (strategy.value === 'first') {
    // 策略 1：必须选择证券 + 文件头核对通过
    return !!pickedSec.value && checkStatus.value === 'ok' && !!confirmChecked.value
  }
  // 策略 2：必须自动匹配成功补全
  return !!matchedSec.value && !!fileHeader.value && !!confirmChecked.value
})

function switchStrategy(s: 'first' | 'file') {
  strategy.value = s
  error.value = ''
  result.value = ''
  checkStatus.value = 'idle'
  checkDetail.value = ''
  matchedSec.value = null
  matchError.value = ''
  // 保留已解析的文件头：切回策略 1 时若已选证券则重新核对
  if (fileHeader.value && s === 'first' && pickedSec.value) {
    verifyStrategy1(fileHeader.value)
  }
}

async function submitImport() {
  error.value = ''
  result.value = ''
  if (!file.value || !fileHeader.value) {
    error.value = '请选择要上传的 MVSV 文件'
    return
  }
  if (strategy.value === 'first' && checkStatus.value !== 'ok') {
    error.value = '文件与证券核对未通过，不允许上传'
    return
  }
  if (strategy.value === 'file' && !matchedSec.value) {
    error.value = '证券匹配未成功，不允许上传'
    return
  }
  if (!source.value.trim()) {
    error.value = '数据源不能为空'
    return
  }
  if (!confirmChecked.value) {
    error.value = '请确认导入将覆盖同时刻同证券的对应字段值'
    return
  }

  const form = new FormData()
  form.append('file', file.value)
  if (marketCode.value.trim()) form.append('market_code', marketCode.value.trim())
  if (secuCode.value.trim()) form.append('secu_code', secuCode.value.trim())
  form.append('source', source.value.trim())
  form.append('upsert_mode', upsertMode.value)
  form.append('imported_by', 'gui')
  if (remark.value.trim()) form.append('remark', remark.value.trim())

  importing.value = true
  try {
    const response = await fetch('/API/V1/Quote/Import/Upload', { method: 'POST', body: form })
    const body = await response.json()
    if (body.code !== 0) {
      error.value = body.message || '导入失败'
      return
    }
    const data = body.data ?? {}
    result.value = `导入完成：${data.secu_code ?? '?'}（market=${data.market_code ?? '?'}）` +
      ` ${data.record_count ?? 0} 条` +
      `（新增 ${data.inserted ?? 0} / 覆盖 ${data.updated ?? 0}）` +
      `｜批次 ${data.batch_id ?? '-'}｜模式 ${data.mode ?? '-'}`
  } catch {
    error.value = '网络错误：无法连接服务端'
  } finally {
    importing.value = false
  }
}
</script>

<template>
  <v-card max-width="860" class="mx-auto">
    <v-card-title>
      <v-icon icon="mdi-database-import" class="mr-2" />
      历史行情数据导入
    </v-card-title>
    <v-card-subtitle>
      上传 MVSV-1 分钟级历史行情文件，服务端解析后字段级覆盖导入 PostgreSQL（finv_quote_secu_kline_min）
    </v-card-subtitle>

    <v-card-text>
      <!-- 双策略切换 -->
      <v-tabs v-model="strategy" color="primary" @update:model-value="switchStrategy">
        <v-tab value="first">
          <v-icon icon="mdi-form-select" class="mr-1" />先选证券，再选文件核对
        </v-tab>
        <v-tab value="file">
          <v-icon icon="mdi-file-search" class="mr-1" />先选文件，自动匹配证券
        </v-tab>
      </v-tabs>

      <v-divider class="my-4" />

      <v-form @submit.prevent="submitImport">
        <!-- 策略 1：先选证券 -->
        <template v-if="strategy === 'first'">
          <v-row>
            <v-col cols="12" md="6">
              <v-combobox
                v-model="secuCode"
                label="证券代码（可搜索/手动输入）"
                :items="secuItems"
                clearable
                hint="选择后自动带出证券信息与市场代码，供核对"
                @update:model-value="onPickSecu"
              />
            </v-col>
            <v-col cols="12" md="6">
              <v-text-field
                v-model="marketCode"
                label="市场代码（自动带出）"
                readonly
                hint="优先取字典 market_code（未维护时回退交易所代码）；选文件后以文件头 MarketCode 二次核对"
              />
            </v-col>
          </v-row>

          <!-- 证券详情核对卡片 -->
          <v-card v-if="pickedSec" variant="tonal" class="mb-4">
            <v-card-title class="text-subtitle-1">
              <v-icon icon="mdi-check-decagram" class="mr-1" color="success" />
              所选证券（字典登记信息）
            </v-card-title>
            <v-card-text>
              <v-row dense>
                <v-col cols="6" md="3"><strong>usc：</strong>{{ pickedSec.usc }}</v-col>
                <v-col cols="6" md="3"><strong>名称：</strong>{{ pickedSec.security_name_cn }}</v-col>
                <v-col cols="6" md="3"><strong>源代码：</strong>{{ pickedSec.security_code }}</v-col>
                <v-col cols="6" md="3"><strong>类型：</strong>{{ pickedSec.security_type }}</v-col>
                <v-col cols="6" md="3"><strong>交易所：</strong>{{ fmtExchange(pickedSec.exchange_code) }}</v-col>
                <v-col cols="6" md="3"><strong>市场：</strong>{{ pickedSec.market_code ? fmtMarket(pickedSec.market_code) : '未维护' }}</v-col>
                <v-col cols="6" md="3"><strong>币种：</strong>{{ pickedSec.currency_type }}</v-col>
                <v-col cols="6" md="3"><strong>状态：</strong>
                  <v-chip :color="pickedSec.flag_enable === '1' ? 'success' : 'error'" size="small">
                    {{ pickedSec.flag_enable === '1' ? '启用' : '禁用' }}
                  </v-chip>
                </v-col>
              </v-row>
            </v-card-text>
          </v-card>
          <v-alert v-if="pickError" type="warning" class="mb-4" density="compact">
            {{ pickError }}
          </v-alert>

          <v-file-input
            label="选择 MVSV 行情文件"
            accept=".mvsv,.txt"
            prepend-icon="mdi-file"
            :loading="parsing"
            @change="onFileChange"
          />

          <!-- 策略 1 双向核对结果 -->
          <v-alert
            v-if="checkStatus !== 'idle'"
            :type="checkStatus === 'ok' ? 'success' : 'error'"
            class="mb-4"
            density="compact"
          >
            {{ checkDetail }}
          </v-alert>
        </template>

        <!-- 策略 2：先选文件自动匹配 -->
        <template v-else>
          <v-file-input
            label="先选择 MVSV 行情文件（将自动解析并匹配证券）"
            accept=".mvsv,.txt"
            prepend-icon="mdi-file-search"
            :loading="parsing"
            @change="onFileChange"
          />

          <v-alert v-if="fileHeader" type="info" class="mb-4" density="compact">
            文件头解析：证券代码 <strong>{{ fileHeader.code }}</strong>；市场代码
            <strong>{{ fileHeader.marketCode }}</strong>
          </v-alert>

          <!-- 匹配结果 -->
          <v-card v-if="matchedSec" variant="tonal" class="mb-4">
            <v-card-title class="text-subtitle-1">
              <v-icon icon="mdi-check-decagram" class="mr-1" color="success" />
              自动匹配到证券（已补全证券/市场代码）
            </v-card-title>
            <v-card-text>
              <v-row dense>
                <v-col cols="6" md="3"><strong>usc：</strong>{{ matchedSec.usc }}</v-col>
                <v-col cols="6" md="3"><strong>名称：</strong>{{ matchedSec.security_name_cn }}</v-col>
                <v-col cols="6" md="3"><strong>源代码：</strong>{{ matchedSec.security_code }}</v-col>
                <v-col cols="6" md="3"><strong>类型：</strong>{{ matchedSec.security_type }}</v-col>
                <v-col cols="6" md="3"><strong>交易所：</strong>{{ fmtExchange(matchedSec.exchange_code) }}</v-col>
                <v-col cols="6" md="3"><strong>市场：</strong>{{ matchedSec.market_code ? fmtMarket(matchedSec.market_code) : '未维护' }}</v-col>
                <v-col cols="6" md="3"><strong>币种：</strong>{{ matchedSec.currency_type }}</v-col>
              </v-row>
            </v-card-text>
          </v-card>
          <v-alert v-if="matchError" type="warning" class="mb-4" density="compact">
            {{ matchError }}
          </v-alert>

          <v-row>
            <v-col cols="12" md="6">
              <v-text-field
                v-model="secuCode"
                label="证券代码（自动补全）"
                readonly
                hint="匹配成功后自动补全，不可手工修改"
              />
            </v-col>
            <v-col cols="12" md="6">
              <v-text-field
                v-model="marketCode"
                label="市场代码（自动补全，取自文件头）"
                readonly
                hint="匹配成功后自动补全，不可手工修改"
              />
            </v-col>
          </v-row>
        </template>

        <v-text-field v-model="source" label="数据源" placeholder="如: cn-feed" />
        <v-text-field
          v-model="remark"
          label="备注"
          placeholder="导入备注（可选，写入行情行 remark 列）"
        />
        <v-select
          v-model="upsertMode"
          label="覆盖模式"
          :items="[
            { title: 'FIELD（只覆盖有值的字段，推荐）', value: 'FIELD' },
            { title: 'ROW（整行覆盖）', value: 'ROW' },
          ]"
        />
        <v-checkbox v-model="confirmChecked" label="我确认导入将覆盖同时刻同证券的对应字段值" />
        <v-btn
          type="submit"
          color="primary"
          :loading="importing"
          :disabled="!canUpload"
          prepend-icon="mdi-upload"
        >
          上传并导入
        </v-btn>
      </v-form>

      <v-alert v-if="error" type="error" class="mt-4" density="compact">
        {{ error }}
      </v-alert>
      <v-alert v-if="result" type="success" class="mt-4" density="compact">
        {{ result }}
      </v-alert>
    </v-card-text>
  </v-card>
</template>
