"""P2-031 Streamlit GUI 框架：导航、账户上下文与 API Client 装配。

- 侧边栏导航（页面注册表）；所有数据只经 ApiClient；
- 账户上下文全局可见（TechSpec 10.1：所有账户相关界面持续显示）；
- 错误信封统一展示（code/message/error.retryable）；
- 导入本模块不启动服务；`serve()` 由 GuiServer 入口调用。
"""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass, field
from typing import Any

from veritasquant.apps.guiclient.ApiClient import ApiClient, ApiClientError
from veritasquant.apps.guiclient import Pages

# Streamlit 延迟导入（保持离线校验入口无副作用）
_ST = None


def _st() -> Any:
    """延迟加载 streamlit 单例。"""
    global _ST
    if _ST is None:
        import streamlit as st

        _ST = st
    return _ST


@dataclass(frozen=True, slots=True)
class Page:
    """一个 GUI 页面定义。"""

    key: str
    title: str
    render: Callable[[ApiClient], None]


def _page_not_implemented(client: ApiClient) -> None:  # noqa: ANN001
    st = _st()
    st.info("该页面将在后续任务中实现（P2-033）。")


# 页面注册表：P2-032 实现数据导入/策略/定投/回测操作页
_DEFAULT_PAGES: tuple[Page, ...] = (
    Page("dashboard", "仪表盘", _page_not_implemented),
    Page("data_import", "数据导入", Pages.renderImportPage),
    Page("strategies", "策略管理", Pages.renderStrategiesPage),
    Page("plans", "定投计划", Pages.renderPlansPage),
    Page("accounts", "账户管理", Pages.renderAccountsPage),
    Page("backtests", "回测中心", Pages.renderBacktestsPage),
    Page("analysis", "结果分析", Pages.renderAnalysisPage),
    Page("monitoring", "实时监控", Pages.renderMonitoringPage),
    Page("settings", "系统设置", _page_not_implemented),
)


def _renderError(error: ApiClientError) -> None:
    """统一错误信封展示。"""
    st = _st()
    retryable = "（可重试）" if error.retryable else ""
    st.error(f"错误 [{error.code}] HTTP {error.httpStatus}{retryable}: {error.message}")


def _accountContextSidebar(client: ApiClient) -> str | None:
    """侧边栏账户上下文：账户选择器 + execution_mode 徽标。"""
    st = _st()
    st.sidebar.markdown("### 账户上下文")
    try:
        accounts = client.accounts()
    except ApiClientError as error:
        _renderError(error)
        return None
    if not accounts:
        st.sidebar.info("无可用账户")
        return None
    labels = {
        account.get("account_id", "?"): (
            f"{account.get('account_id')} · {account.get('execution_mode', '?')}"
        )
        for account in accounts
    }
    selected = st.sidebar.selectbox("当前账户", list(labels.keys()), format_func=lambda key: labels[key])
    # 账户名称与 execution_mode 持续显示（TechSpec 10.1）
    st.sidebar.caption(f"模式: {labels[selected].split(' · ')[-1]}")
    return selected


def _connectionSidebar(client: ApiClient) -> None:
    st = _st()
    st.sidebar.markdown("---")
    st.sidebar.caption(f"API: {client.baseUrl}")
    try:
        info = client.version()
        st.sidebar.caption(f"API v{info.get('api_version', '?')} / catalog {info.get('catalog_version', '?')}")
    except ApiClientError:
        st.sidebar.error("API 不可达")


@dataclass(slots=True)
class GuiContext:
    """GUI 运行时上下文：页面、客户端、账户上下文。"""

    baseUrl: str
    credential: str | None = None
    pages: tuple[Page, ...] = field(default_factory=lambda: _DEFAULT_PAGES)
    _client: ApiClient | None = field(default=None, init=False, repr=False)

    @property
    def client(self) -> ApiClient:
        if self._client is None:
            self._client = ApiClient(self.baseUrl, self.credential)
        return self._client


def serve(baseUrl: str, credential: str | None = None) -> None:
    """启动 Streamlit 页面主体（由 vq-gui 入口调用）。"""
    st = _st()
    st.set_page_config(page_title="VeritasQuant", page_icon="🐝", layout="wide")
    st.title("VeritasQuant 操作台")

    context = GuiContext(baseUrl=baseUrl, credential=credential)
    client = context.client

    # 侧边栏：导航 + 账户上下文 + 连接信息
    st.sidebar.title("导航")
    pageKeys = [page.key for page in context.pages]
    selectedKey = st.sidebar.radio("页面", pageKeys, format_func=lambda key: next(p.title for p in context.pages if p.key == key))
    selectedAccount = _accountContextSidebar(client)
    _connectionSidebar(client)

    # 账户上下文注入页面（session_state 持久）
    st.session_state["account_context"] = selectedAccount

    page = next(p for p in context.pages if p.key == selectedKey)
    st.header(page.title)
    try:
        page.render(client)
    except ApiClientError as error:
        _renderError(error)
