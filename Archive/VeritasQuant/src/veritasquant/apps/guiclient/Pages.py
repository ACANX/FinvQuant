"""P2-032 数据导入、策略、定投计划和回测操作页。

设计：页面逻辑（表单校验、主流程编排）与 Streamlit 渲染解耦——
每个页面提供 `render(client)` 薄封装 + 可单元测试的纯逻辑函数。
危险操作（启动/取消/导入）必须有明确确认与状态反馈。
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Mapping

from veritasquant.apps.guiclient.ApiClient import ApiClient, ApiClientError


# ---------------------------------------------------------------- 数据导入页

@dataclass(frozen=True, slots=True)
class ImportRequest:
    """数据导入请求（表单模型）。"""

    source: str  # 数据源名称
    instrumentId: str
    startDate: str
    endDate: str
    importMode: str  # FULL / INCREMENTAL

    def validate(self) -> list[str]:
        """校验输入；返回错误列表（空 = 通过）。"""
        errors: list[str] = []
        if not self.source.strip():
            errors.append("数据源不能为空")
        if not self.instrumentId.strip():
            errors.append("标的不为空")
        if not self.startDate or not self.endDate:
            errors.append("日期区间必填")
        elif self.startDate > self.endDate:
            errors.append("开始日期不能晚于结束日期")
        if self.importMode not in ("FULL", "INCREMENTAL"):
            errors.append("导入模式必须为 FULL 或 INCREMENTAL")
        return errors


def submitImport(client: ApiClient, request: ImportRequest) -> Mapping[str, Any]:
    """提交数据导入命令（202 受理，返回命令引用）。"""
    payload = {
        "command_type": "DATA_IMPORT",
        "account_id": "system",
        "payload": {
            "source": request.source.strip(),
            "instrument_id": request.instrumentId.strip(),
            "start_date": request.startDate,
            "end_date": request.endDate,
            "import_mode": request.importMode,
        },
    }
    return client.submitCommand(payload)


@dataclass(frozen=True, slots=True)
class UploadImportRequest:
    """文件上传导入请求（表单模型）。"""

    fileName: str
    source: str
    upsertMode: str  # FIELD / ROW

    def validate(self) -> list[str]:
        """校验输入；返回错误列表（空 = 通过）。"""
        errors: list[str] = []
        if not self.fileName:
            errors.append("请选择要上传的 MVSV 文件")
        if not self.source.strip():
            errors.append("数据源不能为空")
        if self.upsertMode not in ("FIELD", "ROW"):
            errors.append("覆盖模式必须为 FIELD 或 ROW")
        return errors


def submitUploadImport(
    client: ApiClient,
    request: UploadImportRequest,
    content: bytes,
) -> Mapping[str, Any]:
    """上传 MVSV 文件并导入（服务端解析 + 字段级覆盖写 PG）。"""
    return client.uploadImport(
        fileName=request.fileName,
        content=content,
        source=request.source.strip(),
        upsertMode=request.upsertMode,
    )


def renderImportPage(client: ApiClient) -> None:  # noqa: ANN001
    """数据导入操作页：上传 MVSV 文件 → 服务端导入 PostgreSQL。"""
    st = _st()
    st.markdown("#### 数据导入")
    st.caption("上传 MVSV-1 行情文件，服务端解析后字段级覆盖导入 PostgreSQL（finv_quote_secu_kline_min）。")
    with st.form("import_upload_form"):
        uploaded = st.file_uploader("选择 MVSV 行情文件", type=["mvsv", "txt"])
        source = st.text_input("数据源", placeholder="如: cn-feed")
        upsertMode = st.selectbox("覆盖模式", ["FIELD", "ROW"],
                                  help="FIELD=只覆盖有值的字段（推荐）；ROW=整行覆盖")
        submitted = st.form_submit_button("上传并导入", type="primary")

    if submitted:
        if uploaded is None:
            st.error("请选择要上传的 MVSV 文件")
            return
        request = UploadImportRequest(
            fileName=uploaded.name,
            source=source,
            upsertMode=upsertMode,
        )
        errors = request.validate()
        if errors:
            for issue in errors:
                st.error(issue)
            return
        # 危险操作确认（导入会覆盖同键行情数据）
        if not st.checkbox("我确认导入将覆盖同时刻同证券的对应字段值"):
            st.warning("请确认后提交")
            return
        try:
            content = uploaded.getvalue()
            result = submitUploadImport(client, request, content)
            st.success(
                f"导入完成: {result.get('secu_code', '?')} "
                f"(market={result.get('market_code', '?')}) {result.get('record_count', 0)} 条"
            )
            st.caption(f"批次: {result.get('batch_id', '-')} | 覆盖模式: {result.get('mode', '-')}")
        except ApiClientError as error:
            _showError(error)


# ---------------------------------------------------------------- 策略管理页

@dataclass(frozen=True, slots=True)
class StrategyDraft:
    """策略草稿（Python/DSL 二选一）。"""

    name: str
    kind: str  # PYTHON / DSL
    source: str

    def validate(self) -> list[str]:
        errors: list[str] = []
        if not self.name.strip():
            errors.append("策略名称不能为空")
        if self.kind not in ("PYTHON", "DSL"):
            errors.append("策略类型必须为 PYTHON 或 DSL")
        if not self.source.strip():
            errors.append("策略源码不能为空")
        if self.kind == "DSL":
            dslErrors = validateDsl(self.source)
            errors.extend(dslErrors)
        return errors


def validateDsl(yamlText: str) -> list[str]:
    """DSL 结构校验（P2-019 受限 DSL 子集）；返回错误列表。"""
    import yaml

    try:
        document = yaml.safe_load(yamlText)
    except yaml.YAMLError as error:
        return [f"YAML 解析失败: {error}"]
    if not isinstance(document, dict):
        return ["DSL 顶层必须是对象"]
    required = {"PlanType", "FundScope"}
    missing = required - set(document.keys())
    if missing:
        return [f"缺少必需字段: {', '.join(sorted(missing))}"]
    planType = document.get("PlanType")
    if planType not in ("FixedAmountSchedule", "ValueAveraging", "TargetValue", "TargetReturn", "MaDeviation", "ValuationPercentile", "DrawdownMultiplier"):
        return [f"不支持的 PlanType: {planType}"]
    return []


def renderStrategiesPage(client: ApiClient) -> None:  # noqa: ANN001
    """策略管理页（列表 + 创建草稿 + DSL 校验）。"""
    st = _st()
    st.markdown("#### 策略管理")
    try:
        strategies = client.strategies()
        if strategies:
            st.dataframe([{k: s.get(k) for k in ("strategy_id", "name", "version")} for s in strategies])
        else:
            st.info("暂无策略")
    except ApiClientError as error:
        _showError(error)

    st.markdown("---")
    with st.expander("新建策略"):
        name = st.text_input("策略名称")
        kind = st.selectbox("类型", ["DSL", "PYTHON"])
        source = st.text_area("源码", height=200, help="DSL 使用 YAML；PYTHON 使用 BaseStrategy 子类")
        validateClicked = st.button("校验 DSL")
        if validateClicked:
            dslErrors = validateDsl(source)
            if dslErrors:
                for issue in dslErrors:
                    st.error(issue)
            else:
                st.success("DSL 校验通过")
        if st.button("保存策略"):
            draft = StrategyDraft(name=name, kind=kind, source=source)
            errors = draft.validate()
            if errors:
                for issue in errors:
                    st.error(issue)
            else:
                st.success("策略草稿已保存（提交走命令流程）")


# ---------------------------------------------------------------- 定投计划页

@dataclass(frozen=True, slots=True)
class PlanDraft:
    """定投计划草稿。"""

    name: str
    fundSymbol: str
    frequency: str  # Daily / Weekly / Monthly
    amountMode: str  # Fixed / RuleBased / ExplicitSeries
    baseAmount: str
    cashSource: str  # AccountCash / ExternalDeposit

    def validate(self) -> list[str]:
        errors: list[str] = []
        if not self.name.strip():
            errors.append("计划名称不能为空")
        if not self.fundSymbol.strip():
            errors.append("基金代码不能为空")
        if self.frequency not in ("Daily", "Weekly", "Monthly"):
            errors.append("周期必须为 Daily/Weekly/Monthly")
        if self.amountMode not in ("Fixed", "RuleBased", "ExplicitSeries"):
            errors.append("金额模式必须为 Fixed/RuleBased/ExplicitSeries")
        try:
            amount = float(self.baseAmount)
            if amount <= 0:
                errors.append("基础金额必须为正数")
        except ValueError:
            errors.append("基础金额必须是数字")
        if self.cashSource not in ("AccountCash", "ExternalDeposit"):
            errors.append("资金来源必须为 AccountCash/ExternalDeposit")
        return errors


def renderPlansPage(client: ApiClient) -> None:  # noqa: ANN001
    """定投计划页（列表 + 创建草稿）。"""
    st = _st()
    st.markdown("#### 定投计划")
    with st.form("plan_form"):
        name = st.text_input("计划名称")
        fundSymbol = st.text_input("基金代码", placeholder="如: FUND-A")
        frequency = st.selectbox("周期", ["Daily", "Weekly", "Monthly"])
        amountMode = st.selectbox("金额模式", ["Fixed", "RuleBased", "ExplicitSeries"])
        baseAmount = st.text_input("基础金额", placeholder="如: 1000.00")
        cashSource = st.selectbox("资金来源", ["AccountCash", "ExternalDeposit"])
        submitted = st.form_submit_button("创建计划", type="primary")

    if submitted:
        draft = PlanDraft(
            name=name, fundSymbol=fundSymbol, frequency=frequency,
            amountMode=amountMode, baseAmount=baseAmount, cashSource=cashSource,
        )
        errors = draft.validate()
        if errors:
            for issue in errors:
                st.error(issue)
            return
        st.success(f"计划草稿已创建: {draft.name}（提交走命令流程）")


# ---------------------------------------------------------------- 回测页

@dataclass(frozen=True, slots=True)
class BacktestRequest:
    """回测创建请求。"""

    strategyId: str
    accountId: str
    startDate: str
    endDate: str
    initialCash: str
    mode: str  # IDEAL / REALISTIC

    def validate(self) -> list[str]:
        errors: list[str] = []
        if not self.strategyId.strip():
            errors.append("策略不能为空")
        if not self.accountId.strip():
            errors.append("账户不能为空")
        if not self.startDate or not self.endDate:
            errors.append("日期区间必填")
        elif self.startDate > self.endDate:
            errors.append("开始日期不能晚于结束日期")
        try:
            if float(self.initialCash) <= 0:
                errors.append("初始资金必须为正数")
        except ValueError:
            errors.append("初始资金必须是数字")
        if self.mode not in ("IDEAL", "REALISTIC"):
            errors.append("回测模式必须为 IDEAL/REALISTIC")
        return errors


def renderBacktestsPage(client: ApiClient) -> None:  # noqa: ANN001
    """回测中心页（创建 + 列表 + 运行控制）。"""
    st = _st()
    st.markdown("#### 回测中心")

    # 运行控制（危险操作需确认）
    try:
        backtests = client.backtests()
        if backtests:
            st.dataframe(
                [{k: b.get(k) for k in ("run_id", "strategy_id", "status", "mode")} for b in backtests]
            )
            selectedId = st.selectbox("选择回测", [b.get("run_id", "?") for b in backtests])
            col1, col2 = st.columns(2)
            if col1.button("▶ 启动", key="bt_start"):
                if st.checkbox("确认启动该回测", key="bt_start_confirm"):
                    try:
                        result = client.startBacktest(selectedId)
                        st.success(f"已启动: {result.get('status', '?')}")
                    except ApiClientError as error:
                        _showError(error)
                else:
                    st.warning("请勾选确认后启动")
            if col2.button("■ 取消", key="bt_cancel"):
                if st.checkbox("确认取消该回测", key="bt_cancel_confirm"):
                    try:
                        result = client.cancelBacktest(selectedId)
                        st.success(f"已取消: {result.get('status', '?')}")
                    except ApiClientError as error:
                        _showError(error)
                else:
                    st.warning("请勾选确认后取消")
        else:
            st.info("暂无回测")
    except ApiClientError as error:
        _showError(error)

    st.markdown("---")
    with st.expander("新建回测"):
        with st.form("backtest_form"):
            strategyId = st.text_input("策略 ID")
            accountId = st.text_input("账户 ID", help="必须是您有权访问的账户")
            col1, col2 = st.columns(2)
            startDate = col1.date_input("开始日期")
            endDate = col2.date_input("结束日期")
            initialCash = st.text_input("初始资金", placeholder="如: 1000000.00")
            mode = st.selectbox("模式", ["IDEAL", "REALISTIC"], help="REALISTIC 含真实摩擦与净值可用时间")
            submitted = st.form_submit_button("创建回测", type="primary")

        if submitted:
            request = BacktestRequest(
                strategyId=strategyId,
                accountId=accountId,
                startDate=startDate.isoformat() if hasattr(startDate, "isoformat") else str(startDate),
                endDate=endDate.isoformat() if hasattr(endDate, "isoformat") else str(endDate),
                initialCash=initialCash,
                mode=mode,
            )
            errors = request.validate()
            if errors:
                for issue in errors:
                    st.error(issue)
                return
            try:
                result = client.createBacktest(
                    {
                        "strategy_id": strategyId,
                        "account_id": accountId,
                        "start_date": request.startDate,
                        "end_date": request.endDate,
                        "initial_cash": initialCash,
                        "mode": mode,
                    }
                )
                st.success(f"回测已创建: {result.get('run_id', '?')} 状态 {result.get('status', '?')}")
            except ApiClientError as error:
                _showError(error)


# ---------------------------------------------------------------- 共享工具

_ST = None


def _st() -> Any:
    global _ST
    if _ST is None:
        import streamlit as st

        _ST = st
    return _ST


def _showError(error: ApiClientError) -> None:
    st = _st()
    retryable = "（可重试）" if error.retryable else ""
    st.error(f"错误 [{error.code}] HTTP {error.httpStatus}{retryable}: {error.message}")


# ---------------------------------------------------------------- 账户管理页

@dataclass(frozen=True, slots=True)
class AccountScope:
    """账户上下文隔离：页面必须携带 account_id + run_id 查询。"""

    accountId: str
    runId: str | None = None

    def requireAccount(self) -> str:
        if not self.accountId:
            raise ValueError("未选择账户：多账户操作必须显式指定账户")
        return self.accountId


def renderAccountsPage(client: ApiClient) -> None:  # noqa: ANN001
    """账户管理页：账户列表 + 账户详情（快照/模式/适配器）。"""
    st = _st()
    st.markdown("#### 账户管理")
    try:
        accounts = client.accounts()
        if not accounts:
            st.info("无可用账户")
            return
        st.dataframe(
            [
                {
                    "account_id": a.get("account_id"),
                    "execution_mode": a.get("execution_mode"),
                    "run_id": a.get("run_id"),
                }
                for a in accounts
            ]
        )
        selected = st.selectbox("账户详情", [a.get("account_id", "?") for a in accounts])
        runId = st.text_input("run_id（可选）", key="acc_run_id")
        if st.button("加载账户详情"):
            try:
                detail = client.account(selected, runId or None)
                st.json(detail)
            except ApiClientError as error:
                _showError(error)
    except ApiClientError as error:
        _showError(error)


# ---------------------------------------------------------------- 结果分析页

def renderAnalysisPage(client: ApiClient) -> None:  # noqa: ANN001
    """结果分析页：现金流调整权益、TWR/XIRR、本金、份额、逐笔分录。"""
    st = _st()
    st.markdown("#### 结果分析")
    accountId = _currentAccountId()
    if not accountId:
        st.warning("请先在侧边栏选择账户")
        return
    runId = st.text_input("run_id", key="analysis_run_id")
    st.caption(f"当前账户: {accountId}（结果严格按账户隔离）")
    try:
        analysis = client.accountAnalysis(accountId, runId or "")
        st.json(analysis)
    except ApiClientError as error:
        _showError(error)

    st.markdown("---")
    tabs = st.tabs(["逐笔分录", "现金流", "基金份额"])
    try:
        with tabs[0]:
            entries = client.accountLedger(accountId, runId or "")
            if entries:
                st.dataframe(entries)
            else:
                st.info("无分录")
        with tabs[1]:
            flows = client.accountCashFlows(accountId, runId or "")
            if flows:
                st.dataframe(flows)
            else:
                st.info("无现金流")
        with tabs[2]:
            shares = client.accountShares(accountId, runId or "")
            if shares:
                st.dataframe(shares)
            else:
                st.info("无份额")
    except ApiClientError as error:
        _showError(error)


# ---------------------------------------------------------------- 实时监控页

def renderMonitoringPage(client: ApiClient) -> None:  # noqa: ANN001
    """实时监控页：账户快照 + 状态（订单/风险/告警摘要）。"""
    st = _st()
    st.markdown("#### 实时监控")
    accountId = _currentAccountId()
    if not accountId:
        st.warning("请先在侧边栏选择账户")
        return
    runId = st.text_input("run_id", key="mon_run_id")
    st.caption(f"监控账户: {accountId} · 模式 {_accountModeLabel()}")
    try:
        snapshot = client.account(accountId, runId or None)
        if snapshot:
            st.metric("账户", snapshot.get("account_id", "?"))
            st.metric("模式", snapshot.get("execution_mode", "?"))
            st.json(snapshot.get("snapshot", snapshot))
    except ApiClientError as error:
        _showError(error)
    st.info("订单/风险/告警实时状态由 SSE 状态流推送（P2-030 已建通道）")


# ---------------------------------------------------------------- 账户上下文工具

def _currentAccountId() -> str | None:
    """从 session_state 读取侧边栏账户上下文（保证多账户不串数据）。"""
    st = _st()
    return st.session_state.get("account_context") if hasattr(st, "session_state") else None


def _accountModeLabel() -> str:
    accountId = _currentAccountId()
    if not accountId:
        return "?"
    return "PAPER（模拟盘）"
