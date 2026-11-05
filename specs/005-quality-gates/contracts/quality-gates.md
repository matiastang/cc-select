# Contracts: 质量门禁接口（v0.1.3）

**Date**: 2026-11-06 | **Spec**: [spec.md](./spec.md)

本特性的对外接口 = make 目标（本地/CI 共用，FR-002）+ qualitygate CLI（工具间契约）+ 基线文件格式（人机契约）。实体字段定义见 [data-model.md](../data-model.md)。

## Contract 1: Make 目标（本地与 CI 的唯一调用面）

| 目标 | 作用 | 执行点 | 幂等/只读 |
|---|---|---|---|
| `make quality-go` | golangci-lint 全量 + 基线过滤；Go 文件行数 | CI / 本地全量 | ✅ |
| `make quality-go-new` | golangci-lint `--new-from-rev HEAD` + 基线过滤 | pre-commit | ✅ |
| `make quality-frontend` | eslint 5 规则全量 + 基线过滤 | CI / 本地 | ✅ |
| `make quality-frontend-staged` | eslint 仅 staged 文件 | pre-commit | ✅ |
| `make quality-dup` | jscpd（Go + 前端）计数棘轮 | CI / 本地 | ✅ |
| `make quality-dead` | knip 计数棘轮（前端）；golangci unused 含于 quality-go | CI / 本地 | ✅ |
| `make quality-vuln` | govulncheck ./... + npm audit（需联网） | CI（本地可选） | ✅ |
| `make quality-coverage` | Go 覆盖率棘轮（`go test -cover`）+ 前端 vitest thresholds | CI / 本地 | ✅ |
| `make quality` | 以上全部（CI 等价集） | CI | ✅ |
| `make check` | **扩展**：现有目标 + `quality` | 两端 | ✅ |

**约束**:

1. 所有目标必须只依赖仓库内已提交的配置与 `npm install`/Go 工具链（现有 pre-commit 同等前提）；工具未安装 → 快速失败并输出安装提示（spec Edge Cases），MUST NOT 静默跳过。
2. `quality-frontend*` 从 `internal/frontend` 目录执行（npm 上下文），其余从仓库根执行。
3. 目标失败时输出 MUST 含：违规位置、实测值 vs 阈值（FR-007）。

## Contract 2: tools/qualitygate CLI

```text
qualitygate filelines --root <dir> --max <N> [--exclude <glob>]...
    遍历 .go 文件（产品代码，跳过 _test.go），行数 > max 输出违规清单，exit 1。

qualitygate filter --tool <golangci|eslint> --baseline <path>
    stdin 读工具输出（golangci: text；eslint: JSON），减去基线 findings，
    剩余（基线外）违规打到 stdout，非空 → exit 1。
    基线命中（存量）不输出。stdin 中的每条基线外违规按原样透传（供人读）。

qualitygate ratchet --baseline <path> [--metric jscpd-go|jscpd-frontend|knip-frontend]
    stdin 读 jscpd/knip JSON，取计数与基线 counters 比较：
    现值 > 基线 → 输出差异 → exit 1；
    现值 < 基线 → 输出「基线可收紧提示」→ exit 0。

qualitygate coverage --tool go --min <N>
    stdin 读 `go tool cover -func` 输出末行 total，< min → exit 1。
    （前端覆盖率门禁内置于 vitest thresholds，不经此命令。）
```

**通用行为**: `--help` 可用；JSON 解析失败 → exit 2（配置错误，区别于门禁失败 exit 1）；所有输出人类可读（含 file:line、实测、阈值）。

## Contract 3: quality-baseline.json 格式

见 [data-model.md](../data-model.md) Entity 1（字段表 + 状态迁移表）。要点：

1. `findings` 匹配语义：`file` + `line` 精确匹配、`rule` 精确匹配、`message` 前缀匹配（容忍行文微调）。
2. 修改基线的合法 diff 方向：删除 findings 条目（修复后）、counters 调小、coverage 阈值上调。反向 diff 在 PR 评审中必须给出理由（棘轮，FR-010）。
3. 文件顶层 `_comment` 字段必须存在（指向维护手册），防止无文档漂移。

## Contract 4: lefthook pre-commit 接线（增量语义）

```yaml
# 新增（与现有并列）：
  quality-go:
    glob: "**/*.go"
    run: make quality-go-new
  quality-frontend:
    glob: "internal/frontend/src/**/*.{ts,tsx}"
    run: make quality-frontend-staged
```

**约束**: 新增命令只校验本次变更文件（lefthook glob 已保证 staged 集合）；`make quality-go-new` 内部用 `--new-from-rev HEAD` 只报新代码。现有 hook 命令一字不动。

## Contract 5: CI lint job 步骤（全量语义）

```yaml
- run: make quality          # golangci 全量+基线 / eslint 全量+基线 / 文件行数 / jscpd / knip
- run: make quality-vuln     # govulncheck + npm audit（lint job 网络可用）
```

覆盖率棘轮：Go 在 test job 的 Linux runner 上执行（需测试运行）；前端由 vitest thresholds 在既有 `npm run coverage` 处生效。

**失败语义**: 任一步骤非零退出 → lint job 失败 → PR 阻止合入（US2）。
