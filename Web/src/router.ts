// FinvQuant 前端路由：每个业务菜单对应独立 URL 路径（多级菜单支持）
import { createRouter, createWebHistory } from 'vue-router'

import DashboardView from './views/DashboardView.vue'
import QuoteQueryView from './views/QuoteQueryView.vue'
import QuoteImportView from './views/QuoteImportView.vue'
import MetaExchangeView from './views/MetaExchangeView.vue'
import MetaMarketView from './views/MetaMarketView.vue'
import MetaSecurityView from './views/MetaSecurityView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/dashboard' },
    { path: '/dashboard', name: 'dashboard', component: DashboardView, meta: { title: '仪表盘' } },
    // 历史行情 → 历史行情查询
    { path: '/quote/query', name: 'quote-query', component: QuoteQueryView, meta: { title: '历史行情查询' } },
    // 元数据管理 → 业务元数据维护 → 四个子菜单
    { path: '/meta/exchange', name: 'meta-exchange', component: MetaExchangeView, meta: { title: '交易所信息维护' } },
    { path: '/meta/market', name: 'meta-market', component: MetaMarketView, meta: { title: '交易所下设市场信息维护' } },
    { path: '/meta/security', name: 'meta-security', component: MetaSecurityView, meta: { title: '规范证券信息维护' } },
    { path: '/meta/import', name: 'meta-import', component: QuoteImportView, meta: { title: '历史行情数据导入' } },
  ],
})

router.afterEach((to) => {
  const title = (to.meta.title as string) ?? ''
  document.title = title ? `${title} · FinvQuant` : 'FinvQuant 量化策略交易平台'
})

export default router
