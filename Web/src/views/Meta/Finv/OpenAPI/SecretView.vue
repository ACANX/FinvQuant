<script setup lang="ts">
// 开放平台 → OpenAPI 密钥配置（需求 11175）
//
// 为什么单独一页：19005（AES-256-GCM 密钥）是 32 字节的 Base64，手填极易出错；
// 而且换密钥会让已签发的 AK/SK 凭证解不开，必须一并重加密——这些都不适合放在通用配置页里裸改。
//
// 配置项：19004 KID / 19005 加密密钥 / 19006 兼容兜底密钥
// 接口：GET  /Meta/Finv/OpenAPI/Secret/Status
//       POST /Meta/Finv/OpenAPI/Secret/Save
import { computed, onMounted, ref } from 'vue'

import { apiGet, apiPost } from '../../../../api'
import { showToast } from '../../../../ui/toast'
import { OPENAPI_BASE as BASE } from './shared'

interface SecretView {
  secret_kid: string
  secret_key_ready: boolean
  fallback_key_ready: boolean
  count_credential: number
  count_stale_kid: number
  updated_credential: number
  skipped_credential: number
}

const state = ref<SecretView>({
  secret_kid: 'k1',
  secret_key_ready: false,
  fallback_key_ready: false,
  count_credential: 0,
  count_stale_kid: 0,
  updated_credential: 0,
  skipped_credential: 0,
})
const loading = ref(false)
const saving = ref(false)

const kid = ref('k1')
const newKey = ref('')
const rotateCipher = ref(true)
const fallbackKey = ref('')
const clearFallback = ref(false)

async function load() {
  loading.value = true
  try {
    const data = await apiGet<SecretView>(`${BASE}/Secret/Status`)
    state.value = data
    kid.value = data.secret_kid
  } catch (err) {
    showToast(`读取密钥配置失败：${(err as Error).message}`, 'error')
  } finally {
    loading.value = false
  }
}

/** 生成 32 字节随机密钥（Base64）——只用浏览器 CSPRNG，不经过任何服务端 */
function generateKey(): string {
  const raw = new Uint8Array(32)
  crypto.getRandomValues(raw)
  let binary = ''
  raw.forEach((byte) => {
    binary += String.fromCharCode(byte)
  })
  return btoa(binary)
}

function fillGenerated() {
  newKey.value = generateKey()
  rotateCipher.value = state.value.secret_key_ready
  showToast('已生成新密钥：请点「保存密钥配置」写入数据库', 'info')
}

async function copy(text: string, label: string) {
  if (!text) {
    showToast(`${label}为空，无需复制`, 'warning')
    return
  }
  try {
    await navigator.clipboard.writeText(text)
    showToast(`${label}已复制`, 'success')
  } catch {
    showToast('复制失败，请手动选择文本复制', 'warning')
  }
}

const kidError = computed(() => {
  const value = kid.value.trim()
  if (!value) return '不能为空'
  if (!/^[A-Za-z0-9._-]{1,16}$/.test(value)) return '只允许字母/数字/._-，1~16 位'
  return ''
})

const keyError = computed(() => {
  const value = newKey.value.trim()
  if (!value) return ''
  try {
    const decoded = atob(value)
    if (decoded.length !== 32) return `解码后必须是 32 字节（当前 ${decoded.length} 字节）`
  } catch {
    return '不是合法的 Base64'
  }
  return ''
})

const canSave = computed(
  () =>
    !kidError.value &&
    !keyError.value &&
    !saving.value &&
    // 至少改一项才让保存，避免误点空提交
    (kid.value.trim() !== state.value.secret_kid ||
      newKey.value.trim() !== '' ||
      fallbackKey.value.trim() !== '' ||
      clearFallback.value),
)

const rotateHint = computed(() => {
  if (!newKey.value.trim()) return ''
  if (!state.value.secret_key_ready) return '当前还没有加密密钥：本次是首次配置，不会影响已有凭证'
  return rotateCipher.value
    ? `保存时会用新密钥重加密现有 ${state.value.count_credential} 条凭证的密文（推荐）`
    : `⚠ 未勾选重加密：${state.value.count_credential} 条凭证的旧密文将无法解密，它们的 AK/SK 调用会失败（19010）`
})

async function save() {
  if (!canSave.value) {
    if (keyError.value) showToast(keyError.value, 'warning')
    return
  }
  if (newKey.value.trim() && state.value.secret_key_ready && !rotateCipher.value) {
    const ok = window.confirm('未勾选「重加密现有凭证」：保存后这些凭证的 AK/SK 调用会因无法解密而失败。确认继续？')
    if (!ok) return
  }
  saving.value = true
  try {
    const data = await apiPost<SecretView>(`${BASE}/Secret/Save`, {
      secret_kid: kid.value.trim(),
      secret_key_base64: newKey.value.trim(),
      rotate_cipher: rotateCipher.value,
      fallback_key: fallbackKey.value.trim(),
      clear_fallback_key: clearFallback.value,
    })
    state.value = data
    newKey.value = ''
    fallbackKey.value = ''
    clearFallback.value = false
    if (data.skipped_credential > 0) {
      showToast(
        `配置已保存，但 ${data.skipped_credential} 条凭证的旧密文解不开（已跳过），请重新签发这些凭证`,
        'warning',
      )
    } else {
      showToast(
        data.updated_credential > 0
          ? `配置已保存，已重加密 ${data.updated_credential} 条凭证`
          : '密钥配置已保存（10 秒内生效）',
        'success',
      )
    }
    await load()
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
        <v-icon icon="mdi-shield-key-outline" class="mr-1" />
        OpenAPI 密钥配置
        <v-chip size="small" variant="tonal" color="primary">sys.sys_config 19004 / 19005 / 19006</v-chip>
      </v-card-title>
      <v-card-subtitle class="pb-2">
        AK/SK 模式的 SK 以 AES-256-GCM 密文入库，加密密钥就是这里的 19005（32 字节）。保存后 10 秒内生效，不需要重启。
      </v-card-subtitle>

      <v-card-text>
        <div class="d-flex flex-wrap align-center ga-2 mb-4">
          <v-chip size="small" variant="tonal">KID：{{ state.secret_kid }}</v-chip>
          <v-chip size="small" variant="tonal" :color="state.secret_key_ready ? 'success' : 'warning'">
            19005 加密密钥：{{ state.secret_key_ready ? '已配置' : '未配置' }}
          </v-chip>
          <v-chip size="small" variant="tonal" :color="state.fallback_key_ready ? 'success' : 'warning'">
            19006 兜底密钥：{{ state.fallback_key_ready ? '已配置' : '未配置' }}
          </v-chip>
          <v-chip size="small" variant="tonal">持密文凭证：{{ state.count_credential }}</v-chip>
          <v-chip v-if="state.count_stale_kid > 0" size="small" variant="tonal" color="error">
            KID 不一致待重加密：{{ state.count_stale_kid }}
          </v-chip>
          <v-btn size="small" variant="text" prepend-icon="mdi-refresh" :loading="loading" @click="load">刷新</v-btn>
        </div>

        <v-alert
          v-if="!state.secret_key_ready"
          type="warning"
          density="compact"
          variant="tonal"
          class="mb-4"
          text="19005 未配置：静态 API Key 可正常签发与使用；AK/SK 模式无法签发（503）也无法验签（19010）。点下方「生成 32 字节密钥」即可。"
        />
        <v-alert
          v-else-if="state.count_stale_kid > 0"
          type="warning"
          density="compact"
          variant="tonal"
          class="mb-4"
          :text="`有 ${state.count_stale_kid} 条凭证的 secret_kid 与当前 KID 不一致，建议用当前密钥重加密一次（勾选「重加密现有凭证」后保存）。`"
        />

        <v-divider class="mb-4" />

        <div class="d-flex flex-column ga-3">
          <div class="d-flex flex-wrap ga-3">
            <v-text-field
              v-model="kid"
              label="密钥版本 KID（19004）"
              density="compact"
              variant="outlined"
              placeholder="k1"
              style="min-width: 200px; max-width: 260px"
              :error="kidError !== ''"
              :error-messages="kidError"
              hint="换密钥时建议同时递增（k1 → k2），便于区分历史密文"
              persistent-hint
            />
            <v-btn
              color="primary"
              variant="tonal"
              prepend-icon="mdi-key-plus"
              class="align-self-start"
              @click="fillGenerated"
            >
              生成 32 字节密钥
            </v-btn>
          </div>

          <v-textarea
            v-model="newKey"
            label="新的加密密钥（19005，Base64，32 字节；留空表示不修改）"
            placeholder="点上方「生成 32 字节密钥」自动填入，或粘贴已有的 Base64 值"
            density="compact"
            variant="outlined"
            rows="2"
            spellcheck="false"
            :error="keyError !== ''"
            :error-messages="keyError"
          >
            <template #append-inner>
              <v-btn
                size="small"
                variant="text"
                prepend-icon="mdi-content-copy"
                :disabled="!newKey"
                @click="copy(newKey, '新密钥')"
              >
                复制
              </v-btn>
            </template>
          </v-textarea>

          <div class="d-flex flex-wrap align-center ga-2">
            <v-checkbox
              v-model="rotateCipher"
              :label="`重加密现有凭证（${state.count_credential} 条）`"
              density="compact"
              hide-details
              :disabled="!state.secret_key_ready"
            />
            <span class="text-caption text-medium-emphasis">换密钥时必须勾选，否则已签发凭证的 SK 将无法解密</span>
          </div>
          <div v-if="rotateHint" class="text-caption text-medium-emphasis">{{ rotateHint }}</div>

          <v-divider />

          <div class="d-flex flex-wrap ga-3 align-start">
            <v-text-field
              v-model="fallbackKey"
              label="兼容兜底静态密钥（19006，留空表示不修改）"
              density="compact"
              variant="outlined"
              placeholder="过渡期给旧调用方用；与环境变量 FINV_OPENAPI_KEY 二者任一命中即放行"
              style="min-width: 320px; flex: 1"
              spellcheck="false"
              :disabled="clearFallback"
            />
            <v-checkbox
              v-model="clearFallback"
              label="清空兜底密钥"
              density="compact"
              hide-details
              class="mt-1"
            />
          </div>

          <div class="d-flex align-center ga-3">
            <v-btn
              color="primary"
              prepend-icon="mdi-content-save"
              :loading="saving"
              :disabled="!canSave"
              @click="save"
            >
              保存密钥配置
            </v-btn>
            <span v-if="!canSave && !saving" class="text-caption text-medium-emphasis">
              至少修改一项后才能保存
            </span>
          </div>
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
          ① <strong>19005 是什么</strong>：AES-256-GCM 密钥（32 字节，Base64 编码），用来加密
          <code>open.open_api_credential.secret_cipher</code>（AK/SK 模式的 SK）。HMAC 验签必须取回明文的 SK，
          所以 SK 只能加密、不能哈希，这把密钥就是唯一的解密钥匙——<strong>务必备份</strong>。
        </p>
        <p>
          ② <strong>密钥在浏览器本地生成</strong>：点「生成 32 字节密钥」用的是 <code>crypto.getRandomValues</code>，
          不经过任何服务端接口；生成后请先复制保存到安全位置，再点保存。
        </p>
        <p>
          ③ <strong>换密钥一定要重加密</strong>：勾选「重加密现有凭证」后，服务端会在<strong>同一个事务</strong>里
          先用旧密钥解出 SK、再用新密钥加密回库并更新时间戳与 KID；不勾选则旧密文再也解不开，
          那些凭证的 AK/SK 调用会返回 <code>19010</code>（只能重新签发）。
        </p>
        <p>
          ④ <strong>KID（19004）</strong>：密钥版本号，只用于标识；换钥时建议递增（<code>k1</code> → <code>k2</code>）。
          凭证行上的 <code>secret_kid</code> 会随重加密一起更新，页面顶部「KID 不一致待重加密」就是提醒这个。
        </p>
        <p>
          ⑤ <strong>19006 兜底密钥</strong>：过渡期给还没改造的旧调用方用，与环境变量
          <code>FINV_OPENAPI_KEY</code> 二者任一命中即放行（审计里记为「兼容兜底」）。
          正式调用方都改完、或鉴权切到强制模式并稳定后，清空它与环境变量即可关闭这条通道。
        </p>
        <p>
          ⑥ 静态 API Key（模式一）只存 <code>sha256</code> 哈希，<strong>不依赖 19005</strong>：
          即使 19005 未配置，静态 Key 也可正常签发与校验。
        </p>
      </v-card-text>
    </v-card>
  </div>
</template>

<style scoped>
.openapi-page {
  padding: 16px;
}
</style>
