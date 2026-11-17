<script setup lang="ts">
import { onMounted, ref } from 'vue'

const serverStatus = ref('检测中...')
const serverInfo = ref<string>('')

onMounted(async () => {
  try {
    const response = await fetch('/API/V1/Health/Live')
    const data = await response.json()
    serverStatus.value = '服务端已连接'
    serverInfo.value = `server=${data.server ?? '?'} status=${data.status ?? '?'}`
  } catch {
    serverStatus.value = '服务端未连接'
    serverInfo.value = '请确认服务端已在 16001 端口启动'
  }
})
</script>

<template>
  <v-card max-width="640" class="mx-auto">
    <v-card-title>前端服务已就绪</v-card-title>
    <v-card-text>
      <v-list>
        <v-list-item>
          <template #prepend>
            <v-icon :icon="serverStatus === '服务端已连接' ? 'mdi-check-circle' : 'mdi-alert-circle'" />
          </template>
          <v-list-item-title>{{ serverStatus }}</v-list-item-title>
          <v-list-item-subtitle>{{ serverInfo }}</v-list-item-subtitle>
        </v-list-item>
      </v-list>
      <v-divider class="my-4" />
      <p class="text-body-2">
        技术栈：Vue {{ '3' }} + Vite {{ '8' }} + Vuetify {{ '4' }}（端口 16002）
      </p>
    </v-card-text>
  </v-card>
</template>
