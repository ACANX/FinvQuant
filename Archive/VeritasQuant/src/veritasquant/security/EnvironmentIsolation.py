"""P5-002 实盘环境、账户组、网络和数据物理/逻辑隔离。

对齐 TechSpec 13 阶段 5 平台 gate：
- LIVE 不能与非 LIVE 混组；
- 测试凭据不能访问实盘；
- 跨环境命令被拒绝。

- `EnvironmentKind`：BACKTEST/PAPER/SIMULATION/LIVE；
- `AccountGroupEnvironmentV1`：账户组的环境归属；
- `EnvironmentIsolationPolicyV1`：混组检查 + 凭据环境匹配 + 跨环境命令门禁。
"""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum


class IsolationError(ValueError):
    """环境隔离不满足契约时抛出。"""


class EnvironmentKind(StrEnum):
    Backtest = "BACKTEST"
    Paper = "PAPER"
    Simulation = "SIMULATION"
    Live = "LIVE"


@dataclass(frozen=True, slots=True)
class AccountGroupEnvironmentV1:
    """账户组的环境归属（一组只能属于一个环境）。"""

    accountGroupId: str
    environment: EnvironmentKind

    def __post_init__(self) -> None:
        if not self.accountGroupId:
            raise IsolationError("账户组 ID 不能为空")


@dataclass(frozen=True, slots=True)
class CredentialEnvironmentV1:
    """凭据的环境绑定（测试凭据不能访问实盘）。"""

    credentialId: str
    environment: EnvironmentKind

    def __post_init__(self) -> None:
        if not self.credentialId:
            raise IsolationError("凭据 ID 不能为空")


class EnvironmentIsolationPolicyV1:
    """环境隔离策略：混组拒绝 + 凭据匹配 + 跨环境命令拒绝。"""

    def __init__(self) -> None:
        self._groups: dict[str, AccountGroupEnvironmentV1] = {}
        self._credentials: dict[str, CredentialEnvironmentV1] = {}

    def registerGroup(self, group: AccountGroupEnvironmentV1) -> None:
        if group.accountGroupId in self._groups:
            raise IsolationError(f"账户组已注册: {group.accountGroupId}")
        self._groups[group.accountGroupId] = group

    def registerCredential(self, credential: CredentialEnvironmentV1) -> None:
        if credential.credentialId in self._credentials:
            raise IsolationError(f"凭据已注册: {credential.credentialId}")
        self._credentials[credential.credentialId] = credential

    def environmentOf(self, accountGroupId: str) -> EnvironmentKind | None:
        group = self._groups.get(accountGroupId)
        return group.environment if group is not None else None

    def validateMixedGroup(self, accountGroupIds: tuple[str, ...]) -> None:
        """同一运行/命令不得混用不同环境的账户组（LIVE 不能与非 LIVE 混组）。"""
        environments = {self.environmentOf(g) for g in accountGroupIds}
        if None in environments:
            unknown = [g for g in accountGroupIds if self.environmentOf(g) is None]
            raise IsolationError(f"未注册账户组: {unknown}")
        if len(environments) > 1:
            raise IsolationError(
                f"跨环境混组被拒绝: {sorted(e.value for e in environments if e is not None)}"
            )

    def validateCredentialForEnvironment(
        self, credentialId: str, environment: EnvironmentKind
    ) -> None:
        """测试凭据不能访问实盘：凭据环境必须匹配目标环境。"""
        credential = self._credentials.get(credentialId)
        if credential is None:
            raise IsolationError(f"凭据未注册: {credentialId}")
        if credential.environment is not environment:
            raise IsolationError(
                f"凭据 {credentialId} 属于 {credential.environment.value}，"
                f"不能访问 {environment.value}"
            )

    def validateCommandEnvironment(
        self,
        *,
        commandEnvironment: EnvironmentKind,
        accountGroupIds: tuple[str, ...],
    ) -> None:
        """跨环境命令被拒绝：命令环境必须与账户组环境一致。"""
        self.validateMixedGroup(accountGroupIds)
        for groupId in accountGroupIds:
            groupEnvironment = self.environmentOf(groupId)
            if groupEnvironment is None:
                raise IsolationError(f"账户组未注册: {groupId}")
            if groupEnvironment is not commandEnvironment:
                raise IsolationError(
                    f"跨环境命令被拒绝: 命令 {commandEnvironment.value} "
                    f"vs 账户组 {groupId} {groupEnvironment.value}"
                )
