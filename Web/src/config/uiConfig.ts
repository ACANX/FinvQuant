// 前端 UI 配置：导航视觉固定为 WebCMS 风格（激活条常开），不再提供风格开关。
//
// 设计依据：Docs/Refactor/2610-012-WebMenuFourLevelRefactor.md §3.7
// 视觉固定项：当前菜单项左侧 3px 主色激活条（`finv-nav-leaf--active`）+ 祖先标题主色。
//
// 说明：早期版本曾提供 classic / webcms 开关与 `?nav=` 参数，2026-10-29 按评审意见
// 固定为 WebCMS 视觉并移除入口；localStorage 旧键 `finvquant.ui.navVisualStyle` 不再读取，
// 残留值无影响（可在浏览器 Application 面板手动清除）。

/** 主题（深色/浅色）存储键：保存在用户浏览器本地，暂不做服务端持久化 */
export const NAV_THEME_STORAGE_KEY = 'finvquant.ui.theme'
/** 菜单展开状态存储键（按系统分桶） */
export const NAV_OPEN_KEYS_STORAGE_KEY = 'finvquant.nav.openKeys.v1'
/** 各系统最后访问路径存储键（切换系统时回到原页面） */
export const NAV_LAST_PATH_STORAGE_KEY = 'finvquant.nav.lastPathBySystem.v1'

export type NavTheme = 'dark' | 'light'

/**
 * 初始主题：localStorage 用户选择 > 浏览器系统偏好 > 深色（项目默认）。
 * 仅在创建 Vuetify 实例时调用一次，之后由用户菜单开关接管。
 */
export function resolveInitialTheme(): NavTheme {
  const stored = readStorage(NAV_THEME_STORAGE_KEY)
  if (stored === 'dark' || stored === 'light') return stored
  try {
    if (window.matchMedia?.('(prefers-color-scheme: light)').matches) return 'light'
  } catch {
    // 忽略：环境不支持 matchMedia 时回落到默认深色
  }
  return 'dark'
}

/** 记录用户选择的主题（本地持久化） */
export function setNavTheme(theme: NavTheme): void {
  writeStorage(NAV_THEME_STORAGE_KEY, theme)
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
