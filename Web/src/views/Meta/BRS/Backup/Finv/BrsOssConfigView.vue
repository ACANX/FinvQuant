<script setup lang="ts">
// BRS 对象存储配置（sys.sys_config 39002 的 JSON 可视化配置）
// 接口：GET  /Meta/BRS/Backup/Finv/OSSConfig/Get   读取（不含 secret_key 明文，返回 secret_key_set）
//       POST /Meta/BRS/Backup/Finv/OSSConfig/Save  保存（secret_key 留空表示不修改）
// 生效时机：下一轮备份读取 39002，保存后无需重启。
import { computed, onMounted, ref } from 'vue'

import { apiGet, apiPost } from '../../../../../api'

interface OssConfig {
  enabled: boolean
  provider: string
  endpoint: string
  bucket: string
  region: string
  access_key: string
  secret_key: string
  prefix: string
  secret_key_set: boolean
  remark: string
  gmt_modify: string | null
}

const API_PREFIX = '/Meta/BRS/Backup/Finv/OSSConfig'

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const message = ref('')

const form = ref<OssConfig>({
  enabled: false,
  provider: 's3',
  endpoint: '',
  bucket: '',
  region: 'auto',
  access_key: '',
  secret_key: '',
  prefix: 'brs/',
  secret_key_set: false,
  remark: '',
  gmt_modify: null,
})

// secret_key 输入框提示：已配置过时留空表示不修改
const secretHint = computed(() =>
  form.value.secret_key_set ? '已配置（留空表示不修改）' : '未配置；启用上传时必填',
)

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet<OssConfig>(`${API_PREFIX}/Get`)
    form.value = { ...form.value, ...data, secret_key: '' }
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

async function save() {
  error.value = ''
  message.value = ''
  saving.value = true
  try {
    const data = await apiPost<OssConfig>(`${API_PREFIX}/Save`, {
      enabled: form.value.enabled,
      provider: form.value.provider.trim() || 's3',
      endpoint: form.value.endpoint.trim(),
      bucket: form.value.bucket.trim(),
      region: form.value.region.trim() || 'auto',
      access_key: form.value.access_key.trim(),
      secret_key: form.value.secret_key.trim(),
      prefix: form.value.prefix.trim() || 'brs/',
    })
    form.value = { ...form.value, ...data, secret_key: '' }
    message.value = data.enabled ? '保存成功：下一轮备份将上传到对象存储' : '保存成功：当前为「仅本地备份」，不上传'
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    saving.value = false
  }
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
      <v-icon icon="mdi-cloud-cog-outline" class="mr-2" />
      对象存储配置
    </v-card-title>
    <v-card-subtitle>
      系统配置 <code>sys.sys_config</code> 39002（JSON）：BRS 备份产物上传的 S3 兼容对象存储。保存后下一轮备份生效，无需重启。
    </v-card-subtitle>

    <v-card-text>
      <v-alert v-if="error" type="error" class="mb-3" density="compact">{{ error }}</v-alert>
      <v-alert v-if="message" type="success" class="mb-3" density="compact">{{ message }}</v-alert>

      <v-row dense>
        <v-col cols="12" sm="6">
          <v-switch
            v-model="form.enabled"
            label="启用上传（关闭则只保留本地备份）"
            color="primary"
            density="compact"
            hide-details
          />
        </v-col>
        <v-col cols="12" sm="6">
          <v-select
            v-model="form.provider"
            :items="[{ title: 'S3 兼容', value: 's3' }]"
            item-title="title"
            item-value="value"
            label="提供方"
            density="compact"
            hide-details
            :disabled="!form.enabled"
          />
        </v-col>

        <v-col cols="12">
          <v-text-field
            v-model="form.endpoint"
            label="Endpoint（R2：https://<account_id>.r2.cloudflarestorage.com；AWS S3 留空；MinIO：http://<host>:9000）"
            density="compact"
            :disabled="!form.enabled"
            :rules="[(v: string) => !form.enabled || !!v || '启用上传时必填']"
          />
        </v-col>
        <v-col cols="12" sm="6">
          <v-text-field
            v-model="form.bucket"
            label="Bucket（需提前创建）"
            density="compact"
            :disabled="!form.enabled"
            :rules="[(v: string) => !form.enabled || !!v || '启用上传时必填']"
          />
        </v-col>
        <v-col cols="12" sm="6">
          <v-text-field
            v-model="form.region"
            label="Region（R2 填 auto；AWS 填实际区域；MinIO 常用 us-east-1）"
            density="compact"
            :disabled="!form.enabled"
          />
        </v-col>

        <v-col cols="12" sm="6">
          <v-text-field
            v-model="form.access_key"
            label="Access Key ID"
            density="compact"
            autocomplete="off"
            :disabled="!form.enabled"
            :rules="[(v: string) => !form.enabled || !!v || '启用上传时必填']"
          />
        </v-col>
        <v-col cols="12" sm="6">
          <v-text-field
            v-model="form.secret_key"
            label="Secret Access Key"
            :hint="secretHint"
            persistent-hint
            type="password"
            density="compact"
            autocomplete="new-password"
            :disabled="!form.enabled"
          />
        </v-col>

        <v-col cols="12" sm="6">
          <v-text-field
            v-model="form.prefix"
            label="对象 Key 前缀（最终 key = <前缀>/<yyyyMMdd>/<schema>.<table>_<HHmmss>.dump）"
            density="compact"
            :disabled="!form.enabled"
          />
        </v-col>
        <v-col cols="12" sm="6" class="d-flex align-center">
          <div class="text-caption text-medium-emphasis">
            最近修改：{{ formatTime(form.gmt_modify) }}
          </div>
        </v-col>
      </v-row>

      <v-expansion-panels v-if="form.remark" class="mt-4" variant="accordion">
        <v-expansion-panel title="字段说明与配置要点（迁移内置说明）">
          <v-expansion-panel-text>
            <pre class="remark-text">{{ form.remark }}</pre>
          </v-expansion-panel-text>
        </v-expansion-panel>
      </v-expansion-panels>

      <div class="d-flex mt-4">
        <v-btn color="primary" prepend-icon="mdi-content-save" :loading="saving" @click="save">保存</v-btn>
        <v-btn variant="text" class="ml-2" prepend-icon="mdi-refresh" :loading="loading" @click="load">重新加载</v-btn>
      </div>
    </v-card-text>
  </v-card>

  <!-- 实现说明卡片：保留卡片容器，去掉 outlined 的白亮边框，内容为纯文字 -->
  <v-card class="mt-4" variant="flat">
    <v-card-title class="text-subtitle-2">实现说明</v-card-title>
    <v-card-text class="text-body-2 text-medium-emphasis">
      <v-row dense class="mb-1">
        <v-col cols="12" sm="6">
          S3 兼容协议：支持 Cloudflare R2、AWS S3、MinIO 等。
        </v-col>
        <v-col cols="12" sm="6">
          前置条件：需提前创建 Bucket；凭证需具备 Object Read &amp; Write 权限。
        </v-col>
      </v-row>
      <p class="mb-1">
        客户端实现：经 <code>internal/meta/base/oss</code> 抽象接入，默认适配器基于 <code>aws-sdk-go-v2</code>，不使用
        <code>minio-go</code>，后续可扩展 local 等适配器。
      </p>
      <p class="mb-1">生效时机：保存后下一轮备份生效，无需重启。</p>
      <p class="mb-0">
        凭证保存：存于本系统配置（键名 <code>ak</code>/<code>sk</code>）；Secret 读取不回传明文、留空表示不修改；上传失败会保留本地文件并记日志，不影响本地备份。
      </p>
    </v-card-text>
  </v-card>
</template>

<style scoped>
.remark-text {
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 0.8rem;
  line-height: 1.5;
  margin: 0;
}
</style>
