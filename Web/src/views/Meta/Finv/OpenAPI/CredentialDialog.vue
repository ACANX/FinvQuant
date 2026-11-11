<script setup lang="ts">
// 签发 / 轮换 OpenAPI 凭证（需求 11169）
//
// 接口：POST /Meta/Finv/OpenAPI/Credential/Create  签发（密钥明文只返回这一次）
//       POST /Meta/Finv/OpenAPI/Credential/Rotate  轮换（新密钥只返回这一次）
//
// 交互：表单 → 提交 → 同一对话框切成「密钥展示」步骤；关闭后密钥即不可再取。
import { computed, ref, watch } from 'vue'

import { apiGet, apiPost } from '../../../../api'
import { showToast } from '../../../../ui/toast'

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

// 「授权范围」下拉的可选项（open.open_api_endpoint，需求 11224）
interface EndpointOption {
  idn: number
  api_code: string
  http_method: string
  path: string
  api_name: string
  api_group: string
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
  { title: '静态 API Key[APIKEY]', value: 'APIKEY' },
  { title: 'AK/SK 签名[AKSK]', value: 'AKSK' },
  { title: 'STS 临时凭据[STS]', value: 'STS' },
]

const form = ref({
  client_idn: 0,
  credential_no: '',
  auth_mode: 'APIKEY',
  scope_picked: [] as string[],
  scope_text: '',
  allow_cidr_text: '',
  dt_expire: '',
  remark: '',
})
// 授权范围下拉：选中项与手输规则合并成最终 scope（前者来自接口清单，后者兜底通配）
const endpointOptions = ref<{ title: string; value: string }[]>([])
const endpointLoading = ref(false)
const saving = ref(false)
const error = ref('')
const issued = ref<IssuedSecret | null>(null)
// STS 模式（需求 11210）：签发的同样是 AK/SK，只是这对 AK/SK 是「兑换凭证」，
// 只能调 POST /OpenAPI/V1/Meta/Auth/Credential/STS/Issue 换 2 小时三要素。
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

const canSubmit = computed(() => {
  if (saving.value) return false
  // STS 走与 AKSK 相同的签发流程（生成 AK/SK 兑换凭证），凭证号同样必填
  return !noError.value && !apiKeyExpireInvalid.value
})

function resetForm() {
  form.value = {
    client_idn: props.clientIdn || props.clients[0]?.idn || 0,
    credential_no: '',
    auth_mode: 'APIKEY',
    scope_picked: [],
    scope_text: '',
    allow_cidr_text: '',
    dt_expire: '',
    remark: '',
  }
  issued.value = null
  error.value = ''
}

function populateCredentialNo() {
  const stamp = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  form.value.credential_no =
    `CRED-${form.value.client_idn}-${stamp.getFullYear()}${pad(stamp.getMonth() + 1)}${pad(stamp.getDate())}` +
    `-${pad(stamp.getHours())}${pad(stamp.getMinutes())}`
}

// loadEndpoints 拉取授权范围下拉的可选项（只含公开 + 启用 + 未下线的接口）。
//
// 失败不阻断签发：下拉拉不到时还能手输路径（界面会提示）。
async function loadEndpoints() {
  endpointLoading.value = true
  try {
    const data = await apiGet<{ list: EndpointOption[] }>(`${BASE}/Endpoint/Options`)
    endpointOptions.value = (data.list ?? []).map((row) => ({
      title: `${row.api_name}［${row.http_method} ${row.api_code}］`,
      value: `${row.http_method} ${row.path}`,
    }))
  } catch (err) {
    endpointOptions.value = []
    showToast(`读取接口清单失败（仍可手输路径）：${(err as Error).message}`, 'warning')
  } finally {
    endpointLoading.value = false
  }
}

// 静态 Key 只能授权 1 个接口（需求 11188）：多选时只保留最后一个，避免提交后才 400
watch(
  () => form.value.scope_picked,
  (picked) => {
    if (form.value.auth_mode === 'APIKEY' && picked.length > 1) {
      form.value.scope_picked = [picked[picked.length - 1]]
      showToast('静态 Key 只能授权 1 个接口，已保留最后一次选择', 'warning')
    }
  },
)

// finalScope 下拉选中 + 手输补充，去重后按顺序返回。
const finalScope = computed(() => {
  const merged: string[] = []
  const seen = new Set<string>()
  for (const item of [...form.value.scope_picked, ...form.value.scope_text.split('\n')]) {
    const value = item.trim()
    if (!value || seen.has(value)) continue
    seen.add(value)
    merged.push(value)
  }
  return merged
})

async function submit() {
  if (!canSubmit.value) return
  saving.value = true
  error.value = ''
  try {
    const cidrs = form.value.allow_cidr_text
      .split(/[\n,;]/)
      .map((item) => item.trim())
      .filter((item) => item !== '')
    const scope = finalScope.value

    if (isRotate.value) {
      issued.value = await apiPost<IssuedSecret>(`${BASE}/Credential/Rotate`, { idn: props.credentialIdn })
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
    if (!isRotate.value) {
      populateCredentialNo()
      void loadEndpoints() // 授权范围下拉（需求 11224）
    }
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
            STS 兑换凭证：签发的是一对长期 <strong>AK/SK</strong>，但它<strong>不能直接调业务接口</strong>，
            只能调用 <code>POST /OpenAPI/V1/Meta/Auth/Credential/STS/Issue</code> 换取 2 小时有效的
            SAK/SSK/SST 三要素再用三要素调业务接口。下面的授权范围是<strong>派生出的临时凭据</strong>的权限（留空 = 全部接口）。
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
          <!-- 授权范围（需求 11224）：从「接口清单」下拉选，避免手敲路径打错；
               通配规则（尾部 /*）无法枚举，仍保留手输兜底。 -->
          <v-select
            v-if="!isRotate"
            v-model="form.scope_picked"
            :items="endpointOptions"
            item-title="title"
            item-value="value"
            :return-object="false"
            :multiple="form.auth_mode !== 'APIKEY'"
            :chips="form.auth_mode !== 'APIKEY'"
            :closable-chips="form.auth_mode !== 'APIKEY'"
            :loading="endpointLoading"
            :label="form.auth_mode === 'APIKEY'
              ? '授权接口（静态 Key 只能选 1 个）'
              : '授权接口（可多选；留空且无补充规则 = 全部接口）'"
            :hint="form.auth_mode === 'APIKEY'
              ? '静态 Key 只允许 1 个具体接口'
              : '选项来自「开放平台 → 安全设置 → 接口清单」，只列对外公开且启用中的接口'"
            persistent-hint
            density="compact"
            variant="outlined"
            class="mb-2"
          />
          <v-textarea
            v-if="!isRotate"
            v-model="form.scope_text"
            label="补充规则（每行一条「方法 路径」，通配用尾部 /*，可留空）"
            placeholder="GET /OpenAPI/V1/Meta/Finv/Quote/Cdeu/*"
            :hint="`已选 ${form.scope_picked.length} 个接口 + 手输补充，合计 ${finalScope.length} 条规则`"
            persistent-hint
            density="compact"
            variant="outlined"
            rows="2"
            class="mb-2"
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

      <!-- 步骤二：一次性密钥展示（APIKEY 静态 Key / AKSK 长期签名 / STS 兑换凭证） -->
      <v-card-text v-else-if="issued">
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
              {{ issued.credential.auth_mode === 'AKSK' || issued.credential.auth_mode === 'STS' ? 'Access Key（AK，公开标识）' : '静态 API Key' }}
            </div>
            <div class="d-flex align-center ga-2">
              <code class="secret-value">{{ issued.access_key }}</code>
              <v-btn size="small" variant="text" prepend-icon="mdi-content-copy" @click="copy(issued.access_key, 'AK')">
                复制
              </v-btn>
            </div>
          </div>
          <div v-if="issued.credential.auth_mode === 'AKSK' || issued.credential.auth_mode === 'STS'">
            <div class="text-caption text-medium-emphasis">Secret Key（SK，用于 HMAC 签名）</div>
            <div class="d-flex align-center ga-2">
              <code class="secret-value">{{ issued.secret_key }}</code>
              <v-btn size="small" variant="text" prepend-icon="mdi-content-copy" @click="copy(issued.secret_key, 'SK')">
                复制
              </v-btn>
            </div>
          </div>
          <v-alert v-if="issued.credential.auth_mode === 'STS'" type="info" density="compact" variant="tonal">
            <div class="mb-1"><strong>这对 AK/SK 不能直接调业务接口</strong>，只能用它换取 2 小时的临时凭据：</div>
            <div class="text-caption">
              ① <code>POST /OpenAPI/V1/Meta/Auth/Credential/STS/Issue</code>（用本 AK/SK 按标准方式签名）<br>
              ② 返回 <code>SAK_LIVE_</code> / <code>SSK_LIVE_</code> / <code>SST_LIVE_</code> 三要素，有效期 2 小时<br>
              ③ 用三要素调业务接口，并额外带 <code>X-META-Token: &lt;SST&gt;</code>（参与签名，待签串第 7 段）<br>
              ④ 临时凭据到期后重新换一次即可；兑换凭证长住，不需要每次来平台签发
            </div>
            <div class="text-caption mt-1">
              完整示例见 <code>Docs/API/OpenAPI/Auth.md</code> §4.8。
            </div>
          </v-alert>
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
