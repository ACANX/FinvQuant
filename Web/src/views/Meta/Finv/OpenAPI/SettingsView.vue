<script setup lang="ts">
// 开放平台 → OpenAPI 鉴权配置（需求 11173 拆分自原单页）
//
// 配置项（sys.sys_config 19xxx）：19001 鉴权模式 / 19002 签名时间窗 / 19003 日志保留天数
//                              19007 凭证缓存 TTL / 19004 密钥 KID / 19005 加密密钥 / 19006 兼容兜底密钥
//                              （19004~19006 只显示是否已配置，在密钥配置页维护）
// 接口：GET /Meta/Finv/OpenAPI/Settings、PUT /Meta/Finv/OpenAPI/Settings
import { computed, onMounted, ref } from 'vue'

import { apiGet, apiPut } from '../../../../api'
import { showToast } from '../../../../ui/toast'
import { MODE_LABELS, MODE_OPTIONS, OPENAPI_BASE as BASE } from './shared'

interface Settings {
  mode: string
  sign_time_window: number
  log_retain_days: number
  secret_kid: string
  secret_key_ready: boolean
  fallback_key_ready: boolean
  cred_cache_ttl: number
}

const settings = ref<Settings>({
  mode: '0',
  sign_time_window: 300,
  log_retain_days: 90,
  secret_kid: 'k1',
  secret_key_ready: false,
  fallback_key_ready: false,
  cred_cache_ttl: 10,
})
const mode = ref('0')
const windowSeconds = ref(300)
const retainDays = ref(90)
const credCacheTTL = ref(10)
const saving = ref(false)

async function load() {
  try {
    const data = await apiGet<Settings>(`${BASE}/Settings`)
    settings.value = data
    mode.value = data.mode
    windowSeconds.value = data.sign_time_window
    retainDays.value = data.log_retain_days
    credCacheTTL.value = data.cred_cache_ttl
  } catch (err) {
    showToast(`读取鉴权配置失败：${(err as Error).message}`, 'error')
  }
}

const windowError = computed(() => {
  const value = Number(windowSeconds.value)
  if (!Number.isFinite(value) || value < 30 || value > 3600) return '需在 30 ~ 3600 秒之间'
  return ''
})
const retainError = computed(() => {
  const value = Number(retainDays.value)
  if (!Number.isFinite(value) || value < 0 || value > 3650) return '需在 0 ~ 3650 天之间（0 = 不清理）'
  return ''
})
const credCacheError = computed(() => {
  const value = Number(credCacheTTL.value)
  if (!Number.isFinite(value) || value < 0 || value > 600) return '需在 0 ~ 600 秒之间（0 = 关闭缓存）'
  return ''
})
const canSave = computed(
  () => !windowError.value && !retainError.value && !credCacheError.value && !saving.value,
)

const modeHint = computed(() => {
  switch (mode.value) {
    case '1':
      return '强制鉴权：未通过校验的调用一律返回 190xx'
    case '2':
      return '宽松模式：照常校验并写审计，但失败仍放行（旧调用方灰度改造用）'
    default:
      return '鉴权关闭：全部放行，行为与早期过渡实现一致（不写审计）'
  }
})

async function save() {
  if (!canSave.value) return
  saving.value = true
  try {
    const data = await apiPut<Settings>(`${BASE}/Settings`, {
      mode: mode.value,
      sign_time_window: Number(windowSeconds.value),
      log_retain_days: Number(retainDays.value),
      cred_cache_ttl: Number(credCacheTTL.value),
    })
    settings.value = data
    showToast('鉴权配置已保存（10 秒内生效）', 'success')
  } catch (err) {
    showToast(`保存失败：${(err as Error).message}`, 'error')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="openapi-page">
    <v-card variant="outlined">
      <v-card-title class="d-flex align-center flex-wrap ga-2">
        <v-icon icon="mdi-shield-lock-outline" class="mr-1" />
        OpenAPI 鉴权配置
        <v-chip size="small" variant="tonal" color="primary">sys.sys_config 19xxx</v-chip>
        <v-chip size="small" variant="tonal" :color="settings.mode === '1' ? 'success' : 'warning'">
          当前：{{ MODE_LABELS[settings.mode] ?? settings.mode }}
        </v-chip>
      </v-card-title>
      <v-card-subtitle class="pb-2">
        控制 <code>/OpenAPI/V1/*</code> 的鉴权行为。保存后 <strong>10 秒内生效</strong>，不需要重启服务。
      </v-card-subtitle>

      <v-card-text>
        <div class="d-flex flex-column ga-4">
          <!-- 一个字段一行（FE17）：横向并排会让标签与校验提示挤在一起 -->
          <v-select
            v-model="mode"
            :items="MODE_OPTIONS"
            item-title="title"
            item-value="value"
            label="鉴权模式（19001）"
            density="compact"
            variant="outlined"
            class="settings-field"
            @update:model-value="mode = String($event)"
          />
          <v-text-field
            v-model.number="windowSeconds"
            type="number"
            label="签名时间窗（秒，19002）"
            density="compact"
            variant="outlined"
            min="30"
            max="3600"
            class="settings-field"
            :error="windowError !== ''"
            :error-messages="windowError"
          />
          <v-text-field
            v-model.number="retainDays"
            type="number"
            label="审计日志保留天数（19003）"
            density="compact"
            variant="outlined"
            min="0"
            max="3650"
            class="settings-field"
            :error="retainError !== ''"
            :error-messages="retainError"
          />
          <v-text-field
            v-model.number="credCacheTTL"
            type="number"
            label="凭证缓存 TTL（秒，19007）"
            density="compact"
            variant="outlined"
            min="0"
            max="600"
            class="settings-field"
            :error="credCacheError !== ''"
            :error-messages="credCacheError"
            hint="0 = 关闭缓存（每次查凭证表）"
            persistent-hint
          />
          <div>
            <v-btn color="primary" prepend-icon="mdi-content-save" :loading="saving" :disabled="!canSave" @click="save">
              保存配置
            </v-btn>
          </div>

          <div class="text-caption text-medium-emphasis">{{ modeHint }}</div>

          <v-divider />

          <div class="d-flex flex-wrap align-center ga-2">
            <span class="text-caption text-medium-emphasis">密钥状态（只读，不回显内容）：</span>
            <v-chip size="small" variant="tonal">版本 KID：{{ settings.secret_kid }}</v-chip>
            <v-chip size="small" variant="tonal" :color="settings.secret_key_ready ? 'success' : 'warning'">
              19005 加密密钥：{{ settings.secret_key_ready ? '已配置' : '未配置' }}
            </v-chip>
            <v-chip size="small" variant="tonal" :color="settings.fallback_key_ready ? 'success' : 'warning'">
              19006 兼容兜底密钥：{{ settings.fallback_key_ready ? '已配置' : '未配置' }}
            </v-chip>
          </div>

          <v-alert
            v-if="!settings.secret_key_ready"
            type="info"
            density="compact"
            variant="tonal"
            text="19005 未配置：静态 API Key 可正常签发与使用；AK/SK 模式会被拒绝（签发 503、鉴权 19010）。请到【开放平台 → 安全设置 → OpenAPI 密钥配置】页生成并保存 32 字节密钥。"
          />
        </div>
      </v-card-text>
    </v-card>

    <v-card class="mt-4" variant="outlined">
      <v-card-title class="text-subtitle-1">
        <v-icon icon="mdi-help-circle-outline" class="mr-1" />
        说明
      </v-card-title>
      <v-card-text class="text-body-2">
        <p>
          ① <strong>鉴权模式</strong>：<code>0</code> 关闭 = 全部放行且不写审计（默认，等价早期过渡实现）；
          <code>1</code> 启用 = 未通过校验即返回 <code>190xx</code>；<code>2</code> 宽松 = 照常校验并写审计，
          但失败仍放行——用于旧调用方灰度改造期间观察差异。
        </p>
        <p>
          ② <strong>签名时间窗</strong>：AK/SK 请求的 <code>X-Finv-Timestamp</code> 与服务器时间允许的最大偏差；
          同时决定 Redis 里 nonce 去重保留时长（= 2 倍时间窗）。
        </p>
        <p>
          ③ <strong>审计日志保留天数</strong>：<code>open.open_api_invoke_log</code> 按 <code>dt_request</code>
          清理更早记录；<code>0</code> 表示不清理（不建议）。
        </p>
        <p>
          ③-b <strong>凭证缓存 TTL</strong>：鉴权读凭证走 Redis 缓存，稳态下每个活跃凭证每 TTL 最多查表 1 次；
          <code>0</code> = 关闭缓存（每次请求都查凭证表）。通过页面吊销/停用/轮换凭证会<strong>立即</strong>让缓存失效，
          该值只影响「直接改库」的生效延迟与查库频率。
        </p>
        <p>
          ④ <strong>密钥项不在本页修改</strong>：<code>19004</code> KID 与 <code>19005</code> 加密密钥、
          <code>19006</code> 兼容兜底密钥都属敏感配置，请在「开放平台 → 安全设置 → OpenAPI 密钥配置」页维护；本页只显示是否已配置。
        </p>
        <p>
          ⑤ <strong>推荐上线顺序</strong>：保持 <code>0</code> → 在凭证页登记调用方与凭证 → 切到 <code>2</code>
          观察审计里的失败 → 清零后切 <code>1</code> → 过渡结束清空 <code>19006</code> 与环境变量
          <code>FINV_OPENAPI_KEY</code>。
        </p>
      </v-card-text>
    </v-card>
  </div>
</template>

<style scoped>
.openapi-page {
  padding: 16px;
}

/* 每个字段独占一行，宽度限制在可读区间（FE17：一个字段一行） */
.settings-field {
  max-width: 420px;
}
</style>
