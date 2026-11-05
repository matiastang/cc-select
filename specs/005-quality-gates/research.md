# Research: 代码质量门禁（v0.1.3）

**Date**: 2026-11-06 | **Spec**: [spec.md](./spec.md) | **Constitution**: v1.2.2

本文档回答 spec 留给 plan 阶段的三个问题：Q22 阈值取值（基线实测）、工具版本与形态、基线/棘轮的落地机制。

## 0. 基线测量（2026-11-06 实测，Q22 数据依据）

### 0.1 Go（149 文件 / 21,670 行；产品代码 413 函数）

| 度量 | 实测 | 说明 |
|---|---|---|
| 文件行数（产品） | 最大 `internal/web/api.go` 1139 | test 最大 `internal/web/api_test.go` 1649 |
| 函数长度 | 最大 `readProviderInput`（cli/add.go:211）104 物理行 | funlen 口径（非空行）实测 ≤100 |
| 圈复杂度 | 产品 top：26 / 19 / 17（>15 共 3 个） | gocyclo 口径；test 文件另最高 22 |
| 行长 >160（产品） | 2 行（server.go:148 = 212 列；add.go 1 行按 rune 计 ≤160） | lll 按 rune 计 |
| 重复块（jscpd v5，min-tokens 60，排除 test/node_modules） | **10 块 / 111 行 / 1.05%**（87 文件） | |
| 死代码（golangci unused） | 2（均在 _test.go） | |
| 覆盖率（statements） | **76.3%** | `go test ./internal/... -cover` |

### 0.2 前端（src/ 6,260 行 TS/TSX）

| 度量 | 实测 | 说明 |
|---|---|---|
| 文件行数（产品） | 最大 `JsonForm.tsx` 560 | test 最大 `JsonForm.test.tsx` 420 |
| ESLint 候选阈值违规（产品，排除 test） | **12 条**：mlpf(150)×5、complexity(15)×4、max-lines(500)×1、max-params(4)×1、max-depth(4)×0 | 见 §3.2 |
| 重复块（jscpd v5，min-tokens 50，排除 test） | **5 块 / 56 行 / 1.14%**（65 文件） | |
| 死代码（knip v6.44.1） | **15 项**：unused exports×10、unused exported types×4、unlisted binaries×1 | |
| 覆盖率（真实口径，修正 include 后） | **statements 79.78 / branches 80.09 / functions 77.18 / lines 79.78** | |

**发现（重要）**：`vitest.config.ts` 的 `coverage.include` 只圈了 `src/ShellIntegrationBanner.tsx` 单文件（R11 遗留），导致覆盖率报告恒为全 0——真实覆盖从未被度量过。本特性必须先修复该配置（属 FR-011 范畴），否则覆盖率门禁没有意义。

## 1. D1 Go 聚合器：golangci-lint v2.15.0（Q24 已确认）

- **Decision**: golangci-lint **v2.15.0**，经 `go.mod` 的 `tool` 指令固定（`go tool golangci-lint`）。
- **Rationale**:
  - v2.15.0 为当前最新稳定版（实测源码编译安装成功；官方二进制下载经代理校验失败，`go tool` 从模块代理取源码反而可靠，且天然满足 FR-017 版本锁定、本地/CI 同版本）。
  - 配置采用 `linters.default: none` + `enable:` 显式清单（实测 v2 的 `enable-only` 键不生效；`default: none` 是 v2 文档口径）。
- **Alternatives considered**:
  - 自写轻量脚本：复杂度/重复检测算法易错、无生态（Q24 已否决）。
  - 固定版本安装脚本（curl installer）：checksum 校验在本环境实测失败；版本漂移需另行锁定。
- **启用的 linter（default: none，严格白名单）**：`cyclop`（圈复杂度）、`funlen`（函数长度）、`lll`（行长）、`dupl`（重复）、`unused`（死代码）。
- **刻意不启用**：`errcheck`/`staticcheck`/`govet` 等默认集——它们与 R12 的"规模/复杂度"正交，且存量违规多（实测 errcheck 大量命中 `os.Setenv` 惯用法）；引入属于后续显式决策（记录于 docs 回流），避免本期基线文件被无关条目淹没。`go vet` 已由现有 `make check` 覆盖。
- **排除项**：`internal/frontend/node_modules/**`（含 flatted 的 Go 移植，无 go.mod 嵌套会被扫入，实测 cyclop 在此误报 2 条）；`_test.go` 对 funlen/cyclop/lll/dupl 豁免（测试函数天然长；unused 保留对 test 生效，实测 2 条真死代码）。

## 2. D2 Go 阈值定案（Q22）

| 检查 | 阈值 | 基线条目数 | 依据 |
|---|---|---|---|
| cyclop max-complexity | **15** | 3（add.go:211=26、cliinstall.go:113=19、validate.go:44=17） | 行业常用宽松起步值；存量 3 个可登记后逐步重构 |
| funlen lines/statements | **100 / 60** | 0 | funlen 口径约等于非空行（实测 104 物理行函数未触发 100）；零基线起步 |
| lll line-length | **160** | 1（router/server.go:148） | 120 会产生 34 条存量噪声；160 下产品代码仅 1 条；test 文件豁免（URL/fixture 天然长） |
| dupl threshold | **150 tokens**（默认） | 1 对（shell/powershell.go ↔ zsh.go 1-71 行） | 两个 emitter 的文档注释+结构天然相似，登记后可观察 |
| unused | 默认 | 2（desktop/bridge_test.go:20、usage/usage_test.go:14） | 真实死代码，登记为清理清单 |
| 文件行数（qualitygate 自研，见 D4） | **1000**（产品，`_test.go` 豁免） | 1（web/api.go 1139） | 整数门槛传达"千行上限"意图；api.go 是已知重构候选 |

## 3. D3 前端门禁：现有 ESLint 体系 + 候选阈值实测

### 3.1 ESLint 规则（零新依赖，沿用现有 eslint.config.js）

| 规则 | 阈值 | 产品违规 | 依据 |
|---|---|---|---|
| `max-lines` | **500**（不含空行与注释） | 1（JsonForm.tsx 560） | 对应 FR-003；test 文件豁免（§3.2） |
| `max-lines-per-function` | **150** | 5 | spec 默认 50 过严（实测大量命中）；150 宽松起步 |
| `complexity` | **15** | 4 | 与 Go 侧 cyclop 同值，口径对齐 |
| `max-depth` | **4** | 0 | 行业默认 |
| `max-params` | **4** | 1 | 对应 FR-006 参数上限 |

### 3.2 test 文件豁免（`**/*.test.*`/`**/*.spec.*` override）

- 规模/复杂度类规则（max-lines、max-lines-per-function、complexity、max-depth、max-params）对测试文件**关闭**：表驱动测试天然长；测试质量由覆盖率门禁约束。
- 与 Go 侧"_test.go 对规模类 linter 豁免"口径一致（FR-002 两端一致）。

### 3.3 文件行数

前端 `max-lines` 即文件行数门禁，无需自研脚本；自研文件行数检查仅覆盖 Go（无现成 linter）。

## 4. D4 基线/棘轮机制：自研 Go 工具 `tools/qualitygate`（本特性唯一新代码）

- **Decision**: 新增 `tools/qualitygate/`（Go CLI，含完整单元测试），职责：
  1. **Go 文件行数检查**（go/parser 不需要——按行数统计即可，walk + count）；
  2. **基线过滤**：读入 linter 输出（golangci text / eslint JSON / knip JSON / jscpd JSON），与 `quality-baseline.json` 对照，输出"基线外新增违规"并退出码 1；基线内存量静默通过；
  3. **计数棘轮**：jscpd 克隆数、knip 项数、Go/前端覆盖率——与基线记录的数值比较，只允许 ≤（覆盖率 ≥）。
- **Rationale**:
  - 宪法原则 VIII 要求 TDD——Go 工具可用标准 `go test` 严格 red-green；bash/mjs 脚本在本仓库没有测试基建。
  - 跨平台（Go 编译静态二进制，三平台 CI 与 Windows 本地一致，宪法原则 III）。
  - 基线以 **`quality-baseline.json`**（仓库根，人类可读 JSON）承载：位置条目（tool/file/line/rule/message）+ 计数条目（tool/scope/count）。diff 可评审、可追责（spec US3 场景 3）。
- **Alternatives considered**:
  - 工具原生基线：golangci-lint v2 无 baseline 功能；eslint 无（--max-warnings 只能计数）；knip/jscpd 同样无。自研是唯一满足 FR-009/FR-010 的路径。
  - bash 脚本：Windows cmd/git-bash 差异大（Makefile CMD_SHELL 已证明痛苦），且无法 TDD。

## 5. D5 重复代码检测：jscpd v5.4.2

- **Decision**: jscpd **v5.4.2**（npm devDependencies 固定于 `internal/frontend/package.json`，`npx --no` 本地优先——与 commitlint 同惯例）；min-tokens：Go 60 / 前端 50；排除 `**/*_test.go`、`**/*.test.*`、`node_modules`。
- **门禁口径**：克隆**块数计数棘轮**（Go 10、前端 5，见 §0），不用百分比——百分比随总行数波动，新增大文件会稀释分母掩盖新重复。
- **Rationale**: v5 为 Rust 引擎维护线（v3 已废弃，npm 实测警告）；块数只减不增即棘轮（spec US5）。
- **Alternatives considered**: golangci `dupl`（已启用，token 级互补，负责 Go 单语言细粒度）；jscpd v3（废弃）。

## 6. D6 死代码检测：golangci `unused` + knip v6.44.1

- **Go**: `unused`（§1），基线 2 条；单点豁免用 `//nolint:unused // 原因`（评审可见）。
- **前端**: knip **v6.44.1**（devDependencies 固定），`knip.json` 配置入口与豁免（`ignore`/tags，提交评审）；门禁 = 项数计数棘轮（15，§0.2），每次 CI 输出完整清单保持可见。
- **Rationale**: knip 是 TS 生态事实标准（exports/types/deps/unlisted binaries 一体）；count-ratchet 让"只减不增"无需逐项定位文件（knip 修复常伴随文件移动）。

## 7. D7 覆盖率阈值：vitest thresholds + qualitygate 棘轮

- **前端**: 修复 `coverage.include` bug（→ `src/**/*.{ts,tsx}`，exclude tests/setup/main）后在 `vitest.config.ts` 写入 `coverage.thresholds`：**statements 79 / branches 80 / functions 77 / lines 79**（当前值向下取整，避免正常波动误报；后续棘轮只升）。`npm run coverage` 自带门禁语义。
- **Go**: `go test ./internal/... -coverprofile` → 语句覆盖率 ≥ **75**（qualitygate 直接解析 coverprofile，避免管道假绿）。**implement 复测修订**：plan 阶段实测 76.3%，implement 阶段稳定复测为 **75.5%**（同命令、同代码、多次一致；差异来源未定位），按棘轮"留缓冲防抖动"原则阈值取 75。T029 回流时复核。
- **执行点**: CI（前端在 e2e 前的独立 step 或复用现有流程；Go 在 test job 的 Linux 分支）；本地 `make` 目标可手动运行。pre-commit 不放覆盖率（慢，秒级约束 SC-004）。

## 8. D8 依赖漏洞扫描：govulncheck + npm audit

- **Go**: `golang.org/x/vuln/cmd/govulncheck`（`go tool` 指令固定版本）`./...`——go 调用图分析天然满足"可触达才阻塞"（US6 场景 2）；实测安装 v1.9.0。
- **前端**: `npm audit --audit-level=high --omit=dev`——仅生产依赖、仅 high+。**已知近似**：npm audit 无调用链分析，达不到严格"可触达"口径；以"生产依赖 + 高危级别"收敛误报，偏差记录于 acceptance 文档。
- **执行点**: CI lint job（网络可用）；本地 make 目标可选执行。pre-commit 不放（需联网，违反 commitlint `npx --no` 惯例精神）。

## 9. D9 执行点与增量策略（FR-001/FR-008）

| 检查 | pre-commit（增量） | CI lint job（全量） | 备注 |
|---|---|---|---|
| golangci-lint | `--new-from-rev HEAD`（只报新代码） | 全量 + 基线过滤 | 实测单次 2–5s，满足 SC-004 |
| eslint（5 规则） | lefthook staged-files glob（现有机制） | 全量 + 基线过滤 | |
| Go 文件行数 | qualitygate 仅 staged .go | 全量 + 基线 | |
| jscpd / knip / govulncheck / npm audit / 覆盖率 | ❌（慢/需联网） | ✅ | 文档明示 |
| 基线文件本身 | 变更可见（git diff） | 同一文件 | 两端一致 FR-002 |

- pre-commit 全部质量检查经 **make 目标**（`make quality-commit`）进入 lefthook，与现有 fmt-check/vet 并列——两端共用同一 make 入口（FR-002）。
- CI：扩展 `.github/workflows/ci.yml` lint job（不新增矩阵维度，Assumptions 既定）。

## 10. D10 平台与工具链约束核对（宪法原则 III）

- `go tool` 指令三平台一致；qualitygate 为纯 Go；eslint/jscpd/knip 经 npm（node 22 已是前端既定依赖）。
- lefthook 在 Windows 走 sh（git-bash），现有 hook 同模式，新增目标沿用。
- govulncheck/npm audit 仅 CI ubuntu——不影响本地三平台开发者。

## 11. 遗留风险

- **R1**: golangci-lint `go tool` 依赖树较大（go.mod 工具依赖 ~50 条间接依赖）——`tool` 指令不进入产品构建（`go build .` 不含工具），go.sum 增大属可接受成本。
- **R2**: jscpd v5 较新（Rust 引擎），npm 装包体积 ~40MB——仅 devDependency + CI 缓存覆盖。
- **R3**: funlen 口径（非空行）与直觉（物理行）有差——文档明示阈值语义（§2 注）。
- **R4**: npm audit "可触达"近似（§8）——已在 acceptance 记录偏差。
