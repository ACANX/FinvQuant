<script setup lang="ts">
// 证券指标页的「列选择」抽屉（需求 11251）。
//
// 132 列不可能一次全铺在屏幕上：主表只渲染用户选中的列，本抽屉按业务分组勾选，
// 并提供预设列方案（概览 / 行情 / 估值与股息 / 基金 / 期货与标的 / 时间与来源 / 全部）。
// 勾选即生效（父组件实时重查），分组支持整组全选 / 清空；勾选结果由父组件持久化到 localStorage。
import { computed, ref } from 'vue'

import {
  INDICATOR_GROUPS,
  INDICATOR_KEY_COLUMNS,
  INDICATOR_PRESETS,
  indicatorColumnLabel,
  indicatorGroupOf,
} from '../../../../../config/indicatorColumns'
import type { IndicatorColumn } from '../../../../../config/indicatorColumns'

const props = defineProps<{
  /** 抽屉开关（v-model） */
  modelValue: boolean
  /** 后端返回的全部列元数据 */
  columns: IndicatorColumn[]
  /** 当前选中的列（v-model:selected） */
  selected: string[]
  /** 当前命中的预设 key（自定义列集时为空串） */
  preset: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'update:selected', value: string[]): void
  (e: 'update:preset', value: string): void
}>()

const keyword = ref('')

/**
 * 后端列 → 前端元数据。
 *
 * 列表文字用**中文短名**（`indicatorColumns.ts` 的 label），不用后端 `title`——
 * 后者取自表列注释，常常是整句口径说明，铺在 132 个勾选项里会成灾。
 * 完整口径只在悬浮（`hint`）与行详情里出现。
 */
interface PickItem {
  name: string
  title: string
  /** 完整口径（后端表列注释），仅用于悬浮 */
  hint?: string
  groupKey: string
  groupTitle: string
}

const items = computed<PickItem[]>(() => {
  const groupTitles = new Map(INDICATOR_GROUPS.map((group) => [group.key, group.title]))
  return props.columns.map((column) => {
    const groupKey = indicatorGroupOf(column.name)
    const title = indicatorColumnLabel(column)
    return {
      name: column.name,
      title,
      hint: column.title && column.title !== title ? column.title : '',
      groupKey,
      groupTitle: groupTitles.get(groupKey) ?? '其它（未登记）',
    }
  })
})

/** 按关键字过滤后的列（标题 / 列名都参与匹配） */
const filtered = computed<PickItem[]>(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return items.value
  return items.value.filter(
    (item) => item.title.toLowerCase().includes(kw) || item.name.toLowerCase().includes(kw),
  )
})

/** 分组 → 过滤后的条目（空分组不展示） */
const groups = computed(() => {
  const order = INDICATOR_GROUPS.map((group) => group.key)
  const buckets = new Map<string, PickItem[]>()
  for (const item of filtered.value) {
    const list = buckets.get(item.groupKey) ?? []
    list.push(item)
    buckets.set(item.groupKey, list)
  }
  const titles = new Map(INDICATOR_GROUPS.map((group) => [group.key, group.title]))
  return order
    .filter((key) => (buckets.get(key)?.length ?? 0) > 0)
    .map((key) => ({ key, title: titles.get(key) ?? key, items: buckets.get(key) ?? [] }))
})

const selectedSet = computed(() => new Set(props.selected))

function isChecked(name: string): boolean {
  return selectedSet.value.has(name)
}

/** 身份列不允许取消（行必须有标识） */
function isLocked(name: string): boolean {
  return INDICATOR_KEY_COLUMNS.includes(name)
}

/** 勾选 / 取消一列：保持既有列顺序（取消后从末尾移除，新勾选的追加在后） */
function toggle(name: string, value: boolean | null): void {
  if (isLocked(name)) return
  const next = props.selected.filter((item) => item !== name)
  if (value) next.push(name)
  commit(next)
}

/** 分组全选 / 清空 */
function toggleGroup(list: PickItem[], value: boolean): void {
  const names = list.map((item) => item.name).filter((name) => !isLocked(name))
  if (value) {
    const next = [...props.selected]
    const exists = new Set(next)
    for (const name of names) if (!exists.has(name)) next.push(name)
    commit(next)
    return
  }
  commit(props.selected.filter((name) => !names.includes(name)))
}

/** 组内已勾选数量 / 组内总数 */
function groupCheckedCount(list: PickItem[]): number {
  return list.filter((item) => isChecked(item.name)).length
}

function applyPreset(key: string): void {
  const preset = INDICATOR_PRESETS.find((item) => item.key === key)
  if (!preset) return
  // 只保留后端确实存在的列（后端删列后旧方案不至于报 400）
  const known = new Set(props.columns.map((column) => column.name))
  const next = preset.columns.filter((name) => known.has(name))
  const keyColumns = INDICATOR_KEY_COLUMNS.filter((name) => known.has(name))
  for (const name of keyColumns) if (!next.includes(name)) next.unshift(name)
  emit('update:preset', key)
  commit(next, true)
}

/** 提交列集：去重、补身份列；presetHandled 为 true 时保留调用方设置的预设 key */
function commit(next: string[], presetHandled = false): void {
  const known = new Set(props.columns.map((column) => column.name))
  const seen = new Set<string>()
  const list: string[] = []
  for (const name of [...INDICATOR_KEY_COLUMNS, ...next]) {
    if (!known.has(name) || seen.has(name)) continue
    seen.add(name)
    list.push(name)
  }
  if (!presetHandled) emit('update:preset', '')
  emit('update:selected', list)
}
</script>

<template>
  <v-navigation-drawer
    :model-value="modelValue"
    location="right"
    temporary
    width="440"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <div class="d-flex flex-column h-100">
      <div class="pa-3 pb-2">
        <div class="d-flex align-center mb-2">
          <v-icon icon="mdi-view-column-outline" class="mr-2" />
          <span class="text-subtitle-1">选择列</span>
          <v-spacer />
          <v-chip size="small" variant="tonal">{{ selected.length }} / {{ columns.length }}</v-chip>
        </div>
        <v-text-field
          v-model="keyword"
          label="搜索列（中文名 / 列名）"
          density="compact"
          hide-details
          clearable
          prepend-inner-icon="mdi-magnify"
        />
      </div>

      <div class="px-3 pb-2">
        <div class="text-caption text-medium-emphasis mb-1">预设列方案</div>
        <div class="d-flex flex-wrap ga-1">
          <v-chip
            v-for="item in INDICATOR_PRESETS"
            :key="item.key"
            :color="preset === item.key ? 'primary' : undefined"
            :variant="preset === item.key ? 'flat' : 'tonal'"
            size="small"
            @click="applyPreset(item.key)"
          >
            {{ item.title }}
          </v-chip>
        </div>
      </div>

      <v-divider />

      <div class="flex-grow-1 overflow-y-auto pa-3">
        <div v-for="group in groups" :key="group.key" class="mb-3">
          <div class="d-flex align-center ga-2 mb-1">
            <span class="text-caption font-weight-medium">{{ group.title }}</span>
            <span class="text-caption text-medium-emphasis">
              {{ groupCheckedCount(group.items) }} / {{ group.items.length }}
            </span>
            <v-spacer />
            <v-btn size="x-small" variant="text" @click="toggleGroup(group.items, true)">全选</v-btn>
            <v-btn size="x-small" variant="text" @click="toggleGroup(group.items, false)">清空</v-btn>
          </div>
          <div class="d-flex flex-column">
            <v-checkbox
              v-for="item in group.items"
              :key="item.name"
              :model-value="isChecked(item.name)"
              :disabled="isLocked(item.name)"
              density="compact"
              hide-details
              class="finv-indicator-pick"
              @update:model-value="toggle(item.name, $event)"
            >
              <template #label>
                <span :title="item.hint">{{ item.title }}</span>
                <code class="ml-2 text-caption text-medium-emphasis">{{ item.name }}</code>
              </template>
            </v-checkbox>
          </div>
        </div>
        <div v-if="!groups.length" class="text-caption text-medium-emphasis py-6 text-center">
          没有匹配的列
        </div>
      </div>

      <v-divider />
      <div class="pa-3 d-flex ga-2">
        <v-btn variant="tonal" @click="emit('update:modelValue', false)">完成</v-btn>
        <v-btn variant="text" @click="applyPreset('overview')">恢复默认（概览）</v-btn>
      </div>
    </div>
  </v-navigation-drawer>
</template>

<style scoped>
/* 132 列的勾选项要紧凑：compact 密度下再压一下行高 */
.finv-indicator-pick :deep(.v-selection-control) {
  min-height: 28px;
}

.finv-indicator-pick :deep(.v-label) {
  opacity: 1;
  font-size: 0.8125rem;
}
</style>
