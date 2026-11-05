# Implementation Plan: 代码质量门禁（v0.1.3）

**Branch**: `005-quality-gates`（工作分支 dev/tdy） | **Date**: 2026-11-06 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/005-quality-gates/spec.md`

## Summary

在 git commit（lefthook pre-commit，增量）与 GitHub Actions CI（全量，权威）两个执行点建立代码质量门禁：规模约束（文件行数/函数长度/行长/参数个数）、复杂度约束（圈复杂度）、覆盖率阈值、重复代码、依赖漏洞、死代码。Go 侧 golangci-lint v2.15.0（`go tool` 锁版本）+ 自研 `tools/qualitygate`（文件行数检查 + 基线过滤 + 计数棘轮，TDD）；前端现有 ESLint 加规模/复杂度规则 + knip + jscpd + vitest thresholds；存量违规以 `quality-baseline.json` 显式豁免，棘轮只紧不松。阈值全部来自 research.md 的当前代码实测基线（Q22 定案）。

## Technical Context

**Language/Version**: Go 1.26（go.mod）；前端 TypeScript 5.7 / node 22（internal/frontend）
**Primary Dependencies**: golangci-lint v2.15.0（go tool 指令）、govulncheck v1.9.0（go tool）、eslint 9（现有）、jscpd 5.4.2 + knip 6.44.1（新增 devDependencies）、vitest 2.1（现有，补 thresholds）
**Storage**: 无运行时存储；门禁配置文件（.golangci.yml / eslint.config.js / vitest.config.ts / knip.json / quality-baseline.json）提交在仓库
**Testing**: Go 单测（tools/qualitygate，TDD）+ 注入式验收（quickstart.md）；既有测试全量回归
**Target Platform**: 开发者本地（macOS/Linux/Windows git-bash）+ GitHub Actions ubuntu CI
**Project Type**: CLI + 本地 Web GUI + 桌面（Wails）；本特性为工程化基建，不改产品行为
**Performance Goals**: pre-commit 新增质量检查 ≤10s（SC-004，增量模式实测 2–5s）
**Constraints**: 零清算上线（SC-002）；本地/CI 同一 make 目标与同一基线文件（FR-002）；版本锁定（FR-017）
**Scale/Scope**: Go 149 文件 21.7k 行 + 前端 src 137 文件 6.3k 行；7 个存量违规类别共 24 条基线条目（research §0）

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| 原则 | 检查 | 结论 |
|---|---|---|
| I. Eval/Wrapper 拆分 | 不触碰 shell 集成/emit 代码 | ✅ PASS |
| II. CLAUDE_CONFIG_DIR 隔离 | 不触碰切换机制/profile/routes | ✅ PASS |
| III. 跨平台三 OS | 质量工具全链路三平台可用：go tool（Go 交叉编译）、qualitygate（纯 Go）、eslint/jscpd/knip（node）；govulncheck/npm audit 仅 CI（ubuntu），不涉产品平台差异；不新增 shell 语义 | ✅ PASS |
| IV. 技术选型不漂移 | 不改语言/存储/GUI 选型；新增依赖均为**开发工具链**（go tool 指令 + devDependencies），不进入产品二进制；简单性原则下每项引入均在 research 给出理由 | ✅ PASS |
| V. 质量门禁与验收同步 | 本特性即质量门禁本身；`make check` 扩展后仍为唯一统一入口；验收用例随实现同步 docs/acceptance-tests.md | ✅ PASS |
| VI. 安全基线不降级 | 漏洞扫描（FR-013）主动加强安全基线；不触碰凭证存储 | ✅ PASS |
| VII. 文档链真值 | 需求已在 docs/requirements.md R12（唯一入口）；spec 不与 docs 冲突；实现完成后回流 docs/ | ✅ PASS |
| VIII. 测试先行（TDD） | tools/qualitygate 严格 red-green-refactor；配置类改动（阈值/规则）以注入式验收验证（quickstart） | ✅ PASS |

**Post-Phase-1 re-check**: 唯一新增产品外代码为 `tools/qualitygate`（开发工具，有测试、不入产品构建）；无违宪项，无需 Complexity Tracking。

## Project Structure

### Documentation (this feature)

```text
specs/005-quality-gates/
├── plan.md              # This file
├── research.md          # Phase 0：基线实测 + 工具/阈值定案（Q22）
├── data-model.md        # Phase 1：基线文件 schema + 配置文件模型
├── contracts/           # Phase 1：make 目标契约 + 基线格式契约
│   └── quality-gates.md
├── quickstart.md        # Phase 1：注入式验收指引
└── tasks.md             # Phase 2（/speckit-tasks 产出）
```

### Source Code (repository root)

```text
.golangci.yml                        # 新增：golangci-lint v2 配置（白名单 + 阈值 + 排除）
quality-baseline.json                # 新增：存量违规基线 + 计数棘轮 + 覆盖率阈值（单一真值）
tools/
└── qualitygate/                     # 新增：自研门禁工具（TDD）
    ├── main.go                      #   CLI 入口（子命令：filelines / filter / ratchet）
    ├── baseline.go                  #   基线读写与匹配
    ├── filelines.go                 #   Go 文件行数检查
    └── *_test.go                    #   单元测试（先于实现）
internal/frontend/
├── eslint.config.js                 # 修改：+5 条规模/复杂度规则 + test 豁免 override
├── vitest.config.ts                 # 修改：修复 coverage.include bug + thresholds
├── knip.json                        # 新增：knip 配置（入口/豁免，评审可见）
├── package.json                     # 修改：+jscpd/knip devDependencies + 脚本
└── package-lock.json                # 随依赖锁定
Makefile                             # 修改：+quality* 目标，check 与 pre-commit 接线
lefthook.yml                         # 修改：pre-commit 增量质量检查
.github/workflows/ci.yml             # 修改：lint job 全量门禁 + test job Linux 覆盖率棘轮
go.mod / go.sum                      # 修改：tool 指令（golangci-lint、govulncheck）
docs/
├── acceptance-tests.md              # 修改：AC20 质量门禁验收用例
└── quality-gates.md                 # 新增：门禁维护手册（阈值语义/豁免/修基线流程，FR-018/019）
```

**Structure Decision**: 自研工具放 `tools/`（非 `internal/`）——它是开发工具链而非产品代码，`go build ./internal/... .` 与发布产物不受影响；Go 生态惯例（golang.org/x/tools 同模式）。

## Complexity Tracking

> Constitution Check 无违宪项，本节留空。

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
