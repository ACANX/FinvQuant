// 登录页：用户名 + 密码，成功后写入会话并跳回来源页。
//
// 安全约定：
//   - 口令只在本页内存中停留，提交后立即清空，不写入 localStorage
//   - 失败提示统一为「用户名或密码错误」，不区分账号是否存在（由服务端保证）
//   - 已登录用户访问本页会被路由守卫重定向到仪表盘
//
// 视觉：背景为纯 CSS 绘制的星空（星点 + 星云 + 流星），不依赖外部图片资源；
// 密码框支持切换明文/密文显示。

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
// 密码明/密文切换（右侧眼睛图标）
const showPassword = ref(false)

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
    const data = await apiPost<LoginResult>('/Meta/Finv/Auth/Login', {
      uid: account,
      password: password.value,
    })
    // 口令不在前端留存
    password.value = ''
    showPassword.value = false
    setSession(data.token, data.user, data.expires_at)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/Dashboard'
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
    <!-- 星空背景：纯 CSS，aria-hidden 避免读屏器朗读装饰元素 -->
    <div class="starfield" aria-hidden="true">
      <span class="stars stars--far" />
      <span class="stars stars--mid" />
      <span class="stars stars--near" />
      <span class="nebula nebula--a" />
      <span class="nebula nebula--b" />
      <span class="nebula nebula--c" />
      <span class="shooting shooting--a" />
      <span class="shooting shooting--b" />
    </div>

    <v-card class="login-card" elevation="16" theme="dark">
      <v-card-title class="login-title">
        <v-icon icon="mdi-finance" size="28" class="mr-2 login-logo" />
        FinvQuant策略回测量化交易
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
            :type="showPassword ? 'text' : 'password'"
            prepend-inner-icon="mdi-lock-outline"
            :append-inner-icon="showPassword ? 'mdi-eye-off-outline' : 'mdi-eye-outline'"
            :aria-label="showPassword ? '隐藏密码' : '显示密码'"
            :title="showPassword ? '隐藏密码' : '显示密码'"
            autocomplete="current-password"
            variant="outlined"
            density="comfortable"
            :disabled="loading"
            @click:append-inner="showPassword = !showPassword"
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
/* ---------- 页面容器：深空底色（多层径向渐变，避免单一纯色/线性渐变） ---------- */
.login-page {
  position: relative;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  overflow: hidden;
  background-color: #05070f;
  background-image:
    radial-gradient(circle at 18% 24%, rgba(76, 60, 180, 0.55), transparent 52%),
    radial-gradient(circle at 82% 18%, rgba(120, 40, 160, 0.45), transparent 48%),
    radial-gradient(circle at 70% 84%, rgba(30, 90, 170, 0.5), transparent 55%),
    radial-gradient(circle at 12% 88%, rgba(50, 40, 130, 0.45), transparent 50%),
    radial-gradient(circle at 50% 50%, #0a1024, #04060e 78%);
}

/* ---------- 星点层 ---------- */
.starfield {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 0;
}

.stars {
  position: absolute;
  inset: 0;
  background-repeat: repeat;
  will-change: opacity;
}

/* 远景：小而密，几乎不闪 */
.stars--far {
  background-image:
    radial-gradient(1px 1px at 14px 26px, rgba(255, 255, 255, 0.7), transparent 100%),
    radial-gradient(1px 1px at 88px 64px, rgba(200, 220, 255, 0.6), transparent 100%),
    radial-gradient(1px 1px at 152px 18px, rgba(255, 255, 255, 0.5), transparent 100%),
    radial-gradient(1px 1px at 48px 132px, rgba(180, 200, 255, 0.55), transparent 100%),
    radial-gradient(1px 1px at 118px 154px, rgba(255, 255, 255, 0.45), transparent 100%);
  background-size: 190px 190px;
  animation: twinkle 9s ease-in-out infinite;
}

/* 中景 */
.stars--mid {
  background-image:
    radial-gradient(1.6px 1.6px at 62px 38px, rgba(255, 255, 255, 0.95), transparent 100%),
    radial-gradient(1.4px 1.4px at 178px 96px, rgba(210, 225, 255, 0.9), transparent 100%),
    radial-gradient(1.5px 1.5px at 120px 210px, rgba(255, 255, 255, 0.85), transparent 100%),
    radial-gradient(1.3px 1.3px at 244px 152px, rgba(190, 210, 255, 0.8), transparent 100%);
  background-size: 300px 300px;
  animation: twinkle 6.5s ease-in-out infinite 1.2s;
}

/* 近景：更大更亮，闪烁明显 */
.stars--near {
  background-image:
    radial-gradient(2.4px 2.4px at 96px 74px, #fff, transparent 100%),
    radial-gradient(2px 2px at 320px 186px, rgba(220, 235, 255, 0.95), transparent 100%),
    radial-gradient(2.2px 2.2px at 210px 330px, rgba(255, 245, 220, 0.9), transparent 100%);
  background-size: 440px 440px;
  animation: twinkle 4.8s ease-in-out infinite 0.6s;
}

@keyframes twinkle {
  0%,
  100% {
    opacity: 0.55;
  }
  50% {
    opacity: 1;
  }
}

/* ---------- 星云光斑 ---------- */
.nebula {
  position: absolute;
  border-radius: 50%;
  filter: blur(70px);
  mix-blend-mode: screen;
  opacity: 0.5;
  will-change: transform;
}

.nebula--a {
  width: 52vw;
  height: 52vw;
  left: -12vw;
  top: -14vw;
  background: radial-gradient(circle, rgba(109, 96, 255, 0.9), rgba(109, 96, 255, 0) 70%);
  animation: drift-a 28s ease-in-out infinite alternate;
}

.nebula--b {
  width: 44vw;
  height: 44vw;
  right: -10vw;
  top: 4vh;
  background: radial-gradient(circle, rgba(196, 76, 255, 0.75), rgba(196, 76, 255, 0) 70%);
  animation: drift-b 34s ease-in-out infinite alternate;
}

.nebula--c {
  width: 60vw;
  height: 40vw;
  left: 20vw;
  bottom: -18vh;
  background: radial-gradient(circle, rgba(56, 150, 248, 0.65), rgba(56, 150, 248, 0) 72%);
  animation: drift-c 40s ease-in-out infinite alternate;
}

@keyframes drift-a {
  from {
    transform: translate3d(0, 0, 0) scale(1);
  }
  to {
    transform: translate3d(6vw, 5vh, 0) scale(1.12);
  }
}

@keyframes drift-b {
  from {
    transform: translate3d(0, 0, 0) scale(1.05);
  }
  to {
    transform: translate3d(-7vw, 8vh, 0) scale(0.95);
  }
}

@keyframes drift-c {
  from {
    transform: translate3d(0, 0, 0) scale(1);
  }
  to {
    transform: translate3d(-5vw, -6vh, 0) scale(1.1);
  }
}

/* ---------- 流星 ---------- */
.shooting {
  position: absolute;
  width: 170px;
  height: 2px;
  border-radius: 2px;
  background: linear-gradient(90deg, rgba(255, 255, 255, 0), rgba(255, 255, 255, 0.95));
  filter: drop-shadow(0 0 6px rgba(170, 200, 255, 0.9));
  opacity: 0;
  transform: rotate(18deg);
}

.shooting--a {
  top: 10%;
  left: 86%;
  animation: shoot 11s linear infinite 2s;
}

.shooting--b {
  top: 34%;
  left: 96%;
  animation: shoot 17s linear infinite 8s;
}

@keyframes shoot {
  0% {
    opacity: 0;
    transform: translate3d(0, 0, 0) rotate(18deg) scaleX(1);
  }
  4% {
    opacity: 1;
  }
  26% {
    opacity: 0;
    transform: translate3d(-62vw, 26vh, 0) rotate(18deg) scaleX(0.7);
  }
  100% {
    opacity: 0;
    transform: translate3d(-62vw, 26vh, 0) rotate(18deg) scaleX(0.7);
  }
}

/* ---------- 登录卡片：毛玻璃，与星空呼应 ---------- */
.login-card {
  position: relative;
  z-index: 2;
  width: 100%;
  max-width: 420px;
  padding: 6px 6px 14px;
  background: rgba(13, 18, 42, 0.72) !important;
  backdrop-filter: blur(18px) saturate(145%);
  -webkit-backdrop-filter: blur(18px) saturate(145%);
  border: 1px solid rgba(150, 165, 255, 0.22);
  border-radius: 16px;
  box-shadow:
    0 24px 64px rgba(3, 6, 24, 0.7),
    0 0 0 1px rgba(255, 255, 255, 0.04) inset,
    0 0 42px rgba(120, 110, 255, 0.16);
}

.login-title {
  display: flex;
  align-items: center;
  font-size: 1.1rem;
  font-weight: 600;
  white-space: normal;
  line-height: 1.4;
  text-shadow: 0 0 18px rgba(150, 160, 255, 0.45);
}

.login-logo {
  color: #a5b4fc;
  filter: drop-shadow(0 0 8px rgba(129, 140, 248, 0.85));
}

.login-subtitle {
  margin-top: 6px;
  opacity: 0.75;
}

/* 密码显示/隐藏图标：可点击区域与悬停反馈 */
:deep(.v-field__append-inner) {
  cursor: pointer;
}

/* ---------- 无障碍：尊重「减少动态效果」偏好 ---------- */
@media (prefers-reduced-motion: reduce) {
  .stars,
  .nebula,
  .shooting {
    animation: none;
  }
  .shooting {
    opacity: 0;
  }
}
</style>
