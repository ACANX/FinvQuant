# P2-031 Streamlit 框架、导航和 API Client 证据

## 任务信息
- **PlanTaskId:** P2-031
- **标题:** 实现 Streamlit 框架、导航和 API Client
- **阶段/里程碑:** M2A 模拟盘与基金能力建设
- **状态:** IN_REVIEW（等待第五批 PR 合并后验收）

## 实现内容

### API Client（`apps/guiclient/ApiClient.py`）
- httpx 客户端：ResponseEnvelopeV1 解包、错误映射（code>=1000 → ApiClientError）
- 凭据注入（Authorization Bearer）、X-Request-Id 生成（gui_ 前缀）
- 账户/策略/标的/基金/回测/命令/报告端点全覆盖

### GUI 框架（`apps/guiclient/GuiApp.py`）
- 页面注册表（Page key/title/render）
- 侧边栏导航 + 账户上下文（account 选择器 + execution_mode 徽标）+ 连接信息
- 账户上下文注入 session_state（多账户隔离基础）
- 统一错误信封展示（code/message/retryable）

### 入口（`apps/guiclient/GuiServer.py`）
- `vq-gui`：默认离线参数校验（packaging 契约），`--serve` 启动 Streamlit

### 依赖
- streamlit 1.60.0 引入（许可证 8 项登记豁免，锁文件固定）

## 验收标准映射
| 验收标准 | 实现 | 证据 |
| --- | --- | --- |
| GUI 所有数据只经 API | ApiClient 唯一数据通道 | test_page_requires_client |
| 错误信封可统一展示 | _renderError/_showError | test_business_error_raises_with_code |
| 账户上下文始终可见 | 侧边栏账户选择器 | test_gui_context_default_pages |

## 测试结果
- `tests/unit/apps/guiclient/test_api_client.py`：9 个测试通过
- `tests/unit/apps/guiclient/test_gui_framework.py`：6 个测试通过
- packaging 16 个测试通过（含 vq-gui 入口契约）
- 真实启动：Streamlit 成功运行（timeout 冒烟）
- 全量 918 测试通过（2026-08-03，本地）

## 关键决策
- Streamlit 延迟导入（离线校验入口无副作用）
- GUI 不直连数据库/内核，只经 API（TechSpec 10.1）
