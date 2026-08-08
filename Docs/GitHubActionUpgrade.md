# GitHub Actions Node 运行时升级手册（GitHubActionUpgrade）

> 适用：任何 GitHub 仓库/workflow 中 action 因 Node.js 运行时版本弃用（deprecation）而出现的警告排查与升级。
> 首次实践：2026-08-05 · FinvQuant PR #325（Node 20 → Node 24 全量升级，18 处 action）
> 关联规范：`Docs/DevSpec/GitSpec.md`（提交信息）、`Docs/DevSpec/DocSpec.md`（文档语言）

## 1. 问题识别（症状）

GitHub 会定期弃用 Actions runner 上较老的 Node 运行时（如 2025-09-19 公告弃用 Node.js 20，强制迁移到 Node 24）。典型警告出现在 **Actions 日志头部**：

```
Node.js 20 is deprecated. The following actions target Node.js 20 but are being forced
to run on Node.js 24: actions/setup-go@v5. For more information see:
https://github.blog/changelog/2025-09-19-deprecation-of-node-20-on-github-actions-runners/
```

要点：
- **警告按 action 逐个列出**，每个 `uses:` 的 action 都可能单独报一次（即使同一 action 多处使用，也各自警告）。
- 警告不阻断构建（强制跑在新 Node 上），但**迟早会变成硬失败**，且新 Node 上可能出现行为差异，应尽快修复。
- 修复目标：让所有 action 的 `runs.using` 声明为**当前受支持的 Node（2026 年 = node24）**。

## 2. 排查清单（全部 action 引用）

```bash
# 1) 列出所有 workflow 文件
ls -la .github/workflows/

# 2) 找出所有 action 引用（注意仓库内可能有多个子项目的 workflow，都要查）
grep -rn "uses:" .github/workflows/ . --include="*.yml" --include="*.yaml" 2>/dev/null \
  | grep -v node_modules | grep -v "\.git/"

# 3) 顺带排查文档中是否提到旧版本 action（可选，CI 只认 .github/workflows/）
grep -rn "uses:" --include="*.md" .
```

> ⚠️ **常见漏网之鱼**：仓库内嵌套子项目（如 FinvQuant 下的 `VeritasQuant/.github/workflows/Ci.yml`）也属于 git 跟踪范围，必须一并排查升级，否则下次跑那个 workflow 仍会警告。

## 3. 确认每个 action 的 Node 运行时版本（关键步骤）

**不要凭记忆猜版本**。每个 action 的 `runs.using` 字段定义在其仓库根目录的 `action.yml` 中，用 GitHub API 直接读取（`raw.githubusercontent.com` 在部分网络环境会挂起，用 `api.github.com/contents` 更稳）：

```bash
check() {
  echo "=== $1@$2 ==="
  curl -s --max-time 15 "https://api.github.com/repos/$1/contents/action.yml?ref=$2" \
    | python3 -c "import sys,json,base64; d=json.load(sys.stdin); print(base64.b64decode(d['content']).decode())" \
    | grep -E "using:" | head -1
}
check actions/setup-go v5   # 期望 node20（旧）
check actions/setup-go v6   # 期望 node24（新）
```

结果解读：`using: node20` = 已弃用需升级；`using: node24` = 当前受支持。

## 4. 查找目标版本（最新 release）

```bash
curl -s "https://api.github.com/repos/<owner>/<repo>/releases/latest" \
  | grep -E '"tag_name"|"published_at"'
```

- 优先选 `runs.using = node24` 的**最低大版本**（迁移风险最小），不一定要追最新 major。
- 若最新 major 也是 node24，可考虑用最新；但注意**大版本跳跃可能带 breaking change**，见 §6 参数兼容性检查。

## 5. 已知版本对照表（2026-08 核实）

| action | 旧（node20） | 新（node24） |
|---|---|---|
| `actions/checkout` | v4 | v5（v7 亦 node24） |
| `actions/setup-go` | v5 | v6（v7 亦 node24） |
| `actions/setup-node` | v4 | v5（v7 亦 node24） |
| `actions/setup-python` | v5 | v6 |
| `actions/upload-artifact` | v4 | **v6**（⚠️ v5 仍是 node20！） |
| `docker/login-action` | v3 | v4 |
| `docker/setup-qemu-action` | v3 | v4 |
| `docker/setup-buildx-action` | v3 | v4 |
| `docker/metadata-action` | v5 | v6 |
| `docker/build-push-action` | v6 | v7 |

> ⚠️ **upload-artifact 特例**：v5 发布时间早于 Node 24 迁移，`runs.using` 仍是 node20；v6 才是 node24。升级时不要照"最新大版本"惯性跳到 v5。

## 6. 升级前参数兼容性检查（防 breaking change）

大版本升级可能改参数。对本次用到的关键参数（尤其 `upload-artifact` 的 `name/path/retention-days/if-no-files-found`），读取新版本 `action.yml` 的 inputs 确认仍在：

```bash
curl -s --max-time 15 "https://api.github.com/repos/actions/upload-artifact/contents/action.yml?ref=v6" \
  | python3 -c "import sys,json,base64; d=json.load(sys.stdin); print(base64.b64decode(d['content']).decode())" \
  | grep -E "^\s+(name|path|retention-days|if-no-files-found):" | head -10
```

若某参数被移除/改名，需同步修改 workflow 中的用法，或评估是否换 action。

## 7. 修改与验证

### 修改原则
- 逐个 `uses:` 替换版本号；同一 action 多处引用全部替换，保持仓库内版本统一。
- 不改动 action 的 `with:` 参数内容（除非 §6 发现参数变更）。
- **node-version 字段（如 setup-node 的 `node-version: "24"`）是构建用的运行时，与 action 自身 Node 无关，不要动。**

### 本地验证
```bash
# YAML 语法校验（必须通过，否则 workflow 直接不运行）
python3 -c "
import yaml
for f in ['.github/workflows/CI.yml', '.../Ci.yml']:
    with open(f) as fh:
        d = yaml.safe_load(fh)
    print(f, 'OK, jobs:', list(d.get('jobs', {}).keys()))
"

# 复查无残留 node20 action
grep -rn "uses:" .github/workflows/ | grep -E "@v[0-9]+" | sort
```

### CI 实跑验证
- PR 触发后观察 `gh pr checks <N>`：重点确认**被警告过的 job 日志里不再出现 Node 20 deprecation 警告**。
- 注意 `if: github.event_name != 'pull_request'` 的 job（如镜像推送）在 PR 上会显示 `skipping`，属正常；其真实验证要等合并后 push 触发。

## 8. 交付规范（Git 流程）

按仓库红线规则，走 PR 流程（参考 `Docs/DevSpec/GitSpec.md`）：

```bash
git fetch acanx dev && git checkout -b fix/ci-node24-actions   # 基于最新 dev
# ...修改 workflow 文件...
git add .github/workflows/XX.yml && git commit -m "fix(ci): 升级 GitHub Actions 至 Node 24，消除 Node 20 弃用警告"
git push origin <branch>
gh pr create --repo <owner>/<repo> --base dev --head <fork>:<branch> --title "..." --body "..."
gh pr checks <N>   # 确认 CI 全绿
```

- **每个独立任务单独 commit**；配套文档可与代码同 PR 追加（`git commit --amend` 或追加 commit 后 push 到同一分支，前提是 PR 未合并）。
- 推送/追加前必须复查 PR 状态（`gh pr view <N> --json state,mergedAt`），已合并则新建分支与 PR。

## 9. 本次实践记录（FinvQuant PR #325）

- 升级范围：FinvQuant `.github/workflows/CI.yml`（7 处）+ `VeritasQuant/.github/workflows/Ci.yml`（11 处），共 18 处 action。
- 结果：3 个 CI job（BuildGoServer / BuildVueWeb / BuildFinvQuantImage-PRValidation）全绿，无 Node 20 警告。
- 经验：`actions/setup-go@v5` 与 `actions/setup-node@v4` 是高频警告源；checkout 在 docker job 中易被遗漏（v4 仍在用）。
