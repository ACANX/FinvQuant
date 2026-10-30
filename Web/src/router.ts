// FinvQuant 前端路由：每个业务菜单对应独立 URL 路径（多级菜单支持）
import { createRouter, createWebHistory } from 'vue-router'

import DashboardView from './views/DashboardView.vue'
import QuoteQueryView from './views/Meta/Finv/Quote/History/HistoryQuoteQueryView.vue'
import QuoteImportView from './views/Meta/Finv/Quote/History/HistoryQuoteImportView.vue'
import DeDupKLineQueryView from './views/Meta/Finv/Quote/DeDup/DeDupKLineQueryView.vue'
import DeDupKLineStatsView from './views/Meta/Finv/Quote/DeDup/DeDupKLineStatsView.vue'
import DeDupExportOverviewView from './views/Meta/Finv/Quote/DeDup/Export/DeDupExportOverviewView.vue'
import DeDupArchiveStockView from './views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveStockView.vue'
import DeDupArchiveChainView from './views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue'
import DeDupCDEUView from './views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue'
import DeDupCDEUWorkflowView from './views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUWorkflowView.vue'
import DeDupExportJobView from './views/Meta/Finv/Quote/DeDup/Export/DeDupExportJobView.vue'
import DeDupExportFileView from './views/Meta/Finv/Quote/DeDup/Export/DeDupExportFileView.vue'
import DeDupArchiveStatsView from './views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveStatsView.vue'
import DeDupArchiveConfigView from './views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveConfigView.vue'
import DeDupArchiveLogView from './views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveLogView.vue'
import MetaExchangeView from './views/Meta/Finv/MetaData/MetaExchangeView.vue'
import MetaMarketView from './views/Meta/Finv/MetaData/MetaMarketView.vue'
import MetaSecurityView from './views/Meta/Finv/MetaData/MetaSecurityView.vue'
// 量化回测（通用量化策略验证）
import BacktestGoldFuturesView from './views/Meta/Finv/Quant/Backtest/BacktestGoldFuturesView.vue'
import AccountManageView from './views/Meta/Finv/Quant/Account/AccountManageView.vue'
import FundManageView from './views/Meta/Finv/Quant/Fund/FundManageView.vue'
import PositionManageView from './views/Meta/Finv/Quant/Position/PositionManageView.vue'
import StrategyManageView from './views/Meta/Finv/Quant/Strategy/StrategyManageView.vue'
import BacktestAnalysisView from './views/Meta/Finv/Quant/Backtest/BacktestAnalysisView.vue'
import BacktestReportView from './views/Meta/Finv/Quant/Backtest/BacktestReportView.vue'
import RunDeleteTasksView from './views/Meta/Finv/Quant/Backtest/RunDeleteTasksView.vue'
import ConfigManageView from './views/Meta/Finv/Quant/Backtest/ConfigManageView.vue'
import EnvironmentView from './views/Meta/Finv/Quant/Backtest/EnvironmentView.vue'
import TemplateView from './views/Meta/Finv/Quant/Backtest/TemplateView.vue'
import PlaceholderView from './views/Common/PlaceholderView.vue'
import LoginView from './views/Common/LoginView.vue'

import { UNAUTHORIZED_EVENT, isLoggedIn } from './auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    // 登录页：公开路由，不渲染主框架（见 App.vue 的 meta.public 判断）
    { path: '/Login', name: 'login', component: LoginView, meta: { title: '登录', public: true } },
    { path: '/', redirect: '/Dashboard' },
    { path: '/Dashboard', name: 'dashboard', component: DashboardView, meta: { title: '仪表盘' } },
    // 一级系统落地路由（顶栏切换到「金融投资」时使用）
    { path: '/Meta/Finv', redirect: '/Dashboard' },
    // 历史行情 → 历史行情查询
    { path: '/Meta/Finv/Quote/History/HistoryQuoteQuery', name: 'quote-query', component: QuoteQueryView, meta: { title: '历史行情查询' } },
    // 行情数据归档 → K线数据查询 / 统计（需求 10856/10857 改名并移入「收剔导传ETL」）
    {
      path: '/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Query',
      name: 'dedup-kline-query',
      component: DeDupKLineQueryView,
      meta: { title: 'K线数据查询' },
    },
    {
      path: '/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Stats',
      name: 'dedup-kline-stats',
      component: DeDupKLineStatsView,
      meta: { title: 'K线数据统计' },
    },
    // 行情 → 收剔导传ETL（文件导出 / 文件上传等，需求 10862 改名）
    {
      path: '/Meta/Finv/Quote/DeDup/Export/Overview',
      name: 'dedup-export-overview',
      component: DeDupExportOverviewView,
      meta: { title: '月归档导出总览' },
    },
    {
      path: '/Meta/Finv/Quote/DeDup/Export/Chain',
      name: 'dedup-export-chain',
      component: DeDupArchiveChainView,
      meta: { title: '端到端链路' },
    },
    {
      path: '/Meta/Finv/Quote/Cdeu/HisMonthExistSecuStat',
      name: 'cdeu-his-month-exist-secu-stat',
      component: DeDupArchiveStockView,
      meta: { title: '历史月份存量数据清点' },
    },
    {
      path: '/Meta/Finv/Quote/Cdeu/CdeuOverview',
      name: 'dedup-export-cdeu',
      component: DeDupCDEUView,
      meta: { title: 'CDEU 工作流控制面板' },
    },
    {
      path: '/Meta/Finv/Quote/Cdeu/CdeuWorkflow',
      name: 'dedup-export-cdeu-workflow',
      component: DeDupCDEUWorkflowView,
      meta: { title: '全流程拓扑' },
    },
    {
      path: '/Meta/Finv/Quote/DeDup/Export/Job',
      name: 'dedup-export-job',
      component: DeDupExportJobView,
      meta: { title: '文件导出' },
    },
    {
      path: '/Meta/Finv/Quote/DeDup/Export/File',
      name: 'dedup-export-file',
      component: DeDupExportFileView,
      meta: { title: '文件上传' },
    },
    {
      path: '/Meta/Finv/Quote/DeDup/Export/Stats',
      name: 'dedup-export-stats',
      component: DeDupArchiveStatsView,
      meta: { title: '归档统计与预算' },
    },
    {
      path: '/Meta/Finv/Quote/DeDup/Export/Config',
      name: 'dedup-export-config',
      component: DeDupArchiveConfigView,
      meta: { title: '归档主数据与规则' },
    },
    {
      path: '/Meta/Finv/Quote/DeDup/Export/Log',
      name: 'dedup-export-log',
      component: DeDupArchiveLogView,
      meta: { title: '归档日志与补丁' },
    },
    // 元数据 → Finv业务元数据 → 子菜单
    { path: '/meta/exchange', name: 'meta-exchange', component: MetaExchangeView, meta: { title: '交易所信息管理' } },
    { path: '/meta/market', name: 'meta-market', component: MetaMarketView, meta: { title: '市场信息管理' } },
    { path: '/meta/security', name: 'meta-security', component: MetaSecurityView, meta: { title: '规范证券信息管理' } },
    { path: '/meta/import', name: 'meta-import', component: QuoteImportView, meta: { title: '历史行情数据导入' } },
    // 系统配置（需求 10859 改名；环境/模板管理等已迁入「量化 → MVP配置」）
    { path: '/Meta/Finv/Quant/Config', name: 'config-manage', component: ConfigManageView, meta: { title: '系统配置' } },
    { path: '/Meta/Finv/Quant/Environment', name: 'environment', component: EnvironmentView, meta: { title: '环境管理' } },
    { path: '/Meta/Finv/Quant/Template', name: 'template', component: TemplateView, meta: { title: '模板管理' } },
    // 账户管理 / 资金管理 / 持仓管理（配置/环境/模板之后、量化策略验证之前）
    { path: '/Meta/Finv/Quant/Account', name: 'account', component: AccountManageView, meta: { title: '账户管理' } },
    { path: '/Meta/Finv/Quant/Fund', name: 'fund', component: FundManageView, meta: { title: '资金管理' } },
    { path: '/Meta/Finv/Quant/Position', name: 'position', component: PositionManageView, meta: { title: '持仓管理' } },
    // 量化策略验证 → 黄金期货合约回测验证
    {
      path: '/Meta/Finv/Quant/Backtest/GoldFutures',
      name: 'backtest-gold-futures',
      component: BacktestGoldFuturesView,
      meta: { title: '黄金期货合约回测验证' },
    },
    // 策略管理 / 回测分析
    { path: '/Meta/Finv/Quant/Strategy', name: 'strategy', component: StrategyManageView, meta: { title: '策略管理' } },
    { path: '/Meta/Finv/Quant/Backtest/Analysis', name: 'backtest-analysis', component: BacktestAnalysisView, meta: { title: '回测分析' } },
    // 回测任务删除管理（异步删除任务 + 审计留痕）
    {
      path: '/Meta/Finv/Quant/Backtest/RunDeleteTasks',
      name: 'run-delete-tasks',
      component: RunDeleteTasksView,
      meta: { title: '回测任务删除管理' },
    },
    // 投资策略回测收益分析报告（独立页，任务列表跳转进入，?runId= 深链）
    {
      path: '/Meta/Finv/Quant/Backtest/Analysis/Report',
      name: 'backtest-report',
      component: BacktestReportView,
      meta: { title: '投资策略回测收益分析报告' },
    },
    // 仿真数据验证 / 模拟盘验证 / 实盘仿真验证 / 实盘交易（规划占位）
    { path: '/Meta/Finv/Quant/Simulation/Data', name: 'simulation-data', component: PlaceholderView, meta: { title: '仿真数据验证' } },
    { path: '/Meta/Finv/Quant/Simulation/Paper', name: 'simulation-paper', component: PlaceholderView, meta: { title: '模拟盘验证' } },
    { path: '/Meta/Finv/Quant/Simulation/LiveSim', name: 'simulation-live-sim', component: PlaceholderView, meta: { title: '实盘仿真验证' } },
    { path: '/Meta/Finv/Quant/LiveTrading', name: 'live-trading', component: PlaceholderView, meta: { title: '实盘交易' } },
    // 内容管理（CMS）系统：本轮仅占位，业务菜单后续整合
    { path: '/Meta/CMS', name: 'cms-home', component: PlaceholderView, meta: { title: '内容管理控制台' } },
  ],
})

// 旧的小写路径归一化。
//
// 为什么不用路由级 redirect：vue-router 4 的路径匹配默认大小写不敏感
// （sensitive: false），`/login` 会直接命中 `/Login` 路由，redirect 记录
// 永远匹配不到；此时 to.path 仍是访问时的原始大小写，因此在守卫里归一化
// 是唯一可靠的位置。
const LEGACY_PATH_MAP: Record<string, string> = {
  '/login': '/Login',
  '/dashboard': '/Dashboard',
}

// 登录守卫：未登录一律重定向到登录页，并记录来源路径供登录后回跳
router.beforeEach((to) => {
  const normalized = LEGACY_PATH_MAP[to.path]
  if (normalized) {
    return { path: normalized, query: to.query, hash: to.hash, replace: true }
  }
  if (to.meta.public) {
    // 已登录时访问登录页直接回首页，避免重复登录
    if (to.name === 'login' && isLoggedIn()) {
      return { path: '/Dashboard' }
    }
    return true
  }
  if (!isLoggedIn()) {
    return { path: '/Login', query: { redirect: to.fullPath } }
  }
  return true
})

// 接口返回 401 时（令牌过期或被吊销）由 api 层广播，这里统一切回登录页
window.addEventListener(UNAUTHORIZED_EVENT, () => {
  const current = router.currentRoute.value
  if (current.meta.public) return
  void router.replace({ path: '/Login', query: { redirect: current.fullPath } })
})

router.afterEach((to) => {
  const title = (to.meta.title as string) ?? ''
  document.title = title ? `${title} · FinvQuant` : 'FinvQuant策略回测量化交易'
})

export default router
