<script setup lang="ts">
import { ref, watch } from 'vue'
import { useDisplay } from 'vuetify'
import { useRoute } from 'vue-router'

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
const display = useDisplay()
const drawer = ref(true)

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
    <v-app-bar color="primary" density="comfortable">
      <v-app-bar-nav-icon @click="drawer = !drawer" />
      <v-app-bar-title>
        <v-icon icon="mdi-finance" class="mr-2" />
        FinvQuant 量化策略交易平台
      </v-app-bar-title>
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
                class="pl-6"
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
                  class="pl-12"
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
  </v-app>
</template>
