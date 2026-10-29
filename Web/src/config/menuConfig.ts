// 前端菜单注册表：一级 = 系统（顶栏切换），其下二/三/四级菜单。
//
// 设计依据：Docs/Refactor/2610-012-WebMenuFourLevelRefactor.md
// 参考实现：D:\Code\JavaCode\Deamon\Web\WebCMS\src\config\menuConfig.js
//
// 约定：
// 1. 内部 key 稳定不变（finvquant / cms），显示名与 CMS 一级菜单一致（金融投资 / 内容管理）；
// 2. 分组节点不填 path（点击只展开/折叠），叶子节点必须有 path；
// 3. 叶子 key 与改造前 App.vue 中 menuItems 的 key 保持一致，避免文档索引二次改动；
// 4. 新增菜单只改本文件，无需改动 App.vue 与菜单组件。

/** 菜单节点（分组或叶子） */
export interface MenuNode {
  /** 同级唯一；展开状态用「父链 key」拼接后使用 */
  key: string
  /** 简体中文标题 */
  title: string
  /** 可选副标题（预留字段，本轮不填内容） */
  subtitle?: string
  /** mdi 图标名；各层级均保留图标 */
  icon?: string
  /** 叶子路由；分组节点不填 */
  path?: string
  /** 子节点：有 children 即为分组节点 */
  children?: MenuNode[]
}

/** 一级系统（顶栏切换器的一项） */
export interface MenuSystem {
  /** 稳定内部 key，用于展开状态持久化与系统识别 */
  key: string
  /** 顶栏与抽屉标题显示名（与 CMS 一级菜单同名） */
  title: string
  /** 可选副标题（顶栏下拉与抽屉标题展示） */
  subtitle?: string
  icon: string
  /** 系统落地路由（首次进入该系统时的目标地址） */
  path: string
  /** 系统归属前缀：命中任一前缀即属于该系统（按最长前缀匹配） */
  match: string[]
  children: MenuNode[]
}

/** 金融投资（本项目已实现的全部业务，对应 CMS 一级菜单「金融投资」） */
const finvQuantMenu: MenuNode[] = [
  { key: 'dashboard', title: '仪表盘', icon: 'mdi-view-dashboard', path: '/Dashboard' },
  {
    key: 'data-collection',
    title: '数据采集',
    icon: 'mdi-database-arrow-down',
    children: [
      {
        key: 'quote-dedup',
        title: '行情数据去重',
        icon: 'mdi-content-duplicate',
        children: [
          { key: 'dedup-kline-query', title: '去重分钟行情查询', icon: 'mdi-table-search', path: '/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Query' },
          { key: 'dedup-kline-stats', title: '去重分钟行情统计', icon: 'mdi-chart-box-outline', path: '/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Stats' },
        ],
      },
      {
        key: 'quote-archive',
        title: '行情数据归档',
        icon: 'mdi-database-export-outline',
        children: [
          { key: 'dedup-export-overview', title: '归档监控总览', icon: 'mdi-monitor-dashboard', path: '/Meta/Finv/Quote/DeDup/Export/Overview' },
          { key: 'dedup-export-chain', title: '端到端链路', icon: 'mdi-transit-connection-variant', path: '/Meta/Finv/Quote/DeDup/Export/Chain' },
          { key: 'dedup-export-cdeu', title: '工作流控制与监控', icon: 'mdi-sitemap-outline', path: '/Meta/Finv/Quote/DeDup/CDEU' },
          { key: 'dedup-export-job', title: '导出任务', icon: 'mdi-clipboard-list-outline', path: '/Meta/Finv/Quote/DeDup/Export/Job' },
          { key: 'dedup-export-file', title: '文件与上传', icon: 'mdi-file-cloud-outline', path: '/Meta/Finv/Quote/DeDup/Export/File' },
          { key: 'dedup-export-stats', title: '归档统计与预算', icon: 'mdi-chart-box-multiple-outline', path: '/Meta/Finv/Quote/DeDup/Export/Stats' },
          { key: 'dedup-export-config', title: '归档主数据与规则', icon: 'mdi-tune-variant', path: '/Meta/Finv/Quote/DeDup/Export/Config' },
          { key: 'dedup-export-log', title: '归档日志与补丁', icon: 'mdi-text-box-search-outline', path: '/Meta/Finv/Quote/DeDup/Export/Log' },
        ],
      },
    ],
  },
  {
    key: 'history-quote',
    title: '历史行情',
    icon: 'mdi-chart-box',
    children: [
      { key: 'quote-query', title: '历史行情查询', icon: 'mdi-chart-line', path: '/Meta/Finv/Quote/History/HistoryQuoteQuery' },
    ],
  },
  {
    key: 'metadata',
    title: '元数据管理',
    icon: 'mdi-database-cog',
    children: [
      {
        key: 'meta-maintenance',
        title: '业务元数据维护',
        icon: 'mdi-database-search',
        children: [
          { key: 'meta-exchange', title: '交易所信息维护', icon: 'mdi-office-building', path: '/meta/exchange' },
          { key: 'meta-market', title: '交易所下设市场信息维护', icon: 'mdi-chart-areaspline', path: '/meta/market' },
          { key: 'meta-security', title: '规范证券信息维护', icon: 'mdi-tag-multiple', path: '/meta/security' },
          { key: 'meta-import', title: '历史行情数据导入', icon: 'mdi-database-import', path: '/meta/import' },
        ],
      },
    ],
  },
  { key: 'config-manage', title: '配置管理', icon: 'mdi-cog-outline', path: '/Meta/Finv/Quant/Config' },
  { key: 'environment', title: '环境管理', icon: 'mdi-application-cog-outline', path: '/Meta/Finv/Quant/Environment' },
  { key: 'template', title: '模板管理', icon: 'mdi-content-copy', path: '/Meta/Finv/Quant/Template' },
  { key: 'account', title: '账户管理', icon: 'mdi-account-cog-outline', path: '/Meta/Finv/Quant/Account' },
  { key: 'fund', title: '资金管理', icon: 'mdi-cash-multiple', path: '/Meta/Finv/Quant/Fund' },
  { key: 'position', title: '持仓管理', icon: 'mdi-briefcase-variant-outline', path: '/Meta/Finv/Quant/Position' },
  {
    key: 'quant-backtest',
    title: '量化策略验证',
    icon: 'mdi-flask-outline',
    children: [
      { key: 'backtest-gold-futures', title: '黄金期货合约回测验证', icon: 'mdi-chart-bell-curve', path: '/Meta/Finv/Quant/Backtest/GoldFutures' },
    ],
  },
  { key: 'strategy', title: '策略管理', icon: 'mdi-sitemap-outline', path: '/Meta/Finv/Quant/Strategy' },
  { key: 'backtest-analysis', title: '回测分析', icon: 'mdi-chart-timeline-variant', path: '/Meta/Finv/Quant/Backtest/Analysis' },
  { key: 'simulation-data', title: '仿真数据验证', icon: 'mdi-database-sync-outline', path: '/Meta/Finv/Quant/Simulation/Data' },
  { key: 'simulation-paper', title: '模拟盘验证', icon: 'mdi-account-cash-outline', path: '/Meta/Finv/Quant/Simulation/Paper' },
  { key: 'simulation-live-sim', title: '实盘仿真验证', icon: 'mdi-robot-outline', path: '/Meta/Finv/Quant/Simulation/LiveSim' },
  { key: 'live-trading', title: '实盘交易', icon: 'mdi-cash-register', path: '/Meta/Finv/Quant/LiveTrading' },
]

/** 内容管理（CMS，本轮仅占位，业务菜单后续整合） */
const cmsMenu: MenuNode[] = [
  { key: 'cms-home', title: '内容管理控制台', icon: 'mdi-view-dashboard-outline', path: '/Meta/CMS' },
]

/** 一级系统注册表：新增系统只在此数组追加一项 */
export const menuSystems: MenuSystem[] = [
  {
    key: 'finvquant',
    title: '金融投资',
    subtitle: '量化策略交易平台',
    icon: 'mdi-finance',
    path: '/Meta/Finv',
    // /Dashboard 与存量小写 /meta/* 路由同样归属金融投资
    match: ['/Meta/Finv', '/Dashboard', '/meta/'],
    children: finvQuantMenu,
  },
  {
    key: 'cms',
    title: '内容管理',
    subtitle: 'CMS 内容管理系统',
    icon: 'mdi-laptop',
    path: '/Meta/CMS',
    match: ['/Meta/CMS'],
    children: cmsMenu,
  },
]

/** 叶子判定：有 path 且无子节点 */
export function isMenuLeaf(node: MenuNode): boolean {
  return Boolean(node.path) && !(node.children && node.children.length > 0)
}

/**
 * 按路由反查所属系统（最长前缀匹配）。
 * 未命中任何前缀时回落到首个系统（金融投资），并输出告警便于排查配置遗漏。
 */
export function resolveSystem(path: string): MenuSystem {
  let matched: MenuSystem | null = null
  let matchedLength = -1
  for (const system of menuSystems) {
    for (const prefix of system.match) {
      const normalized = prefix.endsWith('/') ? prefix : `${prefix}/`
      if (path === prefix || path.startsWith(normalized)) {
        if (prefix.length > matchedLength) {
          matched = system
          matchedLength = prefix.length
        }
      }
    }
  }
  if (matched) return matched
  console.warn(`[menu] 路由 ${path} 未匹配任何系统前缀，已回落到 ${menuSystems[0].key}`)
  return menuSystems[0]
}

/** 返回从系统根到目标叶子的节点链（系统节点在最前）；未命中返回空数组 */
export function findMenuTrail(path: string): MenuNode[] {
  const system = resolveSystem(path)
  const trail: MenuNode[] = []

  function walk(nodes: MenuNode[]): boolean {
    for (const node of nodes) {
      trail.push(node)
      if (isMenuLeaf(node)) {
        if (node.path === path) return true
      } else if (node.children && walk(node.children)) {
        return true
      }
      trail.pop()
    }
    return false
  }

  if (!walk(system.children)) return []
  return [system, ...trail]
}

/** 返回某节点下第一个叶子的路由（用于面包屑分组点击） */
export function findFirstLeaf(node: MenuNode): string | undefined {
  if (isMenuLeaf(node)) return node.path
  for (const child of node.children ?? []) {
    const found = findFirstLeaf(child)
    if (found) return found
  }
  return undefined
}

/**
 * 由节点链计算需要展开的分组 key（父链拼接，不含叶子）。
 * 例：['finvquant', 'data-collection', 'quote-archive'] →
 *   ['finvquant', 'finvquant.data-collection', 'finvquant.data-collection.quote-archive']
 */
export function trailToOpenKeys(trail: MenuNode[]): string[] {
  const keys: string[] = []
  let parent = ''
  for (const node of trail) {
    const current = parent ? `${parent}.${node.key}` : node.key
    if (!isMenuLeaf(node)) keys.push(current)
    parent = current
  }
  return keys
}
