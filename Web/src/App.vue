<script setup lang="ts">
import { ref } from 'vue'
import { useRoute } from 'vue-router'

// 多级菜单定义（支持两级分组：顶级 → 二级 → 三级菜单项）
// 每个叶子菜单项对应独立 URL 路由路径（见 Web/src/router.ts）
interface MenuLeaf {
  key: string
  title: string
  icon: string
  path: string
}

interface MenuGroup {
  key: string
  title: string
  icon: string
  children: (MenuLeaf | MenuGroup)[]
}

const menuItems: (MenuLeaf | MenuGroup)[] = [
  { key: 'dashboard', title: '仪表盘', icon: 'mdi-view-dashboard', path: '/dashboard' },
  {
    key: 'history-quote',
    title: '历史行情',
    icon: 'mdi-chart-box',
    children: [
      { key: 'quote-query', title: '历史行情查询', icon: 'mdi-chart-line', path: '/quote/query' },
    ],
  },
  {
    key: 'metadata',
    title: '元数据管理',
    icon: 'mdi-database-cog',
    children: [
      {
        key: 'meta-maintenance',
        title: '业务元数据维护',
        icon: 'mdi-database-search',
        children: [
          { key: 'meta-exchange', title: '交易所信息维护', icon: 'mdi-office-building', path: '/meta/exchange' },
          { key: 'meta-market', title: '交易所下设市场信息维护', icon: 'mdi-chart-areaspline', path: '/meta/market' },
          { key: 'meta-security', title: '规范证券信息维护', icon: 'mdi-tag-multiple', path: '/meta/security' },
          { key: 'meta-import', title: '历史行情数据导入', icon: 'mdi-database-import', path: '/meta/import' },
        ],
      },
    ],
  },
]

const route = useRoute()
const drawer = ref(true)

// 当前激活菜单项 key（叶子路径匹配）
function isLeaf(item: MenuLeaf | MenuGroup): item is MenuLeaf {
  return 'path' in item
}

function isActive(item: MenuLeaf | MenuGroup): boolean {
  if (isLeaf(item)) return route.path === item.path
  return item.children.some((child) => isActive(child))
}

// 一级菜单展开状态：当前路由所在的分组保持展开
function isGroupOpen(item: MenuGroup): boolean {
  return item.children.some((child) => (isLeaf(child) ? isActive(child) : isGroupOpen(child)))
}
</script>

<template>
  <v-app>
    <v-app-bar color="primary" density="comfortable">
      <v-app-bar-title>
        <v-icon icon="mdi-finance" class="mr-2" />
        FinvQuant 量化策略交易平台
      </v-app-bar-title>
    </v-app-bar>

    <v-navigation-drawer v-model="drawer">
      <v-list density="comfortable" nav>
        <template v-for="item in menuItems" :key="item.key">
          <!-- 叶子菜单 -->
          <v-list-item
            v-if="isLeaf(item)"
            :prepend-icon="item.icon"
            :title="item.title"
            :active="isActive(item)"
            :to="item.path"
          />

          <!-- 分组菜单（可再含二级分组） -->
          <v-list-group v-else :value="item.key" :open="isGroupOpen(item)">
            <template #activator="{ props }">
              <v-list-item
                v-bind="props"
                :prepend-icon="item.icon"
                :title="item.title"
                :active="isActive(item)"
              />
            </template>

            <template v-for="child in item.children" :key="child.key">
              <!-- 二级叶子 -->
              <v-list-item
                v-if="isLeaf(child)"
                :prepend-icon="child.icon"
                :title="child.title"
                :active="isActive(child)"
                :to="child.path"
                class="pl-6"
              />

              <!-- 二级分组（三级叶子） -->
              <v-list-group v-else :value="child.key" subgroup :open="isGroupOpen(child)">
                <template #activator="{ props }">
                  <v-list-item
                    v-bind="props"
                    :prepend-icon="child.icon"
                    :title="child.title"
                    :active="isActive(child)"
                  />
                </template>

                <v-list-item
                  v-for="leaf in child.children.filter(isLeaf)"
                  :key="leaf.key"
                  :prepend-icon="leaf.icon"
                  :title="leaf.title"
                  :active="isActive(leaf)"
                  :to="leaf.path"
                  class="pl-12"
                />
              </v-list-group>
            </template>
          </v-list-group>
        </template>
      </v-list>
    </v-navigation-drawer>

    <v-main>
      <v-container fluid class="mt-4">
        <router-view />
      </v-container>
    </v-main>
  </v-app>
</template>
