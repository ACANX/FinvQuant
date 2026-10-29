// 前端 UI 配置：导航视觉固定为 WebCMS 风格（激活条常开），不再提供风格开关。
//
// 设计依据：Docs/Refactor/2610-012-WebMenuFourLevelRefactor.md §3.7
// 视觉固定项：当前菜单项左侧 3px 主色激活条（`finv-nav-leaf--active`）+ 祖先标题主色。
//
// 说明：早期版本曾提供 classic / webcms 开关与 `?nav=` 参数，2026-10-29 按评审意见
// 固定为 WebCMS 视觉并移除入口；localStorage 旧键 `finvquant.ui.navVisualStyle` 不再读取，
// 残留值无影响（可在浏览器 Application 面板手动清除）。

/** 菜单展开状态存储键（按系统分桶） */
export const NAV_OPEN_KEYS_STORAGE_KEY = 'finvquant.nav.openKeys.v1'
/** 各系统最后访问路径存储键（切换系统时回到原页面） */
export const NAV_LAST_PATH_STORAGE_KEY = 'finvquant.nav.lastPathBySystem.v1'

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
