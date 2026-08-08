// FinvQuant 前端路由：每个业务菜单对应独立 URL 路径（多级菜单支持）
import { createRouter, createWebHistory } from 'vue-router'

import DashboardView from './views/DashboardView.vue'
import QuoteQueryView from './views/Meta/Finv/Quote/History/HistoryQuoteQueryView.vue'
import QuoteImportView from './views/Meta/Finv/Quote/History/HistoryQuoteImportView.vue'
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

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/dashboard' },
    { path: '/dashboard', name: 'dashboard', component: DashboardView, meta: { title: '仪表盘' } },
    // 历史行情 → 历史行情查询
    { path: '/Meta/Finv/Quote/History/HistoryQuoteQuery', name: 'quote-query', component: QuoteQueryView, meta: { title: '历史行情查询' } },
    // 元数据管理 → 业务元数据维护 → 四个子菜单
    { path: '/meta/exchange', name: 'meta-exchange', component: MetaExchangeView, meta: { title: '交易所信息维护' } },
    { path: '/meta/market', name: 'meta-market', component: MetaMarketView, meta: { title: '交易所下设市场信息维护' } },
    { path: '/meta/security', name: 'meta-security', component: MetaSecurityView, meta: { title: '规范证券信息维护' } },
    { path: '/meta/import', name: 'meta-import', component: QuoteImportView, meta: { title: '历史行情数据导入' } },
    // 配置管理 / 环境管理 / 模板管理（元数据管理之后、量化策略验证之前）
    { path: '/Meta/Finv/Quant/Config', name: 'config-manage', component: ConfigManageView, meta: { title: '配置管理' } },
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
  ],
})

router.afterEach((to) => {
  const title = (to.meta.title as string) ?? ''
  document.title = title ? `${title} · FinvQuant` : 'FinvQuant 量化策略交易平台'
})

export default router
