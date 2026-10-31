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
  'cdeu-automation': {
    icon: 'mdi-robot-happy-outline',
    desc: 'CDEU自动化：把「采集 → 去重 → 导出决策 → 导出 → 上传 → 复核」这条链路的调度与运维动作自动化，减少人工触发与值守。',
    features: ['工作流运行模式（Manual/Auto）与低峰窗口的集中配置入口', '按拓扑顺序自动推进各级 JOB 的可视化编排', '异常自愈（失败重试、超时回收、人工复核提醒）'],
  },
  // CMS 内容管理：系统已接入，业务菜单待整合（方案见 Docs/Refactor/2610-012）
  'cms-home': {
    icon: 'mdi-laptop',
    desc: '内容管理（CMS）：系统已接入，业务菜单待整合。后续将按 CMS 侧的内容管理菜单（博客、待办、笔记、书签、媒体库等）逐步接入。',
    features: ['内容管理业务菜单接入', '与金融投资系统的统一导航与权限', '跨系统数据与配置共享'],
  },
  // BRS 备份与恢复（需求 10872）：后端服务已实现，前端 API 待开放
  'brs-table': {
    icon: 'mdi-table-cog',
    desc: '备份登记维护：维护表级备份范围 `brs.brs_bak_rdb_table`（rdb_type / schema_name / table_name / flag_enable / priority 唯一 / bak_scope / compression / where_clause）。',
    features: ['登记行增删改查与启用开关', 'priority 唯一性与执行顺序校验', '备份范围（both/schema/data）与压缩算法选择'],
  },
  'brs-record': {
    icon: 'mdi-file-clock-outline',
    desc: '备份记录与产物：查看每日备份结果（`<PREFIX>/Backup/BRS/<yyyyMMdd>/`）下的 dump 产物、大小与上传对象存储的状态。',
    features: ['按日期/表浏览备份产物与文件大小', '单表备份成功/失败与上传结果汇总', '失败原因与日志（`[brs]` 前缀）联动查看'],
  },
  'brs-storage': {
    icon: 'mdi-cloud-cog-outline',
    desc: '对象存储配置：维护 `sys.sys_config 39002`（S3 兼容：Cloudflare R2 / AWS S3 / MinIO）的开关、endpoint、bucket、prefix 与凭证。',
    features: ['对象存储开关与连接参数维护（凭证脱敏展示，不落日志）', '连通性测试与上传结果校验', '按表级 `flag_upload` 控制是否上传'],
  },
  'brs-restore-run': {
    icon: 'mdi-database-arrow-left',
    desc: '数据恢复：从选定日期的备份产物中挑选表做 `pg_restore` 恢复（支持 --clean --if-exists、按表选择性恢复）。',
    features: ['按日期与表定位备份产物', '恢复前预检（表是否存在/影响行数/目标库）', '恢复执行与结果回显'],
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
          该模块与「量化策略验证 → 黄金期货合约回测验证」同属 FinvQuant策略回测量化交易能力矩阵。
          当前已落地：策略管理（结构化定义）、账户管理（回测账户）、回测分析（任务与报告）、
          黄金期货合约回测验证（GCMain 等标的通用回测）。本模块将在后续迭代中实现。
        </p>
      </v-card-text>
    </v-card>
  </v-container>
</template>
