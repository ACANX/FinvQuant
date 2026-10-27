<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useDisplay } from 'vuetify'
import { useRoute, useRouter } from 'vue-router'

import { apiPost } from './api'
import { authState, clearSession } from './auth'

// 多级菜单定义（支持两级分组：顶级 → 二级 → 三级菜单项）
// 每个叶子菜单项对应独立 URL 路由路径（见 Web/src/router.ts）
interface MenuLeaf {
  key: string
  title: string
  icon: string
  path: string
}

interface MenuGroup {
  key: string
  title: string
  icon: string
  children: (MenuLeaf | MenuGroup)[]
}

const menuItems: (MenuLeaf | MenuGroup)[] = [
  { key: 'dashboard', title: '仪表盘', icon: 'mdi-view-dashboard', path: '/dashboard' },
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
      // 月归档导出（二级分组，含 3 个三级叶子；渲染模板仅支持「一级分组 → 二级分组 → 叶子」两级嵌套）
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
  // 配置管理 / 环境管理 / 模板管理（元数据管理之后、量化策略验证之前）
  { key: 'config-manage', title: '配置管理', icon: 'mdi-cog-outline', path: '/Meta/Finv/Quant/Config' },
  { key: 'environment', title: '环境管理', icon: 'mdi-application-cog-outline', path: '/Meta/Finv/Quant/Environment' },
  { key: 'template', title: '模板管理', icon: 'mdi-content-copy', path: '/Meta/Finv/Quant/Template' },
  // 账户管理 / 资金管理 / 持仓管理（模板管理之后、量化策略验证之前）
  { key: 'account', title: '账户管理', icon: 'mdi-account-cog-outline', path: '/Meta/Finv/Quant/Account' },
  { key: 'fund', title: '资金管理', icon: 'mdi-cash-multiple', path: '/Meta/Finv/Quant/Fund' },
  { key: 'position', title: '持仓管理', icon: 'mdi-briefcase-variant-outline', path: '/Meta/Finv/Quant/Position' },
  // 量化策略验证 → 黄金期货合约回测验证
  {
    key: 'quant-backtest',
    title: '量化策略验证',
    icon: 'mdi-flask-outline',
    children: [
      { key: 'backtest-gold-futures', title: '黄金期货合约回测验证', icon: 'mdi-chart-bell-curve', path: '/Meta/Finv/Quant/Backtest/GoldFutures' },
    ],
  },
  // 策略管理 / 回测分析（通用量化回测）
  { key: 'strategy', title: '策略管理', icon: 'mdi-sitemap-outline', path: '/Meta/Finv/Quant/Strategy' },
  { key: 'backtest-analysis', title: '回测分析', icon: 'mdi-chart-timeline-variant', path: '/Meta/Finv/Quant/Backtest/Analysis' },
  // 仿真 / 模拟盘 / 实盘（规划中）
  { key: 'simulation-data', title: '仿真数据验证', icon: 'mdi-database-sync-outline', path: '/Meta/Finv/Quant/Simulation/Data' },
  { key: 'simulation-paper', title: '模拟盘验证', icon: 'mdi-account-cash-outline', path: '/Meta/Finv/Quant/Simulation/Paper' },
  { key: 'simulation-live-sim', title: '实盘仿真验证', icon: 'mdi-robot-outline', path: '/Meta/Finv/Quant/Simulation/LiveSim' },
  { key: 'live-trading', title: '实盘交易', icon: 'mdi-cash-register', path: '/Meta/Finv/Quant/LiveTrading' },
]

const route = useRoute()
const router = useRouter()
const display = useDisplay()
const drawer = ref(true)

// 公开页（登录）不渲染导航框架，避免未登录时先闪一下侧边栏
const isPublicRoute = computed(() => Boolean(route.meta.public))

/** 退出登录：先尽力吊销服务端会话，再清本地状态并回登录页 */
async function onLogout(): Promise<void> {
  try {
    await apiPost('/Auth/Logout', {})
  } catch {
    // 服务端会话可能已过期；本地状态必须清理，因此忽略该错误
  }
  clearSession()
  await router.replace('/login')
}

// 当前激活菜单项 key（叶子路径匹配）
function isLeaf(item: MenuLeaf | MenuGroup): item is MenuLeaf {
  return 'path' in item
}

function isActive(item: MenuLeaf | MenuGroup): boolean {
  if (isLeaf(item)) return route.path === item.path
  return item.children.some((child) => isActive(child))
}

// 一级菜单展开状态：当前路由所在的分组保持展开
function isGroupOpen(item: MenuGroup): boolean {
  return item.children.some((child) => (isLeaf(child) ? isActive(child) : isGroupOpen(child)))
}

// 路由切换后收起抽屉（移动端/小屏临时抽屉），桌面端保持折叠状态用户可自行展开
watch(
  () => route.fullPath,
  () => {
    if (display.mobile.value) drawer.value = false
  },
)
</script>

<template>
  <v-app>
    <!-- 公开页（登录）：全屏渲染，不显示导航框架 -->
    <router-view v-if="isPublicRoute" />

    <template v-else>
    <v-app-bar color="primary" density="comfortable">
      <v-app-bar-nav-icon @click="drawer = !drawer" />
      <v-app-bar-title>
        <v-icon icon="mdi-finance" class="mr-2" />
        FinvQuant 量化策略交易平台
      </v-app-bar-title>
      <!-- 当前登录用户与退出入口 -->
      <v-menu location="bottom end">
        <template #activator="{ props }">
          <v-btn
            v-bind="props"
            variant="text"
            prepend-icon="mdi-account-circle-outline"
            :text="authState.user?.nick_name || authState.user?.uid || '未登录'"
          />
        </template>
        <v-list density="compact" min-width="180">
          <v-list-item
            :title="authState.user?.nick_name || '-'"
            :subtitle="authState.user?.uid || ''"
            prepend-icon="mdi-account"
          />
          <v-divider />
          <v-list-item prepend-icon="mdi-logout" title="退出登录" @click="onLogout" />
        </v-list>
      </v-menu>
    </v-app-bar>

    <v-navigation-drawer
      v-model="drawer"
      :temporary="display.mobile.value"
      :permanent="!display.mobile.value"
      :width="260"
    >
      <v-list density="comfortable" nav>
        <template v-for="item in menuItems" :key="item.key">
          <!-- 叶子菜单 -->
          <v-list-item
            v-if="isLeaf(item)"
            :prepend-icon="item.icon"
            :title="item.title"
            :active="isActive(item)"
            :to="item.path"
          />

          <!-- 分组菜单（可再含二级分组） -->
          <v-list-group v-else :value="item.key" :open="isGroupOpen(item)">
            <template #activator="{ props }">
              <v-list-item
                v-bind="props"
                :prepend-icon="item.icon"
                :title="item.title"
                :active="isActive(item)"
              />
            </template>

            <template v-for="child in item.children" :key="child.key">
              <!-- 二级叶子 -->
              <v-list-item
                v-if="isLeaf(child)"
                :prepend-icon="child.icon"
                :title="child.title"
                :active="isActive(child)"
                :to="child.path"
              />

              <!-- 二级分组（三级叶子） -->
              <v-list-group v-else :value="child.key" subgroup :open="isGroupOpen(child)">
                <template #activator="{ props }">
                  <v-list-item
                    v-bind="props"
                    :prepend-icon="child.icon"
                    :title="child.title"
                    :active="isActive(child)"
                  />
                </template>

                <v-list-item
                  v-for="leaf in child.children.filter(isLeaf)"
                  :key="leaf.key"
                  :prepend-icon="leaf.icon"
                  :title="leaf.title"
                  :active="isActive(leaf)"
                  :to="leaf.path"
                />
              </v-list-group>
            </template>
          </v-list-group>
        </template>
      </v-list>
    </v-navigation-drawer>

    <v-main>
      <v-container fluid class="mt-4">
        <router-view />
      </v-container>
    </v-main>
    </template>
  </v-app>
</template>

<style>
/* 侧边导航缩进：每级增量 14px（上一版 28px 的 50%），进一步释放三级菜单标题空间 */
.v-navigation-drawer .v-list-group {
  --list-indent-size: 4px;
  --v-list-group-prepend: 10px;
}

/* 菜单图标与标题的水平间距：32px → 约 1/3（10.67px） */
.v-navigation-drawer .v-list-item__prepend {
  --v-list-prepend-gap: 10.67px;
}
</style>
