"""P5-002 实盘环境隔离测试。"""

from __future__ import annotations

import pytest

from veritasquant.security.EnvironmentIsolation import (
    AccountGroupEnvironmentV1,
    CredentialEnvironmentV1,
    EnvironmentIsolationPolicyV1,
    EnvironmentKind,
    IsolationError,
)


def _policy() -> EnvironmentIsolationPolicyV1:
    policy = EnvironmentIsolationPolicyV1()
    policy.registerGroup(AccountGroupEnvironmentV1("grp-live-001", EnvironmentKind.Live))
    policy.registerGroup(AccountGroupEnvironmentV1("grp-paper-001", EnvironmentKind.Paper))
    policy.registerCredential(CredentialEnvironmentV1("cred-live-001", EnvironmentKind.Live))
    policy.registerCredential(CredentialEnvironmentV1("cred-test-001", EnvironmentKind.Paper))
    return policy


class TestEnvironmentIsolationPolicy:
    def test_environment_of(self) -> None:
        policy = _policy()
        assert policy.environmentOf("grp-live-001") is EnvironmentKind.Live
        assert policy.environmentOf("grp-paper-001") is EnvironmentKind.Paper
        assert policy.environmentOf("grp-unknown") is None

    def test_same_environment_group_ok(self) -> None:
        policy = _policy()
        policy.validateMixedGroup(("grp-live-001",))  # 不抛
        policy.validateMixedGroup(("grp-paper-001",))

    def test_mixed_group_rejected(self) -> None:
        """LIVE 不能与非 LIVE 混组。"""
        policy = _policy()
        with pytest.raises(IsolationError, match="跨环境混组"):
            policy.validateMixedGroup(("grp-live-001", "grp-paper-001"))

    def test_unknown_group_rejected(self) -> None:
        policy = _policy()
        with pytest.raises(IsolationError, match="未注册账户组"):
            policy.validateMixedGroup(("grp-unknown",))

    def test_test_credential_cannot_access_live(self) -> None:
        """测试凭据不能访问实盘。"""
        policy = _policy()
        with pytest.raises(IsolationError, match="不能访问"):
            policy.validateCredentialForEnvironment("cred-test-001", EnvironmentKind.Live)

    def test_live_credential_for_live_ok(self) -> None:
        policy = _policy()
        policy.validateCredentialForEnvironment("cred-live-001", EnvironmentKind.Live)

    def test_cross_environment_command_rejected(self) -> None:
        """跨环境命令被拒绝。"""
        policy = _policy()
        with pytest.raises(IsolationError, match="跨环境命令"):
            policy.validateCommandEnvironment(
                commandEnvironment=EnvironmentKind.Live,
                accountGroupIds=("grp-paper-001",),
            )

    def test_command_environment_match_ok(self) -> None:
        policy = _policy()
        policy.validateCommandEnvironment(
            commandEnvironment=EnvironmentKind.Live,
            accountGroupIds=("grp-live-001",),
        )

    def test_duplicate_registration_rejected(self) -> None:
        policy = _policy()
        with pytest.raises(IsolationError, match="已注册"):
            policy.registerGroup(AccountGroupEnvironmentV1("grp-live-001", EnvironmentKind.Live))

    def test_requires_identity(self) -> None:
        with pytest.raises(IsolationError):
            AccountGroupEnvironmentV1("", EnvironmentKind.Live)
        with pytest.raises(IsolationError):
            CredentialEnvironmentV1("", EnvironmentKind.Live)
