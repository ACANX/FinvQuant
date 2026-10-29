<script setup lang="ts">
// 递归菜单节点：分组渲染为可折叠项，叶子渲染为路由链接。
//
// 展开键 = 父链 key 拼接（如 finvquant.data-collection.quote-archive），
// 由 NavMenuTree 统一维护与持久化，本组件只负责渲染与事件上抛。
import { computed } from 'vue'

// 递归组件需显式自导入：本项目未启用自动组件注册，模板里的自定义标签必须可静态解析
import NavMenuTreeNode from './NavMenuTreeNode.vue'
import { isMenuLeaf } from '../../config/menuConfig'
import type { MenuNode } from '../../config/menuConfig'

const props = defineProps<{
  node: MenuNode
  /** 0 = 当前系统的第一层子菜单（全局第二级） */
  depth: number
  /** 父链 key；根层为系统 key */
  parentKey: string
  /** 已展开的分组键集合 */
  openKeys: Set<string>
  /** 当前路由路径 */
  activePath: string
}>()

const emit = defineEmits<{
  toggle: [key: string]
  navigate: []
}>()

const nodeKey = computed(() => `${props.parentKey}.${props.node.key}`)
const leaf = computed(() => isMenuLeaf(props.node))
const expanded = computed(() => props.openKeys.has(nodeKey.value))
const isActive = computed(() => Boolean(props.node.path) && props.activePath === props.node.path)
const containsActive = computed(() => hasActive(props.node, props.activePath))

/** 子容器 id（供 aria-controls 使用） */
const childrenId = computed(() => `finv-nav-${nodeKey.value.replace(/\./g, '-')}`)
/** 每级缩进：二级 12px 起，每级 +16px */
const indent = computed(() => 12 + props.depth * 16)
/** 三/四级字号略小，缓解长标题换行 */
const fontSize = computed(() => (props.depth >= 1 ? '13px' : '14px'))

/** 子孙中是否包含当前激活叶子（用于祖先高亮） */
function hasActive(node: MenuNode, path: string): boolean {
  if (isMenuLeaf(node)) return node.path === path
  return (node.children ?? []).some((child) => hasActive(child, path))
}
</script>

<template>
  <div class="finv-nav-node">
    <!-- 分组节点：点击展开/折叠 -->
    <template v-if="!leaf">
      <v-list-item
        class="finv-nav-item"
        :class="{ 'finv-nav-item--ancestor': containsActive }"
        :style="{ '--finv-nav-indent': `${indent}px`, '--finv-nav-font-size': fontSize }"
        :prepend-icon="node.icon"
        :title="node.title"
        :subtitle="node.subtitle"
        role="button"
        :aria-expanded="expanded"
        :aria-controls="childrenId"
        @click="emit('toggle', nodeKey)"
      >
        <template #append>
          <v-icon :icon="expanded ? 'mdi-chevron-down' : 'mdi-chevron-right'" size="small" />
        </template>
      </v-list-item>

      <div v-show="expanded" :id="childrenId">
        <NavMenuTreeNode
          v-for="child in node.children"
          :key="child.key"
          :node="child"
          :depth="depth + 1"
          :parent-key="nodeKey"
          :open-keys="openKeys"
          :active-path="activePath"
          @toggle="emit('toggle', $event)"
          @navigate="emit('navigate')"
        />
      </div>
    </template>

    <!-- 叶子节点：路由链接 -->
    <v-list-item
      v-else
      class="finv-nav-item finv-nav-leaf"
      :class="{
        'finv-nav-leaf--active': isActive,
        'finv-nav-item--ancestor': containsActive && !isActive,
      }"
      :style="{ '--finv-nav-indent': `${indent}px`, '--finv-nav-font-size': fontSize }"
      :prepend-icon="node.icon"
      :title="node.title"
      :subtitle="node.subtitle"
      :to="node.path"
      :active="isActive"
      @click="emit('navigate')"
    />
  </div>
</template>

<style>
/* 非 scoped：类名统一 finv-nav- 前缀，避免与业务页面样式互相影响 */
.finv-nav-item {
  padding-inline-start: var(--finv-nav-indent, 16px) !important;
}

.finv-nav-item .v-list-item-title {
  font-size: var(--finv-nav-font-size, 14px);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.finv-nav-item .v-list-item-subtitle {
  font-size: 12px;
}

/* 祖先高亮：子孙含激活叶子 */
.finv-nav-item--ancestor .v-list-item-title {
  color: rgb(var(--v-theme-primary));
}

/* WebCMS 视觉：激活叶子左侧 3px 主色条（用内阴影避免布局位移） */
.finv-nav-leaf--active.v-list-item--active {
  box-shadow: inset 3px 0 0 0 rgb(var(--v-theme-primary));
}

.finv-nav-leaf--active.v-list-item--active .v-list-item-title {
  font-weight: 600;
}

/* 分组激活箭头与图标保持默认色，仅标题高亮 */
.finv-nav-item--ancestor .v-list-item__prepend > .v-icon {
  color: rgb(var(--v-theme-primary));
  opacity: 0.9;
}
</style>
