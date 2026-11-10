# 前端改动检查清单（PreFrontendCheckListSpec）

> 所属：FinvQuant 交付检查清单 · 存放：`Docs/CheckList/`
> 用法：本清单**只列本类特有项**。通用项（范围授权、编译测试、提交纪律、红线）不重复，
> 按 [`PreCommitCheckListSpec.md`](PreCommitCheckListSpec.md) §4 路由取；合并侧复核见 [`PreMergeCommitCheckListSpec.md`](PreMergeCommitCheckListSpec.md)。
> 适用：改动 `Web/`（页面、组件、样式、前端构建配置）。
> 展示口径：[`DictFilterSpec.md`](../DevSpec/DictFilterSpec.md)（下拉 `中文[值]`）、[`DictColumnSpec.md`](../DevSpec/DictColumnSpec.md)（表格只中文）、[`TableTextSpec.md`](../DevSpec/TableTextSpec.md)（长文本 ⓘ）、[`UiSpec.md`](../DevSpec/UiSpec.md)（状态色/开关）、[`ExplainCardSpec.md`](../DevSpec/ExplainCardSpec.md)（说明卡片）。

## 1. 命令

```bash
cd Web && npx vue-tsc -b --force && npm run build && cd ..
ls -t Web/dist/assets/index-*.js | head -1   # 记下产物哈希，验收要对得上
```

## 2. 清单

| 编号 | 检查项 | 判据 |
|------|--------|------|
| FE1 | 类型检查与构建通过 | 上面命令退出码 0，无 `any` 兜底掩盖的类型错误 |
| FE2 | 枚举展示分层正确 | 筛选控件 `中文[值]`、表格列仅中文；未登记值原样、空值 `—` |
| FE3 | 表格单元格不读下拉 `title` | 共用 items 时 item 带 `label`，单元格取 `label` |
| FE4 | 长文本不铺单元格 | 说明/备注/依据/错误详情的列是 ⓘ 悬浮，浮层有 `max-width` 与 `pre-wrap` |
| FE5 | 状态列 chip + 语义色 | 同一枚举跨页面同色；对照 `UiSpec` / `MenuSpec` 状态表 |
| FE6 | 三态齐全 | 加载中、空数据、请求失败都有明确界面，不出现空白页或 `null`/`undefined` |
| FE7 | 筛选与分页联动 | 任一筛选变更重置页码为 1；清空筛选即复位 |
| FE8 | 提交值不被翻译 | 请求体/查询参数用原始枚举值，不是中文 |
| FE9 | 错误提示统一 | 走统一 `api` 封装 + Toast；不裸 `alert`、不把后端堆栈整段弹给用户 |
| FE10 | 无调试残留与敏感信息 | 无 `console.log`；`grep -riE 'sk-\|secret\|password\|Bearer ' Web/src` 无命中 |
| FE11 | 静态说明位置正确 | 页面级说明只出现在底部唯一「说明」卡片，不夹在表格前后 |
| FE12 | 组件与工具函数复用 | 已有组件/格式化函数优先复用；新增重复实现需写明为何不复用 |
| FE13 | 菜单文档同步 | `Docs/Menu/` 对应页面的列、筛选项、操作与实现一致 |
| FE14 | 大表格性能可接受 | 千行级列表滚动/筛选无明显卡顿；逐行重计算已收敛到 `computed` |
| FE15 | 产物与页面一致 | 部署后页面加载的 `index-*.js` 哈希等于本次构建 |

## 3. 本类高频失手

| 事故 | 对应条目 |
|------|----------|
| `中文[值]` 从下拉溢出到表格列（返工两次） | FE2、FE3 |
| 时间列/字段命名漏 `dt_`、用 `-ed` 过去分词 | FE13（列头与文档一致） |
| 长文本铺满单元格，行高与列宽被最长一条决定 | FE4 |

## 4. 条目新增规则

同 [`PreCommitCheckListSpec.md`](PreCommitCheckListSpec.md) §8：来源有据（真实事故或规范条款）、判据可证伪、一条只查一件事、编号只增不改。
