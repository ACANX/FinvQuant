<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()

// 各占位模块的规划说明
const plans: Record<string, { icon: string; desc: string; features: string[] }> = {
  'simulation-data': {
    icon: 'mdi-database-sync-outline',
    desc: '仿真数据验证：对历史行情数据与仿真数据源进行校验、比对与质量分析，验证数据准确性后供回测/模拟盘使用。',
    features: ['仿真数据源配置与接入', '行情数据质量校验（缺失/异常/跳变检测）', '真实数据 vs 仿真数据比对分析'],
  },
  'simulation-paper': {
    icon: 'mdi-account-cash-outline',
    desc: '模拟盘验证：使用虚拟资金按实盘规则执行策略，验证策略在接近实盘环境下的表现（含滑点、手续费、撮合排队）。',
    features: ['模拟盘账户与虚拟资金管理', '策略信号实时推送与自动撮合', '模拟盘收益与回撤跟踪'],
  },
  'simulation-live-sim': {
    icon: 'mdi-robot-outline',
    desc: '实盘仿真验证：使用实盘行情流驱动策略引擎，在仿真撮合环境中验证策略的实时执行链路（行情接入 → 信号 → 下单 → 回报）。',
    features: ['实盘行情流接入与重放', '仿真撮合与订单生命周期管理', '策略实时执行链路验证'],
  },
  'live-trading': {
    icon: 'mdi-cash-register',
    desc: '实盘交易：接入真实经纪商/交易所，管理实盘账户、下单与风控。该模块涉及真实资金，将在仿真验证全部通过后开放。',
    features: ['经纪商/交易所接入与账户绑定', '实盘下单/撤单与订单管理', '实盘风控（限额/熔断/人工确认）'],
  },
}

const plan = computed(() => plans[route.name as string] ?? {
  icon: 'mdi-flask-outline',
  desc: '该模块正在规划中。',
  features: ['需求梳理与设计', '技术方案评审', '开发与验证'],
})
</script>

<template>
  <v-container fluid>
    <v-card>
      <v-card-title class="d-flex align-center">
        <v-icon :icon="plan.icon" class="mr-2" color="primary" />
        {{ (route.meta.title as string) ?? '模块' }}
        <v-chip size="small" color="warning" class="ml-3">规划中</v-chip>
      </v-card-title>
      <v-card-text>
        <v-alert type="info" variant="tonal" class="mb-4">
          {{ plan.desc }}
        </v-alert>
        <v-list lines="two">
          <v-list-subheader>规划功能</v-list-subheader>
          <v-list-item v-for="(f, i) in plan.features" :key="i" :prepend-icon="'mdi-numeric-' + (i + 1) + '-circle-outline'">
            <v-list-item-title>{{ f }}</v-list-item-title>
          </v-list-item>
        </v-list>
        <v-divider class="my-4" />
        <p class="text-body-2 text-medium-emphasis">
          该模块与「量化策略验证 → 黄金期货合约回测验证」同属 FinvQuant 量化策略交易平台能力矩阵。
          当前已落地：策略管理（结构化定义）、账户管理（回测账户）、回测分析（任务与报告）、
          黄金期货合约回测验证（GCMain 等标的通用回测）。本模块将在后续迭代中实现。
        </p>
      </v-card-text>
    </v-card>
  </v-container>
</template>
