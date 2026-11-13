<script setup lang="ts">
// USC 映射规则维护（需求 11072）
//
// 三张表（迁移 V300157）：
//   futu.finv_futu_usc_rule       规则层（9 条，只读展示）
//   futu.finv_futu_usc_rule_item  条目层（689 条，可新增/编辑/停用）
//   futu.finv_futu_usc_map        结果表（ftmm_symbol → usc；退市行允许 usc 复用）
// 另有审计日志 futu.finv_futu_usc_rule_log 与复用归档 archive.finv_usc_reuse_snapshot。
//
// 接口（前缀 /Meta/Finv/Quote/UscMap）：
//   GET  /Status          只读快照（规则/条目/结果表/键表匹配率/cron）
//   GET  /Rule/List       规则列表（含条目数）
//   GET  /Item/List       条目分页
//   POST /Item/Save       新增或编辑条目（item_desc 必填）
//   POST /Item/Retire     停用条目
//   GET  /Diff/Preview    预演差异（不落库，返回 preview_token）
//   POST /Sync/Run        异步应用（带预演令牌）
//   GET  /Sync/Run        运行时快照（进度 + 日志）
//   GET  /Log/List        审计日志分页
//
// 交互约束：应用前必须先预演（后端校验 preview_token）；停用/应用都要填原因。
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import { apiGet, apiPost } from '../../../../../api'
import { showToast } from '../../../../../ui/toast'

const BASE = '/Meta/Finv/Quote/UscMap'

// ---------- 枚举下拉选项（值 + 中文翻译） ----------
//
// 取值以数据库 CHECK 约束与存量数据为准：
//   action      → finv_futu_usc_rule_item_action_chk（CODE/FTMM/LITERAL/REWRITE/SH4）
//   status      → finv_futu_usc_rule_item_status_chk（ACTIVE/PENDING/RETIRED）
//   instrument  → futu.finv_futu_secu_key.instrument 实际出现过的 8 种
const ACTION_OPTIONS = [
  { title: '取证券代码[CODE]', value: 'CODE' },
  { title: '取富途全码[FTMM]', value: 'FTMM' },
  { title: '取固定字面量[LITERAL]', value: 'LITERAL' },
  { title: '按正则替换[REWRITE]', value: 'REWRITE' },
  { title: '沪市指数四位码[SH4]', value: 'SH4' },
]
const ACTION_LABELS: Record<string, string> = {
  CODE: '取证券代码',
  FTMM: '取富途全码',
  LITERAL: '取固定字面量',
  REWRITE: '按正则替换',
  SH4: '沪市指数四位码',
}

const STATUS_OPTIONS = [
  { title: '生效[ACTIVE]', value: 'ACTIVE' },
  { title: '待生效[PENDING]', value: 'PENDING' },
  { title: '已停用[RETIRED]', value: 'RETIRED' },
]
const STATUS_LABELS: Record<string, string> = {
  ACTIVE: '生效',
  PENDING: '待生效',
  RETIRED: '已停用',
}

const INSTRUMENT_OPTIONS = [
  { title: '股票[stock]', value: 'stock' },
  { title: 'ETF / 交易所交易基金[etf]', value: 'etf' },
  { title: '指数[index]', value: 'index' },
  { title: '板块 / 行业分类[sector]', value: 'sector' },
  { title: '大盘 / 市场整体[market]', value: 'market' },
  { title: '加密货币[crypto]', value: 'crypto' },
  { title: '外汇[forex]', value: 'forex' },
  { title: '期货[futures]', value: 'futures' },
]
const INSTRUMENT_LABELS: Record<string, string> = {
  stock: '股票',
  etf: 'ETF',
  index: '指数',
  sector: '板块',
  market: '大盘',
  crypto: '加密货币',
  forex: '外汇',
  futures: '期货',
}

// 市场（富途 market_label）：取自 futu.finv_futu_secu_key.market 实际出现过的 16 种，
// 中文名对齐 public.finv_market.market_name / futu.finv_futu_mapping_market_code.market_name
// （一个市场标签对应多个细分market_code，这里只保留该标签的中文口径）。
const MARKET_OPTIONS = [
  { title: '美股[US]', value: 'US' },
  { title: '沪市（上交所）[SH]', value: 'SH' },
  { title: '深市（深交所）[SZ]', value: 'SZ' },
  { title: '北交所[BJ]', value: 'BJ' },
  { title: '港股[HK]', value: 'HK' },
  { title: '加拿大[CA]', value: 'CA' },
  { title: '澳大利亚[AU]', value: 'AU' },
  { title: '日本[JP]', value: 'JP' },
  { title: '新加坡[SG]', value: 'SG' },
  { title: '马来西亚[MY]', value: 'MY' },
  { title: '泰国[THA]', value: 'THA' },
  { title: '加密货币[CC]', value: 'CC' },
  { title: '外汇[FX]', value: 'FX' },
  { title: '板块[BK]', value: 'BK' },
  { title: '债券[BMS]', value: 'BMS' },
  { title: '全球[GL]', value: 'GL' },
]
const MARKET_LABELS: Record<string, string> = Object.fromEntries(
  MARKET_OPTIONS.map((o) => [o.value, o.title.slice(0, o.title.indexOf('['))]),
)

// ---------- 类型 ----------
interface Rule {
  rule_no: string
  rule_name: string
  rule_type: string
  priority: number
  match_scope: string
  default_action?: string | null
  flag_enable: string
  rule_desc: string
  remark: string
  item_count: number
  item_active: number
  gmt_update: string
}

interface Item {
  idn: number
  rule_no: string
  market?: string | null
  instrument?: string | null
  code?: string | null
  code_regex?: string | null
  action?: string | null
  usc_literal?: string | null
  rewrite_tpl?: string | null
  status: string
  effective_from?: string | null
  effective_to?: string | null
  priority: number
  scope_env: string
  item_desc: string
  source_ref: string
  remark: string
  gmt_update: string
}

interface DiffRow {
  ftmm_symbol: string
  market: string
  code: string
  instrument: string
  current_usc?: string | null
  new_usc?: string | null
  rule_no: string
  basis: string
  diff: string
  reuse_of_ftmm?: string | null
}

interface Preview {
  env: string
  rule_version: string
  dt_compute: string
  total: number
  insert: number
  update: number
  retire: number
  reuse: number
  keep: number
  escalate: number
  key_update: number
  preview_token: string
  samples: DiffRow[]
  conflicts: string[]
}

// 结果表（futu.finv_futu_usc_map）一行：随时查询在册映射，不依赖预演缓存
interface MapRow {
  ftmm_symbol: string
  market: string
  code: string
  instrument: string
  usc: string
  rule_no: string
  basis: string
  status: string
  scope_env: string
  rule_version: string
  dt_compute: string
  dt_retire: string | null
  reuse_of_ftmm: string | null
}

interface RunLog {
  seq: number
  time: string
  level: string
  text: string
}

interface Run {
  run_id: string
  status: string
  trigger: string
  env: string
  operator: string
  rule_version: string
  insert: number
  update: number
  retire: number
  reuse: number
  keep: number
  escalate: number
  archive: number
  key_update: number
  start_at: string
  finish_at?: string
  duration_ms: number
  error?: string
  logs: RunLog[]
}

interface Status {
  env: string
  rule_version: string
  rule_count: number
  item_count: number
  item_active: number
  map_active: number
  map_retire: number
  map_reuse: number
  key_total: number
  key_match: number
  key_miss: number
  last_compute_at: string
  archive_rows: number
  cron_expr: string
  cron_next_run: string
  target_tables: string[]
}

interface ScheduleConfig {
  cron_expr: string
  enable: boolean
  remark: string
  gmt_modify?: string | null
  next_run: string[]
}

interface AuditLog {
  idn: number
  src_table: string
  action: string
  key_text: string
  // 审计对象的键（后端已拆好，页面分「主键 / 所属规则」两列展示；需求 11137）
  src_idn: string | null
  src_rule_no: string | null
  src_ftmm: string | null
  before_json?: string | null
  after_json?: string | null
  operator: string
  reason: string
  gmt_create: string
}

// ---------- 状态 ----------
const tab = ref<'rules' | 'items' | 'diff' | 'logs'>('rules')
const loading = ref(false)
const status = ref<Status | null>(null)

const rules = ref<Rule[]>([])
const selectedRule = ref<string>('')
const items = ref<Item[]>([])
const itemTotal = ref(0)
const itemPage = ref(1)
const itemPageSize = ref(20)
// 动作联动：LITERAL 才填「固定取值」，REWRITE 才填「代码正则 + 替换模板」，
// 其余动作（CODE/FTMM/SH4）由规则自己算，不需要任何取值。避免把四个输入框全摊给用户。
const isLiteralAction = computed(() => editForm.value.action === 'LITERAL')
const isRewriteAction = computed(() => editForm.value.action === 'REWRITE')
const actionHint = computed(() => {
  switch (editForm.value.action) {
    case 'CODE':
      return '取证券代码（前处理后的 canon_code），例如 000300 → SH0300 之外的市场直接就是代码；无需填取值。'
    case 'FTMM':
      return '取富途全码（market.code），例如 CC.AMPL；无需填取值。'
    case 'LITERAL':
      return '固定取值：不管代码是什么，usc 都写成你在下面「固定取值」里填的那串字面量。'
    case 'REWRITE':
      return '正则改写：用「代码正则」匹配原代码，再用「替换模板」生成新 usc（捕获组用 $1、$2 引用）。'
    case 'SH4':
      return '沪市指数四位码：000300 → SH0300（SH + 后四位）；无需填取值。'
    default:
      return '未指定动作时按所属规则的默认动作执行（P40/P50 默认是 FTMM 富途全码）。'
  }
})

const filterMarket = ref('')

// 条目说明（item_desc）：默认按「范围 + 匹配键 + 动作效果 + 依据规则」自动生成（需求 11136）；
// 用户手动改过（descTouched）就不再覆盖，可用字段右侧「重新生成」恢复自动值。
const filterStatus = ref('')
const filterKeyword = ref('')

const preview = ref<Preview | null>(null)
const previewing = ref(false)

// 预演明细分页：预演只回前 200 行，完整明细由后端缓存（30 分钟 / 重新预演失效），
// 页面通过 /Diff/Page 分页浏览，避免「说有 7 万条却只能看到一页」。
const diffRows = ref<DiffRow[]>([])
const diffTotal = ref(0)
// 明细/结果表查询：关键字同时匹配 ftmm_symbol / code / usc，几万条里直接定位一只证券
const diffKeyword = ref('')
const diffMode = ref<'preview' | 'map'>('preview')
const mapRows = ref<MapRow[]>([])
const mapTotal = ref(0)
const mapPage = ref(1)
const mapPageSize = ref(50)
const mapKeyword = ref('')
const mapStatus = ref('')
const mapLoading = ref(false)
const diffPage = ref(1)
const diffPageSize = ref(50)
const diffFilter = ref('')
const diffLoading = ref(false)
const diffHint = ref('')

// 定时同步（sys_config 20015）：页面可直接改 cron 或关闭自动同步；「立即执行」= 预演+应用一次，不用等定时
const schedule = ref<ScheduleConfig | null>(null)
const scheduleExpr = ref('')
const scheduleSaving = ref(false)
const run = ref<Run | null>(null)
const starting = ref(false)
let pollTimer: number | null = null

const logs = ref<AuditLog[]>([])
const logTotal = ref(0)
const logPage = ref(1)
// 审计日志「变更」详情对话框：完整 before/after JSON 格式化展示（原先只显示被截断的原始串）
const logDetailOpen = ref(false)
const logDetail = ref<AuditLog | null>(null)

// 编辑对话框
const editOpen = ref(false)
const editSaving = ref(false)
const editForm = ref<Item>(emptyItem())
const editIsNew = ref(true)

const descTouched = ref(false)

function buildItemDesc(): string {
  const f = editForm.value
  const marketLabel = (f.market ? (MARKET_LABELS[f.market] ?? f.market) : '').trim()
  const instrumentLabel = (f.instrument ? (INSTRUMENT_LABELS[f.instrument] ?? f.instrument) : '').trim()
  // 市场与品种中文同名时（如 CC 与 crypto 都叫「加密货币」）不重复一次
  const scopeParts = marketLabel === instrumentLabel ? [marketLabel] : [marketLabel, instrumentLabel]
  const scope = scopeParts.filter(Boolean).join(' ') || '全部市场 / 全部品种'

  const code = (f.code ?? '').trim()
  const regex = (f.code_regex ?? '').trim()
  const tpl = (f.rewrite_tpl ?? '').trim()
  const literal = (f.usc_literal ?? '').trim()

  let key: string
  if (f.action === 'REWRITE') {
    key = `代码正则 ${regex || '（未填）'} → ${tpl || '（未填）'}`
  } else {
    key = code ? `代码 ${code}` : '该范围下全部代码'
  }

  let effect: string
  switch (f.action) {
    case 'CODE':
      effect = 'usc 取交易所代码'
      break
    case 'FTMM':
      effect = `usc 取富途全码${f.market && code ? `（${f.market}.${code}）` : '（市场.代码）'}`
      break
    case 'LITERAL':
      effect = `usc 固定取 ${literal || '（未填）'}`
      break
    case 'REWRITE':
      effect = `usc 按正则改写${tpl ? `（${tpl}）` : ''}`
      break
    case 'SH4':
      effect = 'usc 取 SH + 代码后四位'
      break
    default:
      effect = '沿用规则默认动作'
  }

  const rule = rules.value.find((r) => r.rule_no === f.rule_no)
  const basis = rule ? `依据 ${rule.rule_no} ${rule.rule_name}` : `依据 ${f.rule_no || '（未选规则）'}`
  return `${scope}｜${key}｜${effect}｜${basis}`
}

// 表单里任一影响取值的字段变了，且用户没手改过说明，就重新生成
watch(
  () => [
    editForm.value.rule_no,
    editForm.value.market,
    editForm.value.instrument,
    editForm.value.code,
    editForm.value.code_regex,
    editForm.value.rewrite_tpl,
    editForm.value.usc_literal,
    editForm.value.action,
  ],
  () => {
    if (editOpen.value && !descTouched.value) editForm.value.item_desc = buildItemDesc()
  },
)

function regenerateDesc() {
  descTouched.value = false
  editForm.value.item_desc = buildItemDesc()
}


// 停用对话框
const retireOpen = ref(false)
const retireReason = ref('')
const retireTarget = ref<Item | null>(null)

// 应用对话框
const applyOpen = ref(false)
const applyReason = ref('')

function emptyItem(): Item {
  return {
    idn: 0,
    rule_no: selectedRule.value || 'P50',
    market: null,
    instrument: null,
    code: null,
    code_regex: null,
    action: null,
    usc_literal: null,
    rewrite_tpl: null,
    status: 'ACTIVE',
    effective_from: null,
    effective_to: null,
    priority: 100,
    scope_env: '*',
    item_desc: '',
    source_ref: '',
    remark: '',
    gmt_update: '',
  }
}

const cronText = computed(() => {
  if (!status.value) return ''
  if (!status.value.cron_expr) return '已关闭（空）'
  return status.value.cron_expr
})

const matchRate = computed(() => {
  if (!status.value || status.value.key_total === 0) return 0
  return Math.round((status.value.key_match / status.value.key_total) * 1000) / 10
})

const hasDiff = computed(() => {
  if (!preview.value) return false
  return (
    preview.value.insert + preview.value.update + preview.value.retire +
    preview.value.reuse + preview.value.key_update > 0
  )
})

// ---------- 读取 ----------
async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    showToast('已复制到剪贴板', 'success')
  } catch {
    showToast('复制失败，请手动选中复制', 'warning')
  }
}

async function loadStatus() {
  try {
    status.value = await apiGet<Status>(`${BASE}/Status`)
  } catch (err) {
    showToast(`读取状态失败：${(err as Error).message}`, 'error')
  }
}

async function loadRules() {
  try {
    const data = await apiGet<{ list: Rule[] }>(`${BASE}/Rule/List`)
    rules.value = data.list ?? []
  } catch (err) {
    showToast(`读取规则失败：${(err as Error).message}`, 'error')
  }
}

// ---------- 规则层编辑（启用开关 / 默认动作 / 说明 / 备注） ----------
// 规则号、类型、同层顺序、作用域决定引擎分层与层序，页面不给改（改错整层结果漂移），需要时走迁移。
const ruleEditOpen = ref(false)
const ruleSaving = ref('')
const ruleForm = ref<Rule | null>(null)

function openRuleEdit(rule: Rule) {
  ruleForm.value = { ...rule }
  ruleEditOpen.value = true
}

function rulePayload(rule: Rule) {
  return {
    rule_no: rule.rule_no,
    flag_enable: rule.flag_enable,
    default_action: nullable(rule.default_action),
    rule_desc: (rule.rule_desc ?? '').trim(),
    remark: (rule.remark ?? '').trim(),
  }
}

async function toggleRule(rule: Rule, enabled: boolean) {
  ruleSaving.value = rule.rule_no
  try {
    await apiPost(`${BASE}/Rule/Save`, {
      ...rulePayload(rule),
      flag_enable: enabled ? '1' : '0',
      reason: enabled ? 'Web 维护页启用该层' : 'Web 维护页停用该层',
    })
    showToast(enabled ? `已启用 ${rule.rule_no}` : `已停用 ${rule.rule_no}`, 'success')
  } catch (err) {
    showToast(`切换失败：${(err as Error).message}`, 'error')
  } finally {
    ruleSaving.value = ''
    await Promise.all([loadRules(), loadStatus()])
  }
}

async function saveRule() {
  const rule = ruleForm.value
  if (!rule) return
  ruleSaving.value = rule.rule_no
  try {
    await apiPost(`${BASE}/Rule/Save`, { ...rulePayload(rule), reason: 'Web 维护页编辑规则' })
    showToast(`规则 ${rule.rule_no} 已保存`, 'success')
    ruleEditOpen.value = false
    await Promise.all([loadRules(), loadStatus()])
  } catch (err) {
    showToast(`保存失败：${(err as Error).message}`, 'error')
  } finally {
    ruleSaving.value = ''
  }
}

async function loadItems() {
  const query = new URLSearchParams({
    page: String(itemPage.value),
    pageSize: String(itemPageSize.value),
  })
  if (selectedRule.value) query.set('ruleNo', selectedRule.value)
  if (filterMarket.value.trim()) query.set('market', filterMarket.value.trim())
  if (filterStatus.value) query.set('status', filterStatus.value)
  if (filterKeyword.value.trim()) query.set('keyword', filterKeyword.value.trim())
  try {
    const data = await apiGet<{ list: Item[]; total: number }>(`${BASE}/Item/List?${query.toString()}`)
    items.value = data.list ?? []
    itemTotal.value = data.total ?? 0
  } catch (err) {
    showToast(`读取条目失败：${(err as Error).message}`, 'error')
  }
}

async function loadSchedule() {
  try {
    schedule.value = await apiGet<ScheduleConfig>(`${BASE}/Schedule`)
    scheduleExpr.value = schedule.value.cron_expr
  } catch (err) {
    showToast(`读取定时同步配置失败：${(err as Error).message}`, 'error')
  }
}

async function saveSchedule(expr: string) {
  scheduleSaving.value = true
  try {
    schedule.value = await apiPost<ScheduleConfig>(`${BASE}/Schedule`, { cron_expr: expr })
    scheduleExpr.value = schedule.value.cron_expr
    showToast(
      schedule.value.enable
        ? `已启用：${schedule.value.cron_expr}（下次 ${schedule.value.next_run[0] ?? '—'}）`
        : '已关闭自动同步（仍可随时「立即执行」）',
      'success',
    )
    await loadStatus()
  } catch (err) {
    showToast(`保存失败：${(err as Error).message}`, 'error')
  } finally {
    scheduleSaving.value = false
  }
}

async function loadLogs() {
  try {
    const data = await apiGet<{ list: AuditLog[]; total: number }>(
      `${BASE}/Log/List?page=${logPage.value}&pageSize=20`,
    )
    logs.value = data.list ?? []
    logTotal.value = data.total ?? 0
  } catch (err) {
    showToast(`读取审计日志失败：${(err as Error).message}`, 'error')
  }
}

async function loadAll() {
  loading.value = true
  try {
    await Promise.all([loadStatus(), loadRules(), loadItems(), loadLogs(), loadSchedule()])
  } finally {
    loading.value = false
  }
}

// 规则页「查看条目」：带上该规则过滤切到条目页
function selectRule(ruleNo: string) {
  selectedRule.value = ruleNo
  itemPage.value = 1
  tab.value = 'items'
  void loadItems()
}


// ---------- 条目编辑 ----------
function openCreate() {
  editIsNew.value = true
  editForm.value = emptyItem()
  descTouched.value = false
  editForm.value.item_desc = buildItemDesc()
  editOpen.value = true
}

function openEdit(item: Item) {
  editIsNew.value = false
  editForm.value = { ...item }
  // 既有条目的说明是用户/历史数据，不覆盖；需要时点「重新生成」
  descTouched.value = true
  editOpen.value = true
}

async function saveItem() {
  const form = editForm.value
  if (!form.item_desc.trim()) {
    showToast('条目说明（item_desc）不能为空', 'warning')
    return
  }
  if (form.action === 'LITERAL' && !(form.usc_literal ?? '').trim()) {
    showToast('动作为「固定取值（LITERAL）」时，「固定取值 usc」不能为空', 'warning')
    return
  }
  if (form.action === 'REWRITE' && (!(form.code_regex ?? '').trim() || !(form.rewrite_tpl ?? '').trim())) {
    showToast('动作为「正则改写（REWRITE）」时，「代码正则」与「替换模板」都不能为空', 'warning')
    return
  }
  editSaving.value = true
  try {
    const payload = {
      ...form,
      idn: editIsNew.value ? 0 : form.idn,
      market: nullable(form.market),
      instrument: nullable(form.instrument),
      // 按动作只提交相关取值，其余置空（切过动作的旧值不带上，避免落库后自相矛盾）
      code: form.action === 'REWRITE' ? null : nullable(form.code),
      code_regex: form.action === 'REWRITE' ? nullable(form.code_regex) : null,
      action: nullable(form.action),
      usc_literal: form.action === 'LITERAL' ? nullable(form.usc_literal) : null,
      rewrite_tpl: form.action === 'REWRITE' ? nullable(form.rewrite_tpl) : null,
      effective_from: nullable(form.effective_from),
      effective_to: nullable(form.effective_to),
    }
    await apiPost(`${BASE}/Item/Save`, payload)
    showToast(editIsNew.value ? '条目已新增' : '条目已保存', 'success')
    editOpen.value = false
    await Promise.all([loadItems(), loadRules()])
  } catch (err) {
    showToast(`保存失败：${(err as Error).message}`, 'error')
  } finally {
    editSaving.value = false
  }
}

function openRetire(item: Item) {
  retireTarget.value = item
  retireReason.value = ''
  retireOpen.value = true
}

async function confirmRetire() {
  if (!retireTarget.value) return
  if (!retireReason.value.trim()) {
    showToast('请填写停用原因', 'warning')
    return
  }
  try {
    await apiPost(`${BASE}/Item/Retire`, {
      idn: retireTarget.value.idn,
      reason: retireReason.value.trim(),
    })
    showToast('条目已停用（保留痕迹）', 'success')
    retireOpen.value = false
    await Promise.all([loadItems(), loadRules()])
  } catch (err) {
    showToast(`停用失败：${(err as Error).message}`, 'error')
  }
}

// ---------- 预演与应用 ----------
async function doPreview() {
  previewing.value = true
  try {
    preview.value = await apiGet<Preview>(`${BASE}/Diff/Preview`)
    if (preview.value.conflicts.length > 0) {
      showToast('预演发现 usc 冲突，应用会被拒绝', 'error')
    } else {
      showToast(
        `预演完成：新增 ${preview.value.insert}、更新 ${preview.value.update}、退市 ${preview.value.retire}、复用 ${preview.value.reuse}`,
        'success',
      )
    }
    diffFilter.value = ''
    diffPage.value = 1
    await loadDiffPage()
  } catch (err) {
    showToast(`预演失败：${(err as Error).message}`, 'error')
  } finally {
    previewing.value = false
  }
}

// 结果表查询：任何时刻都能用（不需要先预演）
async function loadMapPage() {
  mapLoading.value = true
  try {
    const query = new URLSearchParams({
      page: String(mapPage.value),
      pageSize: String(mapPageSize.value),
    })
    if (mapKeyword.value.trim()) query.set('keyword', mapKeyword.value.trim())
    if (mapStatus.value) query.set('status', mapStatus.value)
    const data = await apiGet<{ list: MapRow[]; total: number }>(`${BASE}/Map/Page?${query.toString()}`)
    mapRows.value = data.list ?? []
    mapTotal.value = data.total ?? 0
  } catch (err) {
    showToast(`查询结果表失败：${(err as Error).message}`, 'error')
  } finally {
    mapLoading.value = false
  }
}

async function switchDiffMode(mode: 'preview' | 'map') {
  diffMode.value = mode
  if (mode === 'map' && mapRows.value.length === 0) {
    mapPage.value = 1
    await loadMapPage()
  }
}

async function loadDiffPage() {
  if (!preview.value) return
  const query = new URLSearchParams({
    page: String(diffPage.value),
    pageSize: String(diffPageSize.value),
    token: preview.value.preview_token,
  })
  if (diffFilter.value) query.set('diff', diffFilter.value)
  if (diffKeyword.value.trim()) query.set('keyword', diffKeyword.value.trim())
  diffLoading.value = true
  try {
    const data = await apiGet<{ list: DiffRow[]; total: number }>(`${BASE}/Diff/Page?${query.toString()}`)
    diffRows.value = data.list ?? []
    diffTotal.value = data.total ?? 0
    diffHint.value = ''
  } catch (err) {
    diffRows.value = []
    diffTotal.value = 0
    diffHint.value = (err as Error).message
  } finally {
    diffLoading.value = false
  }
}

// runNow 定时任务的「立即执行」：先预演拿到令牌（顺便把差异展示出来），再进入确认应用。
async function runNow() {
  if (starting.value) return
  await doPreview()
  if (!preview.value) return
  if (!hasDiff.value) {
    showToast('当前没有差异，无需应用', 'info')
    return
  }
  openApply()
}

function openApply() {
  if (!preview.value) {
    showToast('请先预演', 'warning')
    return
  }
  applyReason.value = ''
  applyOpen.value = true
}

async function confirmApply() {
  if (!preview.value) return
  starting.value = true
  try {
    const started = await apiPost<Run>(`${BASE}/Sync/Run`, {
      preview_token: preview.value.preview_token,
      reason: applyReason.value.trim(),
    })
    run.value = started
    applyOpen.value = false
    startPolling(started.run_id)
  } catch (err) {
    showToast(`启动失败：${(err as Error).message}`, 'error')
    starting.value = false
  }
}

function startPolling(runId: string) {
  stopPolling()
  pollTimer = window.setInterval(async () => {
    try {
      const snapshot = await apiGet<Run>(`${BASE}/Sync/Run?runId=${encodeURIComponent(runId)}`)
      run.value = snapshot
      if (snapshot.status !== 'RUNNING') {
        stopPolling()
        starting.value = false
        if (snapshot.status === 'DONE') {
          showToast(
            `应用完成：新增 ${snapshot.insert}、更新 ${snapshot.update}、退市 ${snapshot.retire}、复用 ${snapshot.reuse}、回写键表 ${snapshot.key_update}`,
            'success',
          )
          preview.value = null
          await Promise.all([loadStatus(), loadLogs(), loadItems()])
        } else {
          showToast(`应用失败：${snapshot.error ?? '未知错误'}`, 'error')
        }
      }
    } catch (err) {
      stopPolling()
      starting.value = false
      showToast(`读取运行状态失败：${(err as Error).message}`, 'error')
    }
  }, 800)
}

function stopPolling() {
  if (pollTimer !== null) {
    window.clearInterval(pollTimer)
    pollTimer = null
  }
}

// ---------- 工具 ----------
// 审计日志详情两种视图：摘要（字段级变更）/ 对比（整行并排）；点行默认对比（需求 11141）
const logView = ref<'summary' | 'compare'>('compare')

function openLogDetail(row: AuditLog, view: 'summary' | 'compare' = 'compare') {
  logDetail.value = row
  logView.value = view
  logDetailOpen.value = true
}

// logDiff 把 before/after 两个 jsonb 摊成字段级差异（只留真正变化的字段）
const logDiff = computed<{ field: string; before: string; after: string }[]>(() => {
  const row = logDetail.value
  if (!row) return []
  const parse = (raw: string | null | undefined): Record<string, unknown> => {
    if (!raw) return {}
    try {
      const value = JSON.parse(raw) as unknown
      return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : {}
    } catch {
      return {}
    }
  }
  const show = (value: unknown): string => {
    if (value === null || value === undefined) return '（空）'
    if (typeof value === 'object') return JSON.stringify(value)
    return String(value)
  }
  const before = parse(row.before_json)
  const after = parse(row.after_json)
  const keys = Array.from(new Set([...Object.keys(before), ...Object.keys(after)])).sort()
  return keys
    .map((field) => ({ field, before: show(before[field]), after: show(after[field]) }))
    .filter((diff) => diff.before !== diff.after)
})

// prettyJSON 把库里的 jsonb 文本格式化成缩进 2 空格的多行 JSON；解析失败原样返回。
function prettyJSON(raw?: string | null): string {
  if (!raw) return '（无）'
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

// changeSummary 列表里显示的简短摘要：字段名列表 + 关键值，避免整串 JSON 塞进表格。
// 审计日志「主键」：条目行 = idn，结果表行 = ftmm_symbol，规则行 = rule_no（需求 11137 拆列）
function logPrimaryKey(row: AuditLog): string {
  if (row.src_idn) return row.src_idn
  if (row.src_ftmm) return row.src_ftmm
  if (row.src_rule_no) return row.src_rule_no
  return row.key_text || '—'
}

// 审计日志「所属规则」：规则行自身就是规则，不重复显示
function logRuleNo(row: AuditLog): string {
  if (row.src_table === 'rule') return '—'
  return row.src_rule_no || '—'
}

function changeSummary(row: AuditLog): string {
  const parts: string[] = []
  for (const [label, raw] of [
    ['前', row.before_json],
    ['后', row.after_json],
  ] as const) {
    if (!raw) continue
    try {
      const obj = JSON.parse(raw) as Record<string, unknown>
      const keys = Object.keys(obj)
      const brief = keys.slice(0, 6).map((k) => {
        const value = obj[k]
        const text = typeof value === 'object' && value !== null ? '…' : String(value)
        return `${k}=${text.length > 18 ? `${text.slice(0, 18)}…` : text}`
      })
      parts.push(`${label}: ${brief.join(', ')}${keys.length > 6 ? ` 等 ${keys.length} 个字段` : ''}`)
    } catch {
      parts.push(`${label}: ${raw.length > 60 ? `${raw.slice(0, 60)}…` : raw}`)
    }
  }
  return parts.length > 0 ? parts.join(' ｜ ') : '—'
}

// v-combobox 默认 `return-object=true`（Vuetify 4.1），从下拉里选中对象型选项时
// v-model 收到的是 { title, value } 整个对象；直接 String(obj) 会落库成 "[object Object]"（需求 11135）。
// 这里统一把「对象/字符串/空」归一成去空白的字符串，取值优先 value，其次 title。
function textValue(value: unknown): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'object') {
    const obj = value as Record<string, unknown>
    return textValue(obj.value ?? obj.title ?? '')
  }
  return String(value).trim()
}

function nullable(value: unknown): string | null {
  const text = textValue(value)
  return text === '' ? null : text
}

// 差异类别：中文在前、英文值在方括号里（需求 11099 的统一风格）
const DIFF_LABELS: Record<string, string> = {
  insert: '新增',
  update: '更新',
  retire: '退市',
  reuse: '复用',
  keep: '未变',
  key_update: '键表回写',
}

// 表格单元格只显示字典的「值」（中文），不带键；`中文[值]` 只用于下拉选项。
function diffText(diff: string): string {
  return DIFF_LABELS[diff] ?? diff
}

function diffColor(diff: string): string {
  switch (diff) {
    case 'insert':
      return 'success'
    case 'update':
      return 'warning'
    case 'retire':
      return 'error'
    case 'keep':
      return 'grey'
    case 'key_update':
      return 'info'
    default:
      return 'primary'
  }
}

// 表格列只显示字典的「值」（中文）；键留给下拉选项与悬浮提示，不占表格空间。
// 未登记的值原样显示，避免信息丢失。
function actionText(action?: string | null): string {
  if (!action) return '（规则默认）'
  return ACTION_LABELS[action] ?? action
}

function statusText(value?: string | null): string {
  if (!value) return '—'
  return STATUS_LABELS[value] ?? value
}

function marketText(value?: string | null): string {
  if (!value) return '—'
  return MARKET_LABELS[value] ?? value
}

function instrumentText(value?: string | null): string {
  if (!value) return '—'
  return INSTRUMENT_LABELS[value] ?? value
}

function statusColor(value: string): string {
  if (value === 'ACTIVE') return 'success'
  if (value === 'PENDING') return 'warning'
  return 'grey'
}

onMounted(loadAll)
onBeforeUnmount(stopPolling)
</script>

<template>
  <v-card>
    <v-card-title class="d-flex align-center flex-wrap ga-2">
      <v-icon icon="mdi-transit-connection-variant" class="mr-2" />
      USC 映射规则
      <v-chip v-if="status" size="small" variant="tonal" prepend-icon="mdi-source-branch">
        规则版本 {{ status.rule_version }}
      </v-chip>
      <v-chip v-if="status" size="small" variant="tonal" prepend-icon="mdi-format-list-numbered">
        条目 {{ status.item_active }} / {{ status.item_count }}
      </v-chip>
      <v-chip v-if="status" size="small" variant="tonal" prepend-icon="mdi-check-decagram-outline">
        键表匹配 {{ matchRate }}%
      </v-chip>
      <v-chip v-if="status" size="small" variant="tonal" prepend-icon="mdi-timer-outline">
        定时同步 {{ cronText }}
      </v-chip>
      <v-spacer />
      <v-btn variant="text" prepend-icon="mdi-refresh" :loading="loading" @click="loadAll">刷新</v-btn>
    </v-card-title>

    <v-tabs v-model="tab" density="compact" color="primary">
      <v-tab value="rules">规则</v-tab>
      <v-tab value="items">条目</v-tab>
      <v-tab value="diff">预演与应用</v-tab>
      <v-tab value="logs">审计日志</v-tab>
    </v-tabs>

    <v-card-text>
      <!-- Tab 1：规则 + 条目 -->
      <v-window v-model="tab">
        <!-- Tab 1：规则层（只读视图 + 看条目入口） -->
        <v-window-item value="rules">
          <div class="d-flex flex-wrap align-center ga-2 mb-2">
            <v-chip variant="tonal" prepend-icon="mdi-source-branch">共 {{ rules.length }} 条规则</v-chip>
            <span class="text-caption text-medium-emphasis">
              条目数显示「生效 / 全部」；点击规则行（或「查看条目」）会切到「条目」页并自动带上该规则过滤；
              「说明」列是长文本，鼠标悬浮 ⓘ 图标即可查看完整说明与备注。
            </span>
            <v-spacer />
            <v-btn size="small" variant="text" prepend-icon="mdi-refresh" @click="loadRules">刷新</v-btn>
          </div>

          <v-table density="compact" class="usc-table">
            <thead>
              <tr>
                <th>规则</th>
                <th>名称</th>
                <th>类型</th>
                <th>同层顺序</th>
                <th>作用域</th>
                <th>默认动作</th>
                <th>启用</th>
                <th class="text-right">生效条目数</th>
                <th class="text-right">全部条目数</th>
                <th>说明</th>
                <th class="text-right">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="rule in rules" :key="rule.rule_no">
                <td>
                  <v-chip size="x-small" variant="flat">{{ rule.rule_no }}</v-chip>
                </td>
                <td class="text-no-wrap">{{ rule.rule_name }}</td>
                <td>{{ rule.rule_type }}</td>
                <td>{{ rule.priority }}</td>
                <td>{{ rule.match_scope }}</td>
                <td>{{ actionText(rule.default_action) }}</td>
                <td class="text-no-wrap">
                  <v-tooltip location="top" max-width="360" open-delay="120">
                    <template #activator="{ props }">
                      <v-switch
                        v-bind="props"
                        :model-value="rule.flag_enable === '1'"
                        :color="rule.flag_enable === '1' ? 'green' : 'grey'"
                        :loading="ruleSaving === rule.rule_no"
                        density="compact"
                        hide-details
                        @update:model-value="(v: unknown) => toggleRule(rule, Boolean(v))"
                      />
                    </template>
                    {{ rule.flag_enable === '1' ? '点击停用该层（条目不变，只是不再参与计算）' : '点击启用该层' }}
                  </v-tooltip>
                </td>
                <td class="text-right text-no-wrap">{{ rule.item_active }}</td>
                <td class="text-right text-no-wrap">{{ rule.item_count }}</td>
                <td>
                  <!-- 说明/备注是长文本：改用图标 + 悬浮提示，避免把表格撑高变形 -->
                  <v-tooltip location="top" max-width="560" open-delay="120">
                    <template #activator="{ props }">
                      <v-icon
                        v-bind="props"
                        :color="rule.rule_desc ? 'primary' : 'grey'"
                        :icon="rule.rule_desc ? 'mdi-information-outline' : 'mdi-information-off-outline'"
                        size="small"
                      />
                    </template>
                    <div class="font-weight-medium mb-1">{{ rule.rule_no }} {{ rule.rule_name }}</div>
                    <div class="text-caption" style="white-space: pre-wrap">
                      {{ rule.rule_desc || '（未填写规则说明）' }}
                    </div>
                    <div v-if="rule.remark" class="text-caption mt-2 text-medium-emphasis" style="white-space: pre-wrap">
                      备注：{{ rule.remark }}
                    </div>
                  </v-tooltip>
                </td>
                <td class="text-right text-no-wrap">
                  <v-btn size="x-small" variant="text" prepend-icon="mdi-pencil" @click="openRuleEdit(rule)">编辑</v-btn>
                  <v-btn size="x-small" variant="text" @click="selectRule(rule.rule_no)">查看条目</v-btn>
                </td>
              </tr>
              <tr v-if="rules.length === 0">
                <td colspan="11" class="text-center text-medium-emphasis">加载中…</td>
              </tr>
            </tbody>
          </v-table>
        </v-window-item>

        <!-- Tab 2：条目层（过滤 / 新增 / 编辑 / 停用） -->
        <v-window-item value="items">
          <div class="d-flex flex-wrap align-center ga-2 mb-2">
            <v-select
              v-model="selectedRule"
              :items="[
                { title: '全部规则', value: '' },
                ...rules.map((r) => ({ title: `${r.rule_no} ${r.rule_name}`, value: r.rule_no })),
              ]"
              label="规则"
              density="compact"
              variant="outlined"
              hide-details
              style="max-width: 240px"
              @update:model-value="itemPage = 1; loadItems()"
            />
            <v-select
              v-model="filterMarket"
              :items="[{ title: '全部市场', value: '' }, ...MARKET_OPTIONS]"
              label="市场"
              density="compact"
              variant="outlined"
              hide-details
              style="max-width: 200px"
              @update:model-value="itemPage = 1; loadItems()"
            />
            <v-select
              v-model="filterStatus"
              :items="[
                { title: '全部状态', value: '' },
                { title: '生效[ACTIVE]', value: 'ACTIVE' },
                { title: '待生效[PENDING]', value: 'PENDING' },
                { title: '已停用[RETIRED]', value: 'RETIRED' },
              ]"
              label="状态"
              density="compact"
              variant="outlined"
              hide-details
              style="max-width: 170px"
              @update:model-value="itemPage = 1; loadItems()"
            />
            <v-text-field
              v-model="filterKeyword"
              label="代码 / 取值 / 说明"
              density="compact"
              variant="outlined"
              hide-details
              clearable
              prepend-inner-icon="mdi-magnify"
              style="max-width: 220px"
              @keyup.enter="itemPage = 1; loadItems()"
              @click:clear="itemPage = 1; loadItems()"
            />
            <v-btn size="small" variant="tonal" prepend-icon="mdi-magnify" @click="itemPage = 1; loadItems()">
              查询
            </v-btn>
            <v-spacer />
            <span class="text-caption text-medium-emphasis">
              「说明」列是长文本，鼠标悬浮 ⓘ 图标可看完整说明 / 出处 / 备注
            </span>
            <v-btn size="small" color="primary" prepend-icon="mdi-plus" @click="openCreate">
              新增条目
            </v-btn>
          </div>

          <v-table density="compact" class="usc-table">
            <thead>
              <tr>
                <th>规则</th>
                <th>市场</th>
                <th>品种</th>
                <th>代码</th>
                <th>正则</th>
                <th>动作</th>
                <th>取值</th>
                <th>状态</th>
                <th>说明</th>
                <th class="text-right">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in items" :key="item.idn">
                <td>{{ item.rule_no }}</td>
                <td>{{ marketText(item.market) }}</td>
                <td>{{ instrumentText(item.instrument) }}</td>
                <td>{{ item.code ?? '—' }}</td>
                <td>{{ item.code_regex ?? '—' }}</td>
                <td>{{ actionText(item.action) }}</td>
                <td>{{ item.usc_literal ?? item.rewrite_tpl ?? '—' }}</td>
                <td>
                  <v-chip :color="statusColor(item.status)" size="x-small" variant="tonal">
                    {{ statusText(item.status) }}
                  </v-chip>
                </td>
                <td>
                  <!-- 说明/出处是长文本：图标 + 悬浮提示（见 Docs/DevSpec/TableTextSpec.md） -->
                  <v-tooltip location="top" max-width="560" open-delay="120">
                    <template #activator="{ props }">
                      <v-icon
                        v-bind="props"
                        :color="item.item_desc ? 'primary' : 'grey'"
                        :icon="item.item_desc ? 'mdi-information-outline' : 'mdi-information-off-outline'"
                        size="small"
                      />
                    </template>
                    <div class="font-weight-medium mb-1">
                      {{ item.rule_no }} · {{ item.market ?? '*' }} / {{ item.instrument ?? '*' }} /
                      {{ item.code ?? item.code_regex ?? '*' }}
                    </div>
                    <div class="text-caption" style="white-space: pre-wrap">
                      {{ item.item_desc || '（未填写条目说明）' }}
                    </div>
                    <div
                      v-if="item.source_ref"
                      class="text-caption mt-2 text-medium-emphasis"
                      style="white-space: pre-wrap"
                    >
                      出处：{{ item.source_ref }}
                    </div>
                    <div
                      v-if="item.remark"
                      class="text-caption mt-1 text-medium-emphasis"
                      style="white-space: pre-wrap"
                    >
                      备注：{{ item.remark }}
                    </div>
                  </v-tooltip>
                </td>
                <td class="text-right text-no-wrap">
                  <v-btn size="x-small" variant="text" @click="openEdit(item)">编辑</v-btn>
                  <v-btn
                    size="x-small"
                    variant="text"
                    color="error"
                    :disabled="item.status === 'RETIRED'"
                    @click="openRetire(item)"
                  >
                    停用
                  </v-btn>
                </td>
              </tr>
              <tr v-if="items.length === 0">
                <td colspan="10" class="text-center text-medium-emphasis">没有符合条件的条目</td>
              </tr>
            </tbody>
          </v-table>

          <div class="d-flex align-center justify-end mt-2 ga-2">
            <span class="text-caption text-medium-emphasis">共 {{ itemTotal }} 条</span>
            <v-pagination
              v-model="itemPage"
              :length="Math.max(1, Math.ceil(itemTotal / itemPageSize))"
              density="compact"
              total-visible="7"
              @update:model-value="loadItems"
            />
          </div>
        </v-window-item>

        <!-- Tab 2：预演与应用 -->
        <v-window-item value="diff">
          <div class="d-flex flex-wrap align-center ga-2 mb-3">
            <v-btn color="primary" prepend-icon="mdi-diff" :loading="previewing" @click="doPreview">
              预演差异
            </v-btn>
            <v-btn
              color="success"
              prepend-icon="mdi-check-bold"
              :disabled="!hasDiff || starting"
              @click="openApply"
            >
              立即应用
            </v-btn>
            <v-btn
              color="teal"
              variant="tonal"
              prepend-icon="mdi-play-circle-outline"
              :loading="previewing || starting"
              @click="runNow"
            >
              立即执行一次
            </v-btn>
            <v-chip v-if="preview" variant="tonal" prepend-icon="mdi-counter">
              总 {{ preview.total }}
            </v-chip>
            <v-chip v-if="preview" color="success" variant="tonal">新增 {{ preview.insert }}</v-chip>
            <v-chip v-if="preview" color="warning" variant="tonal">更新 {{ preview.update }}</v-chip>
            <v-chip v-if="preview" color="error" variant="tonal">退市 {{ preview.retire }}</v-chip>
            <v-chip v-if="preview" color="info" variant="tonal">复用 {{ preview.reuse }}</v-chip>
            <v-chip v-if="preview && preview.key_update > 0" color="blue-darken-2" variant="tonal">
              键表回写 {{ preview.key_update }}
            </v-chip>
            <v-chip v-if="preview" variant="tonal">未变 {{ preview.keep }}</v-chip>
            <v-chip v-if="preview && preview.escalate > 0" color="deep-purple" variant="tonal">
              冲突升级 {{ preview.escalate }}
            </v-chip>
          </div>

          <v-alert v-if="!hasDiff" type="info" density="compact" variant="tonal" class="mb-3">
            预演结果为空（规则算出的映射与结果表一致，且键表 usc 与规则结果一致）。首次上线时这里会显示「新增 73212」，表示结果表尚未建立；
            若只是手工改过键表 usc，会显示「键表回写 N」，应用后即按规则回写。
          </v-alert>

          <v-alert
            v-if="preview && preview.conflicts.length > 0"
            type="error"
            density="compact"
            variant="tonal"
            class="mb-3"
          >
            存在无法自动消解的 usc 冲突，应用会被拒绝：{{ preview.conflicts.join('；') }}
          </v-alert>

          <div class="d-flex flex-wrap align-center ga-2 mb-2">
            <v-btn-toggle
              :model-value="diffMode"
              density="compact"
              variant="outlined"
              divided
              mandatory
              @update:model-value="(v) => switchDiffMode((v as 'preview' | 'map') ?? 'preview')"
            >
              <v-btn value="preview" size="small" prepend-icon="mdi-diff">本次预演明细</v-btn>
              <v-btn value="map" size="small" prepend-icon="mdi-database-search">结果表查询（随时）</v-btn>
            </v-btn-toggle>
            <v-text-field
              v-if="diffMode === 'preview'"
              v-model="diffKeyword"
              label="按 ftmm_symbol / code / usc 查明细"
              density="compact"
              variant="outlined"
              hide-details
              clearable
              prepend-inner-icon="mdi-magnify"
              style="max-width: 320px"
              @keyup.enter="diffPage = 1; loadDiffPage()"
              @click:clear="diffPage = 1; loadDiffPage()"
            />
            <v-text-field
              v-else
              v-model="mapKeyword"
              label="按 ftmm_symbol / code / usc 查结果表"
              density="compact"
              variant="outlined"
              hide-details
              clearable
              prepend-inner-icon="mdi-magnify"
              style="max-width: 320px"
              @keyup.enter="mapPage = 1; loadMapPage()"
              @click:clear="mapPage = 1; loadMapPage()"
            />
            <v-select
              v-if="diffMode === 'map'"
              v-model="mapStatus"
              :items="[
                { title: '全部状态', value: '' },
                { title: '生效[ACTIVE]', value: 'ACTIVE' },
                { title: '已停用[RETIRED]', value: 'RETIRED' },
              ]"
              density="compact"
              variant="outlined"
              hide-details
              style="max-width: 170px"
              @update:model-value="mapPage = 1; loadMapPage()"
            />
            <v-btn
              size="small"
              variant="tonal"
              prepend-icon="mdi-magnify"
              :loading="diffMode === 'preview' ? diffLoading : mapLoading"
              @click="diffMode === 'preview' ? (diffPage = 1, loadDiffPage()) : (mapPage = 1, loadMapPage())"
            >
              查询
            </v-btn>
          </div>

          <div v-if="preview && diffMode === 'preview'" class="d-flex flex-wrap align-center ga-2 mb-2">
            <v-select
              v-model="diffFilter"
              :items="[
                { title: '全部类别', value: '' },
                { title: '新增[insert]', value: 'insert' },
                { title: '更新[update]', value: 'update' },
                { title: '退市[retire]', value: 'retire' },
                { title: '键表回写[key_update]', value: 'key_update' },
              ]"
              density="compact"
              variant="outlined"
              hide-details
              style="max-width: 170px"
              @update:model-value="diffPage = 1; loadDiffPage()"
            />
            <v-chip variant="tonal" prepend-icon="mdi-format-list-bulleted">
              明细共 {{ diffTotal }} 条
            </v-chip>
            <v-chip variant="text" size="small">每页 {{ diffPageSize }} 条（完整明细缓存在服务端，重新预演会重置）</v-chip>
            <v-progress-circular v-if="diffLoading" size="18" indeterminate />
            <v-spacer />
            <v-btn size="small" variant="text" prepend-icon="mdi-refresh" @click="loadDiffPage">刷新明细</v-btn>
          </div>

          <v-alert v-if="diffHint" type="warning" density="compact" variant="tonal" class="mb-2">
            {{ diffHint }}
          </v-alert>

          <v-table v-if="preview && diffMode === 'preview'" density="compact" class="usc-table mb-3">
            <thead>
              <tr>
                <th>类别</th>
                <th>ftmm_symbol</th>
                <th>市场</th>
                <th>代码</th>
                <th>当前 usc</th>
                <th>新 usc</th>
                <th>依据</th>
                <th>复用来源</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in diffRows" :key="`${row.diff}-${row.ftmm_symbol}`">
                <td>
                  <v-chip :color="diffColor(row.diff)" size="x-small" variant="tonal">
                    {{ diffText(row.diff) }}
                  </v-chip>
                </td>
                <td>{{ row.ftmm_symbol }}</td>
                <td>{{ row.market }}</td>
                <td>{{ row.code }}</td>
                <td>{{ row.current_usc ?? '—' }}</td>
                <td>{{ row.new_usc ?? '—' }}</td>
                <td class="text-caption">{{ row.rule_no }} · {{ row.basis }}</td>
                <td class="text-caption">{{ row.reuse_of_ftmm ?? '—' }}</td>
              </tr>
              <tr v-if="diffRows.length === 0">
                <td colspan="8" class="text-center text-medium-emphasis">
                  {{ diffLoading ? '加载中…' : '没有符合条件的明细' }}
                </td>
              </tr>
            </tbody>
          </v-table>

          <div
            v-if="preview && diffMode === 'preview' && diffTotal > 0"
            class="d-flex align-center justify-end ga-2 mb-3"
          >
            <v-pagination
              v-model="diffPage"
              :length="Math.max(1, Math.ceil(diffTotal / diffPageSize))"
              :total-visible="7"
              density="compact"
              @update:model-value="loadDiffPage"
            />
          </div>

          <!-- 结果表查询：不依赖预演，任何时候都能按 ftmm_symbol / code / usc 定位一只证券 -->
          <template v-if="diffMode === 'map'">
            <v-alert type="info" density="compact" variant="tonal" class="mb-2 text-caption">
              `futu.finv_futu_usc_map` 在册映射（预演与应用写入的结果）。关键字同时匹配
              ftmm_symbol / code / usc，例如输入 <code>US.XRP</code>、<code>XRP</code> 或 <code>ETFXRP</code>。
            </v-alert>
            <v-table density="compact" class="usc-table mb-3">
              <thead>
                <tr>
                  <th>ftmm_symbol</th>
                  <th>市场</th>
                  <th>代码</th>
                  <th>品种</th>
                  <th>usc</th>
                  <th>命中规则</th>
                  <th>依据</th>
                  <th>状态</th>
                  <th>计算时间</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in mapRows" :key="row.ftmm_symbol">
                  <td>{{ row.ftmm_symbol }}</td>
                  <td>{{ marketText(row.market) }}</td>
                  <td>{{ row.code }}</td>
                  <td>{{ instrumentText(row.instrument) }}</td>
                  <td><strong>{{ row.usc }}</strong></td>
                  <td>{{ row.rule_no }}</td>
                  <td class="text-caption">{{ row.basis }}</td>
                  <td>
                    <v-chip :color="statusColor(row.status)" size="x-small" variant="tonal">
                      {{ statusText(row.status) }}
                    </v-chip>
                  </td>
                  <td class="text-caption">{{ row.dt_compute || '—' }}</td>
                </tr>
                <tr v-if="mapRows.length === 0">
                  <td colspan="9" class="text-center text-medium-emphasis">
                    {{ mapLoading ? '加载中…' : '没有符合条件的在册映射' }}
                  </td>
                </tr>
              </tbody>
            </v-table>
            <div class="d-flex align-center justify-end ga-2 mb-3">
              <span class="text-caption text-medium-emphasis">命中 {{ mapTotal }} 条</span>
              <v-pagination
                v-model="mapPage"
                :length="Math.max(1, Math.ceil(mapTotal / mapPageSize))"
                :total-visible="7"
                density="compact"
                @update:model-value="loadMapPage"
              />
              <v-btn size="small" variant="text" prepend-icon="mdi-refresh" @click="loadMapPage">刷新</v-btn>
            </div>
          </template>

          <v-card v-if="run" variant="tonal" class="mt-2">
            <v-card-title class="text-subtitle-1 d-flex align-center ga-2">
              运行 {{ run.run_id }}
              <v-chip
                :color="run.status === 'DONE' ? 'success' : run.status === 'FAILED' ? 'error' : 'info'"
                size="small"
                variant="tonal"
              >
                {{ run.status }}
              </v-chip>
              <v-chip size="small" variant="text">触发 {{ run.trigger }}</v-chip>
              <v-chip size="small" variant="text">{{ run.duration_ms }} ms</v-chip>
            </v-card-title>
            <v-card-text>
              <div class="d-flex flex-wrap ga-2 mb-2">
                <v-chip size="small" variant="tonal">新增 {{ run.insert }}</v-chip>
                <v-chip size="small" variant="tonal">更新 {{ run.update }}</v-chip>
                <v-chip size="small" variant="tonal">退市 {{ run.retire }}</v-chip>
                <v-chip size="small" variant="tonal">复用 {{ run.reuse }}</v-chip>
                <v-chip size="small" variant="tonal">归档 {{ run.archive }}</v-chip>
                <v-chip size="small" variant="tonal">回写键表 {{ run.key_update }}</v-chip>
              </div>
              <pre class="usc-log">{{ run.logs.map((l) => `[${l.time}] ${l.level} ${l.text}`).join('\n') }}</pre>
            </v-card-text>
          </v-card>
          <v-card variant="tonal" class="mt-3">
            <v-card-title class="text-subtitle-1 d-flex align-center flex-wrap ga-2">
              <v-icon icon="mdi-clock-outline" size="20" />
              定时自动执行
              <v-chip size="small" :color="schedule?.enable ? 'success' : 'grey'" variant="tonal">
                {{ schedule?.enable ? '已启用' : '已关闭' }}
              </v-chip>
              <v-chip v-if="schedule?.enable" size="small" variant="text">
                下次执行 {{ schedule?.next_run?.[0] ?? '—' }}
              </v-chip>
              <v-spacer />
              <v-btn variant="text" prepend-icon="mdi-refresh" @click="loadSchedule">刷新</v-btn>
            </v-card-title>
            <v-card-text>
              <p class="text-body-2 mb-2">
                这里只决定「上面那套动作多久自动跑一次」——定时执行的内容与「预演 → 应用」**完全相同**
                （算差异 → 归档复用 → 退市 → 写结果表 → 回写键表 → 记账），不是额外的一个步骤。
                想立刻跑一次，用上方「立即执行一次」。
              </p>
              <div class="d-flex flex-wrap align-center ga-2">
                <v-text-field
                  v-model="scheduleExpr"
                  label="cron（6 段：秒 分 时 日 月 周；留空=关闭自动执行）"
                  density="compact"
                  variant="outlined"
                  hide-details
                  style="max-width: 420px"
                  placeholder="0 30 3 * * *"
                />
                <v-btn color="primary" variant="tonal" :loading="scheduleSaving" @click="saveSchedule(scheduleExpr)">
                  保存
                </v-btn>
                <v-btn variant="text" :loading="scheduleSaving" @click="saveSchedule('')">关闭自动执行</v-btn>
              </div>
            </v-card-text>
          </v-card>
        </v-window-item>

        <!-- Tab 3：审计日志 -->
        <v-window-item value="logs">
          <v-table density="compact" class="usc-table">
            <thead>
              <tr>
                <th>时间</th>
                <th>对象</th>
                <th>动作</th>
                <th>主键</th>
                <th>所属规则</th>
                <th>操作人</th>
                <th>原因</th>
                <th class="text-center">变更摘要</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in logs"
                :key="row.idn"
                class="usc-log-row"
                style="cursor: pointer"
                @click="openLogDetail(row, 'compare')"
              >
                <td class="text-no-wrap">{{ row.gmt_create }}</td>
                <td>{{ row.src_table }}</td>
                <td>{{ row.action }}</td>
                <td class="text-no-wrap">{{ logPrimaryKey(row) }}</td>
                <td class="text-no-wrap">{{ logRuleNo(row) }}</td>
                <td>{{ row.operator || '—' }}</td>
                <td class="text-caption">{{ row.reason }}</td>
                <td class="text-center text-no-wrap">
                  <!-- 两种看法：摘要（字段级变更列表）/ 对比（变更前后并排）；点行其他列默认走对比 -->
                  <v-tooltip location="top" max-width="240" open-delay="120">
                    <template #activator="{ props }">
                      <v-btn
                        v-bind="props"
                        size="x-small"
                        variant="text"
                        icon="mdi-text-box-search-outline"
                        color="primary"
                        @click.stop="openLogDetail(row, 'summary')"
                      />
                    </template>
                    看摘要：只列发生变化的字段与前后取值
                  </v-tooltip>
                  <v-tooltip location="top" max-width="240" open-delay="120">
                    <template #activator="{ props }">
                      <v-btn
                        v-bind="props"
                        size="x-small"
                        variant="text"
                        icon="mdi-compare-horizontal"
                        color="primary"
                        @click.stop="openLogDetail(row, 'compare')"
                      />
                    </template>
                    看对比：变更前 / 变更后整行并排
                  </v-tooltip>
                </td>
              </tr>
              <tr v-if="logs.length === 0">
                <td colspan="8" class="text-center text-medium-emphasis">暂无审计记录</td>
              </tr>
            </tbody>
          </v-table>
          <div class="d-flex align-center justify-end mt-2 ga-2">
            <span class="text-caption text-medium-emphasis">共 {{ logTotal }} 条</span>
            <v-pagination
              v-model="logPage"
              :length="Math.max(1, Math.ceil(logTotal / 20))"
              density="compact"
              total-visible="5"
              @update:model-value="loadLogs"
            />
          </div>
        </v-window-item>
      </v-window>
    </v-card-text>

    <!-- 编辑规则（启用开关 / 默认动作 / 说明 / 备注） -->
    <v-dialog v-model="ruleEditOpen" max-width="680">
      <v-card v-if="ruleForm">
        <v-card-title>
          编辑规则 {{ ruleForm.rule_no }} · {{ ruleForm.rule_name }}
        </v-card-title>
        <v-card-text>
          <v-row dense>
            <v-col cols="12" md="4">
              <v-switch
                v-model="ruleForm.flag_enable"
                true-value="1"
                false-value="0"
                :color="ruleForm.flag_enable === '1' ? 'green' : 'grey'"
                density="compact"
                hide-details
                label="启用该层"
              />
            </v-col>
            <v-col cols="12" md="8">
              <v-select
                v-model="ruleForm.default_action"
                :items="ACTION_OPTIONS"
                item-title="title"
                item-value="value"
                label="默认动作（空=该层不指定动作）"
                density="compact"
                variant="outlined"
                clearable
                hide-details="auto"
              />
            </v-col>
            <v-col cols="12">
              <v-textarea
                v-model="ruleForm.rule_desc"
                label="规则说明"
                rows="3"
                auto-grow
                density="compact"
                variant="outlined"
                hide-details="auto"
              />
            </v-col>
            <v-col cols="12">
              <v-text-field
                v-model="ruleForm.remark"
                label="备注（变更原因、评审单号）"
                density="compact"
                variant="outlined"
                hide-details="auto"
              />
            </v-col>
            <v-col cols="12">
              <v-alert density="compact" variant="tonal" type="info" class="text-caption py-2">
                规则号、类型、同层顺序、作用域决定引擎的分层与先后顺序（P10 闸门 → 前处理 → 市场默认 → 例外 → 兜底），
                页面不允许改；确需调整请走数据库迁移。停用整层只影响「是否参与计算」，条目本身保持原状态。
              </v-alert>
            </v-col>
          </v-row>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="ruleEditOpen = false">取消</v-btn>
          <v-btn color="primary" :loading="ruleSaving === ruleForm.rule_no" @click="saveRule">保存</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- 编辑条目 -->
    <v-dialog v-model="editOpen" max-width="720">
      <v-card>
        <v-card-title>{{ editIsNew ? '新增条目' : `编辑条目 #${editForm.idn}` }}</v-card-title>
        <v-card-text>
          <v-row dense>
            <v-col cols="12" md="6">
              <v-select
                v-model="editForm.rule_no"
                :items="rules.map((r) => ({ title: `${r.rule_no} ${r.rule_name}`, value: r.rule_no }))"
                label="规则 *"
                density="compact"
                variant="outlined"
              />
            </v-col>
            <v-col cols="12" md="3">
              <v-combobox
                v-model="editForm.market"
                :items="MARKET_OPTIONS"
                item-title="title"
                item-value="value"
                :return-object="false"
                label="市场（可选可填）"
                density="compact"
                variant="outlined"
                clearable
                hide-details="auto"
              />
            </v-col>
            <v-col cols="12" md="3">
              <v-combobox
                v-model="editForm.instrument"
                :items="INSTRUMENT_OPTIONS"
                item-title="title"
                item-value="value"
                :return-object="false"
                label="品种（可选可填）"
                density="compact"
                variant="outlined"
                clearable
                hide-details="auto"
              />
            </v-col>
            <v-col cols="12" md="6">
              <v-select
                v-model="editForm.action"
                :items="ACTION_OPTIONS"
                label="动作（空=规则默认）"
                density="compact"
                variant="outlined"
                clearable
                hide-details="auto"
              />
            </v-col>
            <v-col v-if="!isRewriteAction" cols="12" md="6">
              <v-text-field
                v-model="editForm.code"
                label="代码（精确匹配，留空=该市场/品种下全部代码）"
                density="compact"
                variant="outlined"
                hide-details="auto"
              />
            </v-col>
            <v-col cols="12">
              <v-alert density="compact" variant="tonal" type="info" class="text-caption py-2">
                {{ actionHint }}
              </v-alert>
            </v-col>
            <!-- 只有「固定取值」动作才需要填字面量 -->
            <v-col v-if="isLiteralAction" cols="12" md="6">
              <v-text-field
                v-model="editForm.usc_literal"
                label="固定取值 usc *（要用它替换成的那串字面量）"
                placeholder="例如：US.XRP / ETFXRP / IDX.SPX"
                density="compact"
                variant="outlined"
                hide-details="auto"
              />
            </v-col>
            <!-- 只有「正则改写」动作才需要正则 + 替换模板 -->
            <v-col v-if="isRewriteAction" cols="12" md="6">
              <v-text-field
                v-model="editForm.code_regex"
                label="代码正则 *（匹配原代码）"
                placeholder="例如：^\. 或 ^(\d{6})$"
                density="compact"
                variant="outlined"
                hide-details="auto"
              />
            </v-col>
            <v-col v-if="isRewriteAction" cols="12" md="6">
              <v-text-field
                v-model="editForm.rewrite_tpl"
                label="替换模板 *（生成新 usc，捕获组用 $1）"
                placeholder="例如：SH$1"
                density="compact"
                variant="outlined"
                hide-details="auto"
              />
            </v-col>
            <v-col cols="12" md="4">
              <v-select
                v-model="editForm.status"
                :items="STATUS_OPTIONS"
                label="状态"
                density="compact"
                variant="outlined"
                hide-details="auto"
              />
            </v-col>
            <v-col cols="12" md="4">
              <v-text-field v-model="editForm.scope_env" label="环境范围" density="compact" variant="outlined" />
            </v-col>
            <v-col cols="12" md="4">
              <v-text-field v-model.number="editForm.priority" type="number" label="同层顺序" density="compact" variant="outlined" />
            </v-col>
            <v-col cols="12">
              <v-text-field
                v-model="editForm.item_desc"
                label="条目说明（默认自动生成，可直接改）"
                density="compact"
                variant="outlined"
                hide-details="auto"
                hint="按「市场/品种 + 匹配键 + 动作效果 + 依据规则」自动生成；手动改过就不再覆盖"
                @update:model-value="descTouched = true"
              >
                <template #append-inner>
                  <v-tooltip location="top" max-width="320" open-delay="120">
                    <template #activator="{ props }">
                      <v-btn v-bind="props" size="x-small" variant="text" icon="mdi-autorenew" @click="regenerateDesc" />
                    </template>
                    按当前选择重新生成说明（会丢掉手工修改）
                  </v-tooltip>
                </template>
              </v-text-field>
            </v-col>
            <v-col cols="12">
              <v-text-field v-model="editForm.source_ref" label="出处（如 USCMappingRule.md 附录 B）" density="compact" variant="outlined" />
            </v-col>
            <v-col cols="12">
              <v-text-field v-model="editForm.remark" label="备注" density="compact" variant="outlined" />
            </v-col>
          </v-row>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="editOpen = false">取消</v-btn>
          <v-btn color="primary" :loading="editSaving" @click="saveItem">保存</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- 停用条目 -->
    <v-dialog v-model="retireOpen" max-width="520">
      <v-card>
        <v-card-title>停用条目</v-card-title>
        <v-card-text>
          <div class="mb-3">
            将把
            <strong>{{ retireTarget?.rule_no }}</strong>
            {{ retireTarget?.market ?? '*' }} / {{ retireTarget?.code ?? retireTarget?.code_regex ?? '*' }}
            置为 RETIRED，并记录失效日期。条目行保留（不删除），可随时改回 ACTIVE。
          </div>
          <v-textarea v-model="retireReason" label="停用原因 *" rows="2" density="compact" variant="outlined" />
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="retireOpen = false">取消</v-btn>
          <v-btn color="error" @click="confirmRetire">确认停用</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- 应用 -->
    <v-dialog v-model="applyOpen" max-width="620">
      <v-card>
        <v-card-title>应用 USC 映射</v-card-title>
        <v-card-text>
          <v-alert type="warning" density="compact" variant="tonal" class="mb-3">
            将按预演结果写入 <code>futu.finv_futu_usc_map</code>、回写
            <code>futu.finv_futu_secu_key.usc</code>；usc 被复用时会先把下游历史行归档到
            <code>archive.finv_usc_reuse_snapshot</code>。
          </v-alert>
          <div class="mb-2">
            新增 {{ preview?.insert }} · 更新 {{ preview?.update }} · 退市 {{ preview?.retire }} · 复用
            {{ preview?.reuse }}
          </div>
          <v-textarea v-model="applyReason" label="应用原因（写入审计日志）" rows="2" density="compact" variant="outlined" />
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="applyOpen = false">取消</v-btn>
          <v-btn color="success" :loading="starting" @click="confirmApply">确认应用</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </v-card>

  <!-- 审计日志详情：before / after 格式化 JSON -->
  <v-dialog v-model="logDetailOpen" max-width="980" scrollable>
    <v-card v-if="logDetail">
      <v-card-title class="d-flex align-center ga-2">
        审计日志 #{{ logDetail.idn }}
        <v-chip size="small" variant="tonal">{{ logDetail.src_table }}</v-chip>
        <v-chip size="small" variant="tonal">{{ logDetail.action }}</v-chip>
        <v-spacer />
        <v-chip size="small" variant="text">{{ logDetail.gmt_create }}</v-chip>
      </v-card-title>
      <v-card-subtitle>
        对象 {{ logDetail.src_table }} · 主键 {{ logPrimaryKey(logDetail) }}
        <template v-if="logDetail.src_table !== 'rule'"> · 所属规则 {{ logRuleNo(logDetail) }}</template>
        · 操作人 {{ logDetail.operator || '—' }} · 原因 {{ logDetail.reason }}
      </v-card-subtitle>
      <v-tabs v-model="logView" density="compact" color="primary">
        <v-tab value="summary" prepend-icon="mdi-text-box-search-outline">摘要</v-tab>
        <v-tab value="compare" prepend-icon="mdi-compare-horizontal">对比</v-tab>
      </v-tabs>
      <v-card-text>
        <!-- 摘要：一句话总览 + 字段级变更列表（只留真正变化的字段） -->
        <div v-if="logView === 'summary'">
          <v-alert density="compact" variant="tonal" type="info" class="text-caption py-2 mb-3">
            {{ changeSummary(logDetail) }}
          </v-alert>
          <v-table density="compact" class="usc-table">
            <thead>
              <tr>
                <th>字段</th>
                <th>变更前</th>
                <th>变更后</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="diff in logDiff" :key="diff.field">
                <td class="text-no-wrap"><code>{{ diff.field }}</code></td>
                <td class="text-caption" style="white-space: pre-wrap; word-break: break-all">{{ diff.before }}</td>
                <td class="text-caption" style="white-space: pre-wrap; word-break: break-all">{{ diff.after }}</td>
              </tr>
              <tr v-if="logDiff.length === 0">
                <td colspan="3" class="text-center text-medium-emphasis">
                  没有字段级差异（新增/删除记录或快照为空）
                </td>
              </tr>
            </tbody>
          </v-table>
        </div>
        <!-- 对比：变更前 / 变更后整行并排 -->
        <v-row v-else dense>
          <v-col cols="12" md="6">
            <div class="d-flex align-center ga-2 mb-1">
              <strong>变更前（before_json）</strong>
              <v-spacer />
              <v-btn size="x-small" variant="text" prepend-icon="mdi-content-copy"
                     @click="copyText(prettyJSON(logDetail.before_json))">复制</v-btn>
            </div>
            <pre class="usc-json">{{ prettyJSON(logDetail.before_json) }}</pre>
          </v-col>
          <v-col cols="12" md="6">
            <div class="d-flex align-center ga-2 mb-1">
              <strong>变更后（after_json）</strong>
              <v-spacer />
              <v-btn size="x-small" variant="text" prepend-icon="mdi-content-copy"
                     @click="copyText(prettyJSON(logDetail.after_json))">复制</v-btn>
            </div>
            <pre class="usc-json">{{ prettyJSON(logDetail.after_json) }}</pre>
          </v-col>
        </v-row>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="logDetailOpen = false">关闭</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- 说明（页面最底部唯一说明卡片） -->
  <v-card variant="tonal" class="mt-3">
    <v-card-title class="text-subtitle-1">说明</v-card-title>
    <v-card-text class="text-body-2">
      <p class="usc-note">
        <strong>规则分层：</strong>P10 是闸门（前处理后大小写不敏感计数为 1 即定稿），P12/P14 是前处理（指数别名、
        期货合约后缀），P20–P22 是市场默认，P40/P50 是例外，P60 兜底。命中靠前的规则后不再看后面的规则。
      </p>
      <v-divider class="my-2" />
      <p class="usc-note">
        <strong>指数去前导点：</strong>指数代码带的前导点（如 <code>.SPX</code>、<code>.XFL</code>）由前处理规则
        P16 统一去掉；去点后若与本市场同码的股票/ETF 撞名，指数侧改用 <code>IDX.{code}</code>
        （如 <code>IDX.BFLY</code>），股票侧保持裸码。
      </p>
      <v-divider class="my-2" />
      <p class="usc-note">
        <strong>应用前必须先预演：</strong>预演返回 <code>preview_token</code>，应用时后端比对；预演之后规则或键表变了，
        令牌失效需要重新预演。冲突升级（候选与其它证券撞名时改用 <code>ftmm_symbol</code>）后仍冲突则整次拒绝。
      </p>
      <v-divider class="my-2" />
      <p class="usc-note">
        <strong>退市与复用：</strong>键表里已不存在的证券会被置为 <code>RETIRED</code>（保留历史行，<code>usc</code> 释放）；
        <code>usc</code> 允许被新证券复用，但复用时会先归档下游各表历史行，并在结果表写入 <code>reuse_of_ftmm</code> 留痕。
      </p>
      <v-divider class="my-2" />
      <p class="usc-note">
        <strong>定时同步：</strong>当前 <code>sys_config 20015</code> 为空 = 自动同步关闭（按规则首次应用会改写约 1.68 万行存量
        <code>usc</code>，需先确认存量对齐方案）。运维在系统配置里填回 cron（如 <code>0 30 3 * * *</code>）即可恢复每日同步。
      </p>
    </v-card-text>
  </v-card>
</template>

<style scoped>
.usc-table {
  font-variant-numeric: tabular-nums;
}

.usc-log {
  max-height: 260px;
  overflow: auto;
  font-size: 0.8125rem;
  line-height: 1.5;
  background: rgba(var(--v-theme-surface-variant), 0.4);
  padding: 8px 10px;
  border-radius: 4px;
}

.usc-log-row {
  cursor: pointer;
}

.usc-log-row:hover {
  background: rgba(var(--v-theme-primary), 0.06);
}

.usc-json {
  max-height: 420px;
  overflow: auto;
  font-size: 0.8125rem;
  line-height: 1.45;
  background: rgba(var(--v-theme-surface-variant), 0.4);
  padding: 10px 12px;
  border-radius: 4px;
  white-space: pre-wrap;
  word-break: break-all;
}

.usc-note {
  margin: 0;
  border-left: 3px solid rgba(var(--v-theme-primary), 0.6);
  padding-left: 10px;
}
</style>
