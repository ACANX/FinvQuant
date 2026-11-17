<script setup lang="ts">
// 配置管理（占位页）：后续承载系统级/平台级配置项
// 环境管理、模板管理已拆分为独立菜单（EnvironmentView / TemplateView）
</script>

<template>
  <v-container fluid>
    <v-card>
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-cog-outline" class="mr-2" color="primary" />
        配置管理
      </v-card-title>
      <v-card-text class="text-center py-12 text-medium-emphasis">
        <v-icon icon="mdi-cog-outline" size="56" class="mb-3" />
        <p class="text-h6">配置管理（规划中）</p>
        <p class="text-body-2">
          系统级 / 平台级配置项将在此集中管理。当前环境管理、模板管理已作为独立菜单提供，
          请通过左侧「环境管理」「模板管理」进入对应功能。
        </p>
      </v-card-text>
    </v-card>
  </v-container>
</template>
