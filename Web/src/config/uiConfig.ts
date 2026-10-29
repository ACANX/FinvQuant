// 前端 UI 开关：导航视觉风格（界面右上角用户菜单内可直接切换）。
//
// 设计依据：Docs/Refactor/2610-012-WebMenuFourLevelRefactor.md §3.7
// - classic：保持改造前视觉（无激活条、无面包屑）
// - webcms ：参考 WebCMS 视觉（左侧 3px 主色激活条、面包屑）
//
// 切换入口：右上角用户菜单 →「WebCMS 视觉风格」开关（即时生效，无需刷新）；
// 选择结果持久化到 localStorage，下次打开沿用。
//
// 回退到旧视觉：把 uiDefaults.navVisualStyle 改为 'classic' 并清除 localStorage 同名键。

import { ref } from 'vue'

export type NavVisualStyle = 'classic' | 'webcms'

/** 导航视觉风格存储键 */
export const NAV_STYLE_STORAGE_KEY = 'finvquant.ui.navVisualStyle'
/** 菜单展开状态存储键（按系统分桶） */
export const NAV_OPEN_KEYS_STORAGE_KEY = 'finvquant.nav.openKeys.v1'
/** 各系统最后访问路径存储键（切换系统时回到原页面） */
export const NAV_LAST_PATH_STORAGE_KEY = 'finvquant.nav.lastPathBySystem.v1'

/** 默认开关（R3 已裁决：默认使用 webcms 新视觉） */
export const uiDefaults = {
  navVisualStyle: 'webcms' as NavVisualStyle,
  /** 面包屑（仅 webcms 风格） */
  navBreadcrumb: true,
}

function isNavVisualStyle(value: string | null): value is NavVisualStyle {
  return value === 'classic' || value === 'webcms'
}

/** 安全读取 localStorage（隐私模式等场景可能抛错） */
export function readStorage(key: string): string | null {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

/** 安全写入 localStorage */
export function writeStorage(key: string, value: string): void {
  try {
    localStorage.setItem(key, value)
  } catch {
    // 忽略：存储不可用时不阻断导航
  }
}

/** 从 localStorage 读取上次选择，无有效值时用默认值 */
function loadNavVisualStyle(): NavVisualStyle {
  const stored = readStorage(NAV_STYLE_STORAGE_KEY)
  return isNavVisualStyle(stored) ? stored : uiDefaults.navVisualStyle
}

/** 当前导航视觉风格（响应式：切换后界面立即更新） */
export const navVisualStyle = ref<NavVisualStyle>(loadNavVisualStyle())

/** 设置导航视觉风格并持久化 */
export function setNavVisualStyle(style: NavVisualStyle): void {
  navVisualStyle.value = style
  writeStorage(NAV_STYLE_STORAGE_KEY, style)
}

/** 在 classic / webcms 之间切换 */
export function toggleNavVisualStyle(): void {
  setNavVisualStyle(navVisualStyle.value === 'webcms' ? 'classic' : 'webcms')
}
