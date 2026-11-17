"""P2-024 API 版本信息默认提供者与内置 readiness 探针。

版本号从包元数据读取；导入模块不得连接外部服务或启动线程。
"""

from __future__ import annotations

from importlib import metadata

from veritasquant.application.ApiApp import ApiVersionProvider
from veritasquant.application.ApiErrors import ApiErrorCatalog


class PackagedApiVersionProvider(ApiVersionProvider):
    """从已安装 wheel 元数据读取 API/服务版本。"""

    def __init__(self, distribution: str = "veritasquant", apiVersion: str = "v1") -> None:
        self._distribution = distribution
        self._apiVersion = apiVersion

    @property
    def apiVersion(self) -> str:
        return self._apiVersion

    @property
    def catalogVersion(self) -> str:
        try:
            return metadata.version(self._distribution)
        except metadata.PackageNotFoundError:
            return "0.0.0"


class ErrorCatalogProbe:
    """readiness 探针：错误目录已加载且含必需错误码。"""

    def __init__(self, catalog: ApiErrorCatalog) -> None:
        self._catalog = catalog

    def check(self) -> tuple[bool, str]:
        try:
            self._catalog.getError(2006)
            self._catalog.getError(2005)
            return True, "错误目录已加载"
        except Exception:  # noqa: BLE001 - 探针必须捕获全部失败
            return False, "错误目录未就绪"
