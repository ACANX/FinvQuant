<script setup lang="ts">
// BRS 备份调度配置（sys.sys_config 39001，6 段 cron：秒 分 时 日 月 周）
// 接口：GET  /Meta/BRS/Backup/Finv/Schedule/Get   读取当前表达式 + 未来 5 次执行时间
//       POST /Meta/BRS/Backup/Finv/Schedule/Save  保存（后端校验 6 段）
// 生效：调度器每 60 秒重新读取 39001，保存后最多 1 分钟生效，无需重启。
import { onMounted, ref } from 'vue'

import { apiGet, apiPost } from '../../../../../api'
import { showToast } from '../../../../../ui/toast'

interface ScheduleConfig {
  cron_expr: string
  remark: string
  gmt_modify: string | null
  next_runs: string[]
}

const API_PREFIX = '/Meta/BRS/Backup/Finv/Schedule'

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const config = ref<ScheduleConfig>({ cron_expr: '', remark: '', gmt_modify: null, next_runs: [] })
const cronExpr = ref('')

// 常用预设：直接填进输入框，仍需点保存
const presets = [
  { title: '每天 03:00:00', expr: '0 0 3 * * *' },
  { title: '每天 02:30:00', expr: '0 30 2 * * *' },
  { title: '每 6 小时（0 分 0 秒）', expr: '0 0 */6 * * *' },
  { title: '每小时整点', expr: '0 0 * * * *' },
  { title: '每 5 分钟', expr: '0 */5 * * * *' },
]

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet<ScheduleConfig>(`${API_PREFIX}/Get`)
    config.value = data
    cronExpr.value = data.cron_expr
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    const data = await apiPost<ScheduleConfig>(`${API_PREFIX}/Save`, { cron_expr: cronExpr.value.trim() })
    config.value = data
    cronExpr.value = data.cron_expr
    showToast('保存成功：调度器最多 1 分钟内按新表达式执行')
  } catch (e) {
    showToast((e as Error).message, 'error')
  } finally {
    saving.value = false
  }
}

function applyPreset(expr: string) {
  cronExpr.value = expr
}

function formatTime(v: string | null): string {
  if (!v) return '-'
  return v.replace('T', ' ').slice(0, 19)
}

onMounted(load)
</script>

<template>
  <v-card>
    <v-card-title>
      <v-icon icon="mdi-calendar-clock" class="mr-2" />
      备份调度配置
    </v-card-title>
    <v-card-subtitle>
      系统配置 <code>sys.sys_config</code> 39001：自动模式（定时任务）的 6 段 cron 表达式（秒 分 时 日 月 周）。保存后最多 1 分钟生效，无需重启。
    </v-card-subtitle>

    <v-card-text>
      <v-alert v-if="error" type="error" class="mb-3" density="compact">{{ error }}</v-alert>

      <v-row dense>
        <v-col cols="12" sm="6">
          <v-text-field
            v-model="cronExpr"
            label="cron 表达式（6 段：秒 分 时 日 月 周）"
            placeholder="0 0 3 * * *"
            density="compact"
            hide-details
            @keyup.enter="save"
          />
        </v-col>
        <v-col cols="auto" class="d-flex align-center">
          <v-btn color="primary" prepend-icon="mdi-content-save" :loading="saving" @click="save">保存</v-btn>
          <v-btn variant="text" class="ml-2" prepend-icon="mdi-refresh" :loading="loading" @click="load">重新加载</v-btn>
        </v-col>
      </v-row>

      <div class="text-caption text-medium-emphasis mt-3 mb-1">常用预设（点击填入，仍需保存）</div>
      <div class="d-flex flex-wrap ga-2">
        <v-chip
          v-for="p in presets"
          :key="p.expr"
          size="small"
          variant="tonal"
          class="cursor-pointer"
          @click="applyPreset(p.expr)"
        >
          {{ p.title }} · <code class="ml-1">{{ p.expr }}</code>
        </v-chip>
      </div>

      <v-row dense class="mt-4">
        <v-col cols="12" sm="6">
          <div class="text-subtitle-2 mb-1">当前生效表达式</div>
          <code>{{ config.cron_expr || '（未配置）' }}</code>
        </v-col>
        <v-col cols="12" sm="6">
          <div class="text-subtitle-2 mb-1">最近修改</div>
          <span class="text-medium-emphasis">{{ formatTime(config.gmt_modify) }}</span>
        </v-col>
      </v-row>

      <div class="text-subtitle-2 mt-4 mb-1">未来 5 次执行时间（服务器本地时区）</div>
      <ul class="text-body-2 text-medium-emphasis">
        <li v-for="t in config.next_runs" :key="t">{{ t }}</li>
        <li v-if="config.next_runs.length === 0">（表达式无效或未来 5 年内无命中）</li>
      </ul>
    </v-card-text>
  </v-card>

  <!-- 配置说明：标签/内容两列对齐，无冒号 -->
  <v-card class="mt-4" variant="flat">
    <v-card-title class="text-subtitle-2">配置说明</v-card-title>
    <v-card-text class="text-body-2 text-medium-emphasis">
      <v-row dense>
        <v-col cols="4" sm="3">表达式格式</v-col>
        <v-col cols="8" sm="9">
          6 段：秒 分 时 日 月 周，如 <code>0 0 3 * * *</code>（每天 03:00:00）；支持 <code>*</code>、
          <code>?</code>（仅日/周）、范围 <code>a-b</code>、步长 <code>*/n</code>、列表 <code>a,b,c</code>
        </v-col>
      </v-row>
      <v-row dense>
        <v-col cols="4" sm="3">自动模式</v-col>
        <v-col cols="8" sm="9">按本表达式定时执行；命中时若已有任务在跑则跳过本次</v-col>
      </v-row>
      <v-row dense>
        <v-col cols="4" sm="3">手动模式</v-col>
        <v-col cols="8" sm="9">「小表全量备份」页面随时触发，与自动模式共用同一执行链</v-col>
      </v-row>
      <v-row dense>
        <v-col cols="4" sm="3">生效时机</v-col>
        <v-col cols="8" sm="9">调度器每 60 秒重新读取 39001，保存后最多 1 分钟生效</v-col>
      </v-row>
    </v-card-text>
  </v-card>
</template>
