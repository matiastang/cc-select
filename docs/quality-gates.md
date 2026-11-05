# 代码质量门禁维护手册（v0.1.3 / R12）

> 需求见 [requirements.md v0.1.3](./requirements.md#v013)；spec 与阈值依据见 [specs/005-quality-gates](../specs/005-quality-gates/)。
> 本手册是门禁的**运维真值**：每类检查查什么、阈值多少、怎么本地跑、怎么豁免单点、怎么修基线。

---

## 1. 总览

| 检查 | 工具（锁版） | 阈值真值 | pre-commit | CI |
|---|---|---|---|---|
| Go 规模/复杂度/重复/死代码 | golangci-lint v2.15.0（go.mod `tool` 指令） | [.golangci.yml](../.golangci.yml) | ✅ 增量 | ✅ 全量 |
| Go 文件行数 | tools/qualitygate | [quality-baseline.json](../quality-baseline.json) `fileLines.go-max: 1000` | ✅ | ✅ |
| 前端规模/复杂度 | eslint 9（5 条规则） | [eslint.config.js](../internal/frontend/eslint.config.js) | ✅ staged | ✅ 全量 |
| 覆盖率 | vitest thresholds + qualitygate | baseline `coverage` + vitest.config.ts | ❌ | ✅（test job） |
| 重复代码（跨语言） | jscpd v5.4.2 | baseline `counters.jscpd-*` | ❌ | ✅ |
| 死代码（前端） | knip v6.44.1 | baseline `counters.knip-frontend` | ❌ | ✅ |
| 依赖漏洞 | govulncheck v1.9.0 + npm audit | 可触达口径 | ❌ | ✅ lint job |

**执行点入口**（两端共用，FR-002）：

```bash
make quality            # 全量聚合（golangci + eslint + jscpd + knip）
make quality-coverage   # 覆盖率棘轮（Go + 前端）
make quality-vuln       # 漏洞扫描（需联网）
make check              # 既有检查 + 全部新门禁
```

本地 pre-commit（lefthook）自动跑增量版：`make quality-go-new` / `make quality-frontend-staged`——只拦**本次变更**引入的违规，存量经基线豁免。

## 2. 阈值语义（Go 侧）

| Linter | 阈值 | 口径说明 |
|---|---|---|
| cyclop | max-complexity 15 | 圈复杂度 = 1 + 分支/循环/case/&&/`\|\|` 计数 |
| funlen | 100 行 / 60 语句 | **行数≈非空行**（注释空行不计）——104 物理行的函数实测不触发 |
| lll | 160 列 | 按 **rune** 计（中文注释友好）；`_test.go` 豁免（URL/fixture 天然长） |
| dupl | 150 tokens | token 级重复；与 jscpd（行级）互补 |
| unused | 默认 | 对测试文件**保留生效**（实测抓到过真死代码） |

前端（eslint，`src/**`，test 文件五规则全关）：`max-lines` 500 / `max-lines-per-function` 150（均 skipBlankLines+skipComments）/ `complexity` 15 / `max-depth` 4 / `max-params` 4。

**棘轮纪律（FR-010，不可协商）**：阈值与基线只允许「收紧」方向的 diff——删基线条目（修复后）、counters 调小、coverage 上调。放大豁免面的 diff 必须在 PR 评审给出书面理由。

## 3. 违规了怎么办

### 3.1 就地豁免（极少数合理例外）

| 场景 | 手段 | 评审可见性 |
|---|---|---|
| Go 单条违规确属合理 | 行内 `//nolint:<linter> // 原因` | ✅ 代码 diff |
| 前端单条违规 | `// eslint-disable-next-line <rule> // 原因` | ✅ 代码 diff |
| knip 误报（有契约理由的导出/二进制） | [knip.json](../internal/frontend/knip.json) `ignore` / `ignoreBinaries` | ✅ 配置 diff |
| 生成代码/第三方目录 | .golangci.yml `exclusions.paths` / eslint `ignores` / 脚本 `--exclude` | ✅ 配置 diff |

### 3.2 修基线（存量变化时）

1. **违规被真正修复** → 运行 `make quality`，工具会输出「基线中 N 条已失效，可删除」→ 从 `quality-baseline.json` 删除对应条目，随修复同 PR 提交。
2. **棘轮改善**（jscpd/knip 计数下降）→ ratchet 输出「基线可收紧到 N」→ 同步调小 `counters`。
3. **覆盖率上升** → 上调 `coverage.go-statements-min` / vitest `thresholds`。
4. **禁止**：为让门禁变绿而向基线**新增**条目或调大计数——那是把违规洗白成存量，review 必须拒收。

### 3.3 基线条目格式

```jsonc
// findings.<tool>：位置型（file+line+rule 精确，message 为前缀——容忍实测数值漂移）
{ "file": "internal/cli/add.go", "line": 211, "rule": "cyclop", "message": "calculated cyclomatic complexity for function readProviderInput" }
// line: 0 表示整文件级（如 filelines 的 api.go 豁免、eslint 的整文件规则）
// counters：计数型棘轮（现值 > 基线 = 劣化 = 阻塞）
// coverage：下限阈值（现值 < 阈值 = 劣化 = 阻塞）
```

## 4. 工具升级流程（FR-017）

- golangci-lint / govulncheck：`go get -tool <path>@<version> && go mod tidy`，跑 `make quality` 验证基线仍成立（新规则引入的存量违规要么修复要么走 3.2 流程评审）。
- jscpd / knip：`npm i -D <pkg>@<version>`（internal/frontend），跑 `make quality-dup quality-dead` 验证。
- 工具升级 = 显式流程决策：PR 说明新版本带来的规则差异。

## 5. 已知限制（记录在案，非缺陷）

- **funlen 口径**：约等于非空行数，与直觉的物理行数有差（§2 注）。
- **npm audit 无调用链分析**：以「生产依赖 + high 级别」近似 FR-013 的"可触达"口径；Go 侧 govulncheck 是严格调用图口径。
- **`--new-from-rev` 行号漂移**：基线条目按 file+line 匹配，重构移动代码后条目会"失效"——工具会提示（§3.2-1），按提示更新行号即可。
- **quality-coverage 为 sh 语法**（`$(go list ...)`）：Windows 本地跑该目标需 git-bash（cmd 不支持，与 scripts-check 同口径）；CI 为 Linux 不受影响。
- **jscpd 扫描范围**：排除 `_test.go` / `*.test.*` 与 node_modules；测试代码的重复由评审把关。
