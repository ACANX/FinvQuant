// 登录页：用户名 + 密码，成功后写入会话并跳回来源页。
//
// 安全约定：
//   - 口令只在本页内存中停留，提交后立即清空，不写入 localStorage
//   - 失败提示统一为「用户名或密码错误」，不区分账号是否存在（由服务端保证）
//   - 已登录用户访问本页会被路由守卫重定向到仪表盘

<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { apiPost } from '../../api'
import { setSession } from '../../auth'

interface LoginResult {
  token: string
  expires_at: string
  ttl_seconds: number
  user: { uid: string; nick_name: string }
}

const router = useRouter()
const route = useRoute()

const uid = ref('')
const password = ref('')
const loading = ref(false)
const errorMessage = ref('')

async function onSubmit(): Promise<void> {
  if (loading.value) return
  errorMessage.value = ''
  const account = uid.value.trim()
  if (!account || !password.value) {
    errorMessage.value = '请输入用户名和密码'
    return
  }
  loading.value = true
  try {
    const data = await apiPost<LoginResult>('/Auth/Login', {
      uid: account,
      password: password.value,
    })
    // 口令不在前端留存
    password.value = ''
    setSession(data.token, data.user, data.expires_at)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/dashboard'
    await router.replace(redirect)
  } catch (e) {
    errorMessage.value = (e as Error).message || '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <v-card class="login-card" elevation="8">
      <v-card-title class="login-title">
        <v-icon icon="mdi-finance" size="28" class="mr-2" />
        FinvQuant 量化策略交易平台
      </v-card-title>
      <v-card-subtitle class="login-subtitle">请登录后使用</v-card-subtitle>

      <v-card-text>
        <v-form @submit.prevent="onSubmit">
          <v-text-field
            v-model="uid"
            label="用户名"
            prepend-inner-icon="mdi-account-outline"
            autocomplete="username"
            variant="outlined"
            density="comfortable"
            class="mb-3"
            :disabled="loading"
            autofocus
          />
          <v-text-field
            v-model="password"
            label="密码"
            type="password"
            prepend-inner-icon="mdi-lock-outline"
            autocomplete="current-password"
            variant="outlined"
            density="comfortable"
            :disabled="loading"
          />

          <v-alert
            v-if="errorMessage"
            type="error"
            variant="tonal"
            density="compact"
            class="mt-3"
            :text="errorMessage"
          />

          <v-btn
            type="submit"
            color="primary"
            block
            size="large"
            class="mt-5"
            :loading="loading"
            :disabled="!uid || !password"
          >
            登录
          </v-btn>
        </v-form>
      </v-card-text>
    </v-card>
  </div>
</template>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: linear-gradient(135deg, #1e3a5f 0%, #0d1b2a 100%);
}

.login-card {
  width: 100%;
  max-width: 420px;
  padding: 8px 8px 16px;
}

.login-title {
  display: flex;
  align-items: center;
  font-size: 1.1rem;
  font-weight: 600;
  white-space: normal;
  line-height: 1.4;
}

.login-subtitle {
  margin-top: 6px;
}
</style>
