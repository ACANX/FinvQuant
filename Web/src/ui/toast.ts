// 全局轻提示（toast）状态：操作结果统一悬浮展示，若干秒后自动消失。
//
// 为什么集中管理：此前各页面用内嵌 v-alert 承载「保存成功」这类一次性消息，
// 消息会永久占据页面一块固定位置、需要用户手动忽略，还会把内容顶下去。
// 规范见 Docs/DevSpec/UiSpec.md：一次性操作反馈一律走 showToast，
// v-alert 只用于需要长期可见的状态（运行中、磁盘水位、目录不可用等）。
import { computed, reactive } from 'vue'

/** 提示类型：决定配色与图标 */
export type ToastType = 'success' | 'error' | 'info' | 'warning'

/** 默认展示时长（毫秒）：5 秒后自动消失 */
export const TOAST_DEFAULT_TIMEOUT = 5000

export interface ToastItem {
  id: number
  text: string
  type: ToastType
  timeout: number
}

const state = reactive<{ queue: ToastItem[] }>({ queue: [] })
let seq = 0

/** 当前正在展示的一条（队列为空时为 undefined） */
export const currentToast = computed<ToastItem | undefined>(() => state.queue[0])

/** 队列长度（含正在展示的一条），用于单测与调试 */
export const toastQueueSize = computed<number>(() => state.queue.length)

/**
 * 展示一条轻提示。
 *
 * @param text 提示文案（纯文本，不解析 HTML）
 * @param type 类型，默认 success
 * @param timeout 展示时长（毫秒），默认 5000；0 表示不自动消失（需手动关闭）
 */
export function showToast(text: string, type: ToastType = 'success', timeout = TOAST_DEFAULT_TIMEOUT): number {
  const trimmed = (text ?? '').toString().trim()
  if (!trimmed) return 0
  const id = ++seq
  state.queue.push({ id, text: trimmed, type, timeout: timeout < 0 ? 0 : timeout })
  return id
}

/** 关闭指定提示（不传则关闭当前正在展示的一条） */
export function dismissToast(id?: number): void {
  if (state.queue.length === 0) return
  if (id === undefined || state.queue[0].id === id) {
    state.queue.shift()
    return
  }
  const index = state.queue.findIndex((item) => item.id === id)
  if (index > 0) state.queue.splice(index, 1)
}

/** 清空所有排队提示（离开页面/登出时可调用） */
export function clearToasts(): void {
  state.queue.splice(0, state.queue.length)
}
