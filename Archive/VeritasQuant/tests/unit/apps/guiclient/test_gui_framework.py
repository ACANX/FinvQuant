"""P2-031 GUI 框架测试：页面注册表、账户上下文与离线入口。"""

from __future__ import annotations

from veritasquant.apps.guiclient.GuiApp import GuiContext
from veritasquant.apps.guiclient.GuiServer import main as guiMain


def test_gui_context_default_pages() -> None:
    context = GuiContext(baseUrl="http://127.0.0.1:8000")
    keys = [page.key for page in context.pages]
    assert "accounts" in keys
    assert "backtests" in keys
    assert "monitoring" in keys
    assert "settings" in keys


def test_page_requires_client() -> None:
    """每个页面渲染签名必须接收 ApiClient（GUI 数据只经 API）。"""
    context = GuiContext(baseUrl="http://127.0.0.1:8000")
    import inspect

    for page in context.pages:
        params = list(inspect.signature(page.render).parameters)
        assert params[0] == "client", f"页面 {page.key} 渲染函数必须接收 client"


def test_gui_offline_validation_exit_zero() -> None:
    """离线参数校验：--help 返回 0（packaging 契约）。"""
    assert guiMain(["--help"]) == 0


def test_gui_offline_validation_no_serve() -> None:
    """不带 --serve 时只校验参数，返回 0 且无副作用。"""
    assert guiMain(["--api-url", "http://127.0.0.1:9999"]) == 0


def test_gui_invalid_args_returns_nonzero() -> None:
    assert guiMain(["--unknown-flag"]) != 0


def test_gui_context_client_lazy() -> None:
    """客户端延迟创建（导入/构造无副作用）。"""
    context = GuiContext(baseUrl="http://127.0.0.1:8000")
    assert context._client is None  # noqa: SLF001 - 验证惰性
    client = context.client
    assert client.baseUrl == "http://127.0.0.1:8000"
    assert context._client is client  # noqa: SLF001
