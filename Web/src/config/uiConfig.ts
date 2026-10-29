// 前端 UI 开关：导航视觉风格与本地持久化键名。
//
// 设计依据：Docs/Refactor/2610-012-WebMenuFourLevelRefactor.md §3.7
// - classic：保持改造前视觉（无激活条、无抽屉标题、无面包屑）
// - webcms ：参考 WebCMS 视觉（左侧 3px 主色激活条、面包屑）
//
// 切换方式（优先级从高到低）：
// 1. URL 参数 ?nav=classic|webcms（命中后写入 localStorage，便于不重新构建即可对比）；
// 2. localStorage['finvquant.ui.navVisualStyle']；
// 3. 本文件 uiDefaults.navVisualStyle。
//
// 回退到旧视觉：把 uiDefaults.navVisualStyle 改为 'classic' 并清除 localStorage 同名键。

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

/** 解析当前导航视觉风格：URL 参数 > localStorage > 默认值 */
export function resolveNavVisualStyle(): NavVisualStyle {
  const fromQuery = new URLSearchParams(window.location.search).get('nav')
  if (isNavVisualStyle(fromQuery)) {
    writeStorage(NAV_STYLE_STORAGE_KEY, fromQuery)
    return fromQuery
  }
  const stored = readStorage(NAV_STYLE_STORAGE_KEY)
  if (isNavVisualStyle(stored)) return stored
  return uiDefaults.navVisualStyle
}
