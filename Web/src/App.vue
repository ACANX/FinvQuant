<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useDisplay } from 'vuetify'
import { useRoute, useRouter } from 'vue-router'

import { apiPost } from './api'
import { authState, clearSession } from './auth'
import NavBreadcrumb from './components/menu/NavBreadcrumb.vue'
import NavMenuTree from './components/menu/NavMenuTree.vue'
import SystemSwitcher from './components/menu/SystemSwitcher.vue'
import { findMenuTrail, menuSystems, resolveSystem } from './config/menuConfig'
import {
  NAV_LAST_PATH_STORAGE_KEY,
  readStorage,
  resolveNavVisualStyle,
  uiDefaults,
  writeStorage,
} from './config/uiConfig'
import ChangePasswordDialog from './views/Common/ChangePasswordDialog.vue'

// 导航结构：一级 = 系统（顶栏切换器），左侧抽屉渲染当前系统的二~四级菜单。
// 菜单数据见 Web/src/config/menuConfig.ts；视觉开关见 Web/src/config/uiConfig.ts。

const route = useRoute()
const router = useRouter()
const display = useDisplay()
const drawer = ref(true)

// 当前系统（按路由前缀判定；未命中回落到金融投资）
const currentSystem = computed(() => resolveSystem(route.path))
// 面包屑节点链（系统 → 二/三/四级）
const currentTrail = computed(() => findMenuTrail(route.path))
// 视觉风格：classic 保持改造前样式；webcms 追加激活条、抽屉标题、面包屑
const navVisualStyle = ref(resolveNavVisualStyle())
const showBreadcrumb = computed(() => navVisualStyle.value === 'webcms' && uiDefaults.navBreadcrumb)

// 公开页（登录）不渲染导航框架，避免未登录时先闪一下侧边栏
const isPublicRoute = computed(() => Boolean(route.meta.public))

// 修改密码对话框（需求 10763）
const changePasswordOpen = ref(false)
// 顶部提示条：改密结果与吊销的其它会话数
const toast = ref(false)
const toastText = ref('')

/** 读取各系统上次访问的路由（切换系统时回到原页面） */
function readLastPaths(): Record<string, string> {
  const raw = readStorage(NAV_LAST_PATH_STORAGE_KEY)
  if (!raw) return {}
  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>
    const result: Record<string, string> = {}
    for (const [key, value] of Object.entries(parsed ?? {})) {
      if (typeof value === 'string') result[key] = value
    }
    return result
  } catch {
    return {}
  }
}

/** 记录当前系统最后访问的路由 */
function rememberCurrentPath(): void {
  if (route.meta.public) return
  const paths = readLastPaths()
  paths[currentSystem.value.key] = route.path
  writeStorage(NAV_LAST_PATH_STORAGE_KEY, JSON.stringify(paths))
}

/** 切换一级系统：优先回到该系统上次访问的路由，否则落到系统落地路由 */
function onSelectSystem(key: string): void {
  const target = menuSystems.find((system) => system.key === key)
  if (!target || target.key === currentSystem.value.key) return
  const remembered = readLastPaths()[key]
  const dest = remembered && resolveSystem(remembered).key === key ? remembered : target.path
  void router.push(dest)
}

/** 叶子菜单点击：移动端收起抽屉（桌面端保持展开） */
function onMenuNavigate(): void {
  if (display.mobile.value) drawer.value = false
}

/** 改密成功回调：当前会话保留，仅提示结果 */
function onChangePasswordDone(revokedSessions: number): void {
  toastText.value =
    revokedSessions > 0
      ? `密码已修改，已退出其它 ${revokedSessions} 个设备的登录`
      : '密码已修改'
  toast.value = true
}

/** 退出登录：先尽力吊销服务端会话，再清本地状态并回登录页 */
async function onLogout(): Promise<void> {
  try {
    await apiPost('/Meta/Finv/Auth/Logout', {})
  } catch {
    // 服务端会话可能已过期；本地状态必须清理，因此忽略该错误
  }
  clearSession()
  await router.replace('/Login')
}

// 记录当前系统最后访问路径（含首次进入）
watch(() => route.path, rememberCurrentPath, { immediate: true })

// 路由切换后收起抽屉（移动端/小屏临时抽屉），桌面端保持折叠状态用户可自行展开
watch(
  () => route.fullPath,
  () => {
    if (display.mobile.value) drawer.value = false
  },
)
</script>

<template>
  <v-app>
    <!-- 公开页（登录）：全屏渲染，不显示导航框架 -->
    <router-view v-if="isPublicRoute" />

    <template v-else>
    <v-app-bar color="primary" density="comfortable">
      <v-app-bar-nav-icon @click="drawer = !drawer" />
      <v-app-bar-title class="finv-app-title">
        <v-icon icon="mdi-finance" class="mr-2" />
        FinvQuant 量化策略交易平台
      </v-app-bar-title>

      <!-- 一级系统切换（金融投资 / 内容管理） -->
      <SystemSwitcher
        :systems="menuSystems"
        :current-key="currentSystem.key"
        @select="onSelectSystem"
      />

      <!-- 占位：把登录用户按钮推回顶栏最右侧（与改动前一致），不与一级菜单切换器撮堆 -->
      <v-spacer />

      <!-- 当前登录用户与退出入口 -->
      <v-menu location="bottom end">
        <template #activator="{ props }">
          <v-btn
            v-bind="props"
            variant="text"
            prepend-icon="mdi-account-circle-outline"
            :text="authState.user?.nick_name || authState.user?.uid || '未登录'"
          />
        </template>
        <v-list density="compact" min-width="180">
          <v-list-item
            :title="authState.user?.nick_name || '-'"
            :subtitle="authState.user?.uid || ''"
            prepend-icon="mdi-account"
          />
          <v-divider />
          <v-list-item
            prepend-icon="mdi-lock-reset"
            title="修改密码"
            @click="changePasswordOpen = true"
          />
          <v-list-item prepend-icon="mdi-logout" title="退出登录" @click="onLogout" />
        </v-list>
      </v-menu>
    </v-app-bar>

    <v-navigation-drawer
      v-model="drawer"
      :temporary="display.mobile.value"
      :permanent="!display.mobile.value"
      :width="300"
    >
      <!-- 当前系统的二~四级菜单树 -->
      <NavMenuTree
        :system="currentSystem"
        :active-path="route.path"
        :visual="navVisualStyle"
        @navigate="onMenuNavigate"
      />
    </v-navigation-drawer>

    <v-main>
      <!-- 面包屑（仅 webcms 视觉） -->
      <NavBreadcrumb v-if="showBreadcrumb" :trail="currentTrail" />
      <v-container fluid class="mt-4">
        <router-view />
      </v-container>
    </v-main>

    <ChangePasswordDialog v-model="changePasswordOpen" @changed="onChangePasswordDone" />
    <v-snackbar v-model="toast" :timeout="4000" color="success" location="top">
      {{ toastText }}
    </v-snackbar>
    </template>
  </v-app>
</template>

<style>
/* 平台名完整展示：不参与 flex 拉伸、不限宽、不省略（宽度按内容自适应） */
.finv-app-title {
  flex: 0 0 auto;
  max-width: none;
  overflow: visible;
  text-overflow: clip;
}

/* 窄屏（平板竖屏/手机）才限宽，避免顶栏溢出把切换器与用户菜单挤掉 */
@media (max-width: 900px) {
  .finv-app-title {
    max-width: 200px;
  }
}
</style>
