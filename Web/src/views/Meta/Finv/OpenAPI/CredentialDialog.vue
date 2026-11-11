<script setup lang="ts">
// 签发 / 轮换 OpenAPI 凭证（需求 11169）
//
// 接口：POST /Meta/Finv/OpenAPI/Credential/Create  签发（密钥明文只返回这一次）
//       POST /Meta/Finv/OpenAPI/Credential/Rotate  轮换（新密钥只返回这一次）
//
// 交互：表单 → 提交 → 同一对话框切成「密钥展示」步骤；关闭后密钥即不可再取。
import { computed, ref, watch } from 'vue'

import { apiPost } from '../../../../api'
import { showToast } from '../../../../ui/toast'
import { timeText } from './shared'

interface ClientRow {
  idn: number
  client_code: string
  client_name: string
}

interface CredentialRow {
  idn: number
  client_idn: number
  client_code: string
  credential_no: string
  auth_mode: string
  access_key_id: string
  key_prefix: string
}

interface IssuedSecret {
  credential: CredentialRow
  access_key: string
  secret_key: string
  note: string
}

const props = defineProps<{
  modelValue: boolean
  mode: 'create' | 'rotate'
  clientIdn: number
  credentialIdn: number
  clients: ClientRow[]
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'saved'): void
}>()

const BASE = '/Meta/Finv/OpenAPI'

const AUTH_MODE_OPTIONS = [
  { title: '静态 API Key[key=APIKEY]', value: 'APIKEY' },
  { title: 'AK/SK 签名[key=AKSK]', value: 'AKSK' },
  { title: 'STS 临时凭据[key=STS]', value: 'STS' },
]

const form = ref({
  client_idn: 0,
  credential_no: '',
  auth_mode: 'APIKEY',
  scope_text: '',
  allow_cidr_text: '',
  dt_expire: '',
  remark: '',
})
const saving = ref(false)
const error = ref('')
const issued = ref<IssuedSecret | null>(null)
// STS 临时凭据的签发结果（三要素 + 到期，与落库凭证结构不同）
interface StsIssued {
  access_key_id: string
  secret_key: string
  security_token: string
  expire_at: string
  expire_seconds: number
}
const stsIssued = ref<StsIssued | null>(null)
const isSts = computed(() => form.value.auth_mode === 'STS')

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

const isRotate = computed(() => props.mode === 'rotate')
const title = computed(() => (isRotate.value ? '轮换密钥' : '签发凭证'))

const clientOptions = computed(() =>
  props.clients.map((row) => ({ title: `${row.client_code}（${row.client_name}）`, value: row.idn })),
)

const noError = computed(() => {
  if (isRotate.value) return ''
  const value = form.value.credential_no.trim()
  if (!value) return '不能为空'
  if (!/^[A-Z0-9][A-Z0-9._-]{1,47}$/.test(value)) return '只允许字母/数字/._-，2~48 位（字母统一大写）'
  return ''
})
// 静态 Key 的最长有效期 90 天（需求 11188）
const maxApiKeyExpire = computed(() => {
  const d = new Date()
  d.setDate(d.getDate() + 90)
  return d.toISOString().slice(0, 10)
})

const apiKeyExpireInvalid = computed(
  () => form.value.auth_mode === 'APIKEY' &&
    (!form.value.dt_expire || form.value.dt_expire > maxApiKeyExpire.value),
)

const stsScopeMissing = computed(
  () => isSts.value && form.value.scope_text.split('\n').map((item) => item.trim()).filter((item) => item !== '').length === 0,
)

const canSubmit = computed(() => {
  if (saving.value) return false
  if (isSts.value) return !stsScopeMissing.value && Number(form.value.client_idn) > 0
  return !noError.value && !apiKeyExpireInvalid.value
})

function resetForm() {
  form.value = {
    client_idn: props.clientIdn || props.clients[0]?.idn || 0,
    credential_no: '',
    auth_mode: 'APIKEY',
    scope_text: '',
    allow_cidr_text: '',
    dt_expire: '',
    remark: '',
  }
  issued.value = null
  stsIssued.value = null
  error.value = ''
}

function populateCredentialNo() {
  const stamp = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  form.value.credential_no =
    `CRED-${form.value.client_idn}-${stamp.getFullYear()}${pad(stamp.getMonth() + 1)}${pad(stamp.getDate())}` +
    `-${pad(stamp.getHours())}${pad(stamp.getMinutes())}`
}

async function submit() {
  if (!canSubmit.value) return
  saving.value = true
  error.value = ''
  try {
    const cidrs = form.value.allow_cidr_text
      .split(/[\n,;]/)
      .map((item) => item.trim())
      .filter((item) => item !== '')
    const scope = form.value.scope_text
      .split('\n')
      .map((item) => item.trim())
      .filter((item) => item !== '')

    if (isRotate.value) {
      issued.value = await apiPost<IssuedSecret>(`${BASE}/Credential/Rotate`, { idn: props.credentialIdn })
    } else if (isSts.value) {
      // STS 临时凭据：不落库、固定 2 小时，只返回三要素（需求 11200）
      stsIssued.value = await apiPost<StsIssued>(`${BASE}/Credential/StsIssue`, {
        client_idn: Number(form.value.client_idn),
        scope,
        allow_cidr: cidrs,
        remark: form.value.remark.trim(),
      })
    } else {
      issued.value = await apiPost<IssuedSecret>(`${BASE}/Credential/Create`, {
        client_idn: Number(form.value.client_idn),
        credential_no: form.value.credential_no.trim().toUpperCase(),
        auth_mode: form.value.auth_mode,
        scope,
        allow_cidr: cidrs,
        dt_expire: form.value.dt_expire ? `${form.value.dt_expire}T00:00:00+08:00` : '',
        remark: form.value.remark.trim(),
      })
    }
    showToast('密钥已生成，请立即复制保存（只显示这一次）', 'success')
    emit('saved')
  } catch (err) {
    error.value = (err as Error).message
    showToast(`${isRotate.value ? '轮换' : '签发'}失败：${(err as Error).message}`, 'error')
  } finally {
    saving.value = false
  }
}

async function copy(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text)
    showToast(`${label}已复制`, 'success')
  } catch {
    showToast('复制失败，请手动选择文本复制', 'warning')
  }
}

function close() {
  if (issued.value) {
    showToast('已关闭：密钥不再显示，如需重发请再次轮换', 'info')
  }
  visible.value = false
}

watch(
  () => [props.modelValue, props.mode, props.clientIdn, props.credentialIdn] as const,
  ([open]) => {
    if (!open) return
    resetForm()
    if (!isRotate.value) populateCredentialNo()
  },
)
</script>

<template>
  <v-dialog v-model="visible" max-width="760" persistent>
    <v-card>
      <v-card-title class="text-subtitle-1 d-flex align-center">
        <v-icon icon="mdi-key-plus" class="mr-2" />
        {{ title }}
        <v-chip v-if="isRotate" size="small" variant="tonal" color="warning" class="ml-2">旧密钥立即失效</v-chip>
      </v-card-title>
      <v-card-subtitle v-if="!issued">
        密钥只在提交成功后显示一次；库里静态 Key 只存 sha256、SK 只存 AES-256-GCM 密文，无法再取回。
      </v-card-subtitle>

      <!-- 步骤一：表单 -->
      <v-card-text v-if="!issued">
        <v-alert v-if="error" type="error" density="compact" variant="tonal" class="mb-3" :text="error" />
        <div class="d-flex flex-column ga-3">
          <div v-if="!isRotate" class="d-flex flex-wrap ga-3">
            <v-select
              v-model="form.client_idn"
              :items="clientOptions"
              item-title="title"
              item-value="value"
              label="调用方 *"
              density="compact"
              variant="outlined"
              style="min-width: 260px; flex: 1"
              @update:model-value="form.client_idn = Number($event)"
            />
            <v-text-field
              :model-value="form.credential_no"
              label="凭证号 *（字母统一大写）"
              placeholder="如 CRED-CDEU-01"
              @update:model-value="form.credential_no = String($event ?? '').toUpperCase()"
              density="compact"
              variant="outlined"
              style="min-width: 240px; flex: 1"
              :error="noError !== ''"
              :error-messages="noError"
            />
            <v-select
              v-model="form.auth_mode"
              :items="AUTH_MODE_OPTIONS"
              item-title="title"
              item-value="value"
              label="认证模式 *"
              density="compact"
              variant="outlined"
              style="min-width: 220px; flex: 1"
            />
          </div>
          <v-alert v-if="isSts" type="warning" density="compact" variant="tonal">
            STS 临时凭据：有效期固定 <strong>2 小时</strong>，不落库、不可续期、不能在凭证列表里管理；
            到期自动失效。授权范围必填（不允许放开全部接口）。
          </v-alert>
          <v-alert v-else-if="!isRotate" type="info" density="compact" variant="tonal">
            轮换会生成一套新密钥：静态 Key 的公开标识会改变；AK/SK 的旧 SK 在签名时间窗内仍可验签，之后失效。
          </v-alert>

          <v-alert
            v-if="!isRotate && form.auth_mode === 'APIKEY'"
            type="warning"
            density="compact"
            variant="tonal"
            class="mb-2"
          >
            静态 Key 只能授权 <strong>1 个具体接口</strong>（不允许留空或通配），且调用方必须先配置调用额度；
            需要多接口请改用 AK/SK 签名模式。写接口调用时还必须带查询参数 <code>_</code>（毫秒时间戳，±60 秒）。
          </v-alert>
          <v-textarea
            v-if="!isRotate"
            v-model="form.scope_text"
            :label="form.auth_mode === 'APIKEY'
              ? '授权接口（1 条「方法 路径」，必填）'
              : isSts
                ? '授权范围（每行一条「方法 路径」，必填）'
                : '授权范围（每行一条「方法 路径」，留空 = 全部接口）'"
            :placeholder="form.auth_mode === 'APIKEY'
              ? 'POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert'
              : 'POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert\nGET /OpenAPI/V1/Meta/Finv/Quote/Cdeu/*'"
            density="compact"
            variant="outlined"
            rows="3"
          />
          <v-textarea
            v-if="!isRotate"
            v-model="form.allow_cidr_text"
            label="IP 白名单（每行一个 IP 或 CIDR，留空 = 不限）"
            density="compact"
            variant="outlined"
            rows="2"
          />
          <div v-if="!isRotate" class="d-flex flex-wrap ga-3">
            <v-text-field
              v-if="!isSts"
              v-model="form.dt_expire"
              type="date"
              :max="form.auth_mode === 'APIKEY' ? maxApiKeyExpire : undefined"
              :label="form.auth_mode === 'APIKEY'
                ? '凭证到期（必填，最长 90 天）'
                : '凭证到期（留空 = 随调用方）'"
              :error-messages="apiKeyExpireInvalid
                ? '静态 Key 必须设置到期日，且不超过 90 天（' + maxApiKeyExpire + '）'
                : []"
              density="compact"
              variant="outlined"
              style="min-width: 200px; flex: 1"
            />
            <v-text-field
              v-model="form.remark"
              label="备注"
              density="compact"
              variant="outlined"
              style="min-width: 240px; flex: 2"
            />
          </div>
        </div>
      </v-card-text>

      <!-- 步骤二（STS）：三要素一次性展示 -->
      <v-card-text v-else-if="stsIssued">
        <v-alert type="warning" density="compact" variant="tonal" class="mb-3">
          三要素只显示这一次；有效期 2 小时（到 {{ timeText(stsIssued.expire_at) }}），到期自动失效、不可续期。
        </v-alert>
        <div class="d-flex flex-column ga-3">
          <div v-for="item in [
            { label: 'Access Key（SAK，临时）', value: stsIssued.access_key_id },
            { label: 'Secret Key（SSK，临时）', value: stsIssued.secret_key },
            { label: 'Security Token（SST）', value: stsIssued.security_token },
          ]" :key="item.label">
            <div class="text-caption text-medium-emphasis">{{ item.label }}</div>
            <div class="d-flex align-center ga-2">
              <code class="secret-value">{{ item.value }}</code>
              <v-btn size="small" variant="text" prepend-icon="mdi-content-copy" @click="copy(item.value, item.label)">
                复制
              </v-btn>
            </div>
          </div>
          <v-alert type="info" density="compact" variant="tonal">
            调用时三要素都要带：<code>Authorization: META1-HMAC-SHA256 Credential=&lt;SAK&gt;, Signature=...</code>
            + <code>X-META-TS</code> + <code>X-META-Nonce</code> + <code>X-META-Token: &lt;SST&gt;</code>，
            且 SecurityToken 参与签名（待签串第 7 段）。
          </v-alert>
        </div>
      </v-card-text>

      <!-- 步骤二：一次性密钥展示 -->
      <v-card-text v-else>
        <v-alert type="warning" density="compact" variant="tonal" class="mb-3" :text="issued.note" />
        <div class="d-flex flex-column ga-3">
          <div>
            <div class="text-caption text-medium-emphasis">凭证号 / 调用方</div>
            <div>
              <strong>{{ issued.credential.credential_no }}</strong>
              （{{ issued.credential.client_code }} · {{ issued.credential.auth_mode }}）
            </div>
          </div>
          <div>
            <div class="text-caption text-medium-emphasis">
              {{ issued.credential.auth_mode === 'AKSK' ? 'Access Key（AK，公开标识）' : '静态 API Key' }}
            </div>
            <div class="d-flex align-center ga-2">
              <code class="secret-value">{{ issued.access_key }}</code>
              <v-btn size="small" variant="text" prepend-icon="mdi-content-copy" @click="copy(issued.access_key, 'AK')">
                复制
              </v-btn>
            </div>
          </div>
          <div v-if="issued.credential.auth_mode === 'AKSK'">
            <div class="text-caption text-medium-emphasis">Secret Key（SK，用于 HMAC 签名）</div>
            <div class="d-flex align-center ga-2">
              <code class="secret-value">{{ issued.secret_key }}</code>
              <v-btn size="small" variant="text" prepend-icon="mdi-content-copy" @click="copy(issued.secret_key, 'SK')">
                复制
              </v-btn>
            </div>
          </div>
          <v-alert type="info" density="compact" variant="tonal">
            关闭对话框后密钥不再显示。请立即粘贴到调用方的配置里；如果丢失，只能轮换生成新密钥。
          </v-alert>
        </div>
      </v-card-text>

      <v-card-actions>
        <v-spacer />
        <template v-if="!issued">
          <v-btn variant="text" :disabled="saving" @click="visible = false">取消</v-btn>
          <v-btn color="primary" :loading="saving" :disabled="!canSubmit" @click="submit">
            {{ isRotate ? '确认轮换' : '签发' }}
          </v-btn>
        </template>
        <v-btn v-else color="primary" @click="close">我已保存，关闭</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.secret-value {
  display: inline-block;
  padding: 4px 8px;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.06);
  word-break: break-all;
  font-size: 13px;
}
</style>
