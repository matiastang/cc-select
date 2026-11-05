# Data Model: 代码质量门禁（v0.1.3）

**Date**: 2026-11-06 | **Spec**: [spec.md](./spec.md)

本特性无运行时数据；"实体"全部是仓库内配置文件与其字段。工具实现模型（tools/qualitygate 内部结构）见 tasks/implement。

## Entity 1: quality-baseline.json（质量基线，spec Key Entities 之核心）

**作用**: 存量违规显式豁免（FR-009）+ 计数棘轮 + 覆盖率阈值的单一真值（FR-002）。仓库根，提交在库，diff 可评审。

```jsonc
{
  "_comment": "手工维护说明见 docs/quality-gates.md；字段语义见 contracts/quality-gates.md",
  "version": 1,

  // 位置型条目：逐条豁免存量违规（按 tool 分组）
  "findings": {
    "golangci": [
      { "file": "internal/cli/add.go", "line": 211, "rule": "cyclop", "message": "calculated cyclomatic complexity for function readProviderInput is 26, max is 15" }
      // ...
    ],
    "eslint": [
      { "file": "src/features/providers/JsonForm.tsx", "line": 1, "rule": "max-lines", "message": "File has too many lines (560)." }
    ]
  },

  // 计数型棘轮：只允许减少（覆盖率只允许升高）
  "counters": {
    "jscpd-go":  { "count": 10, "metric": "clones" },
    "jscpd-frontend": { "count": 5, "metric": "clones" },
    "knip-frontend": { "count": 15, "metric": "issues" }
  },

  // 覆盖率阈值（下限，只允许上调）
  "coverage": {
    "go-statements-min": 76,
    "frontend-statements-min": 79
  },

  // 文件行数阈值（上限，产品代码；_test.go 豁免）
  "fileLines": {
    "go-max": 1000
  }
}
```

**字段规则**:

| 字段 | 类型 | 校验（qualitygate 实现） |
|---|---|---|
| `version` | int | 必须 = 1（格式演进入口） |
| `findings.<tool>[]` | array | `file` 非空相对路径；`line` ≥0（0 表示整文件级）；`rule` 非空；`message` 前缀匹配用 |
| `counters.<key>.count` | int ≥0 | 现值 > 基线 → 失败（棘轮）；< 基线 → 提示可下调基线 |
| `coverage.*-min` | number 0–100 | 现值 < 阈值 → 失败 |
| `fileLines.go-max` | int >0 | 任一产品 .go 行数超限且无 findings 豁免 → 失败 |

**状态迁移**（棘轮语义，spec FR-010）:

```text
违规存在 + 基线有条目 → PASS（存量）
违规存在 + 基线无条目 → FAIL（增量违规）
违规消失 + 基线有条目 → PASS + 提示「可删除条目」（棘轮向下，人工确认）
基线新增条目         → PASS 但 diff 中可见（评审追责）
counters 现值 > 基线  → FAIL（劣化）
counters 现值 < 基线  → PASS + 提示「可收紧基线」
coverage 现值 < min   → FAIL
```

## Entity 2: .golangci.yml（Go linter 白名单与阈值）

```yaml
version: "2"
linters:
  default: none            # v2 实测：enable-only 不生效，必须 default: none
  enable: [cyclop, funlen, lll, dupl, unused]
  settings:
    cyclop: { max-complexity: 15 }
    funlen:  { lines: 100, statements: 60 }   # 口径≈非空行（research R3）
    lll:     { line-length: 160 }             # 按 rune 计
    dupl:    { threshold: 150 }
  exclusions:
    paths: [internal/frontend/node_modules]
    rules:
      - path: _test\.go
        linters: [funlen, cyclop, lll, dupl]  # unused 对 test 保留
```

**不变量**: 本地与 CI 读同一份（FR-002）；`go tool golangci-lint` 版本由 go.mod 锁定（FR-017）。

## Entity 3: eslint.config.js 增量（前端规模/复杂度）

```js
// 追加到现有 config 的 src 规则组
"max-lines": ["error", { "max": 500, "skipBlankLines": true, "skipComments": true }],
"max-lines-per-function": ["error", { "max": 150, "skipBlankLines": true, "skipComments": true }],
"complexity": ["error", 15],
"max-depth": ["error", 4],
"max-params": ["error", 4],
// override：**/*.test.{ts,tsx} / **/*.spec.{ts,tsx} 五规则全关（research §3.2）
```

## Entity 4: knip.json（前端死代码配置）

入口（src/index.tsx 等）+ 豁免清单（`ignore`/`tags`，评审可见，FR-014 豁免机制）；具体入口在 implement 时按前端构建实际（vite 多入口? e2e?）核实。

## Entity 5: vitest.config.ts coverage（修复 + 阈值）

```ts
coverage: {
  provider: "v8",
  reporter: ["text", "html"],
  include: ["src/**/*.{ts,tsx}"],          // 修复：原为单文件（R11 遗留 bug）
  exclude: ["src/**/*.{test,spec}.{ts,tsx}", "src/test-setup.ts", "src/main.tsx"],
  thresholds: { statements: 79, branches: 80, functions: 77, lines: 79 },
},
```

## Entity 6: go.mod tool 指令（FR-017 版本锁定）

```go
tool (
    github.com/golangci/golangci-lint/v2/cmd/golangci-lint
    golang.org/x/vuln/cmd/govulncheck
)
```

**不变量**: `go build ./internal/... .` 产物不含工具依赖（tool 指令不参与产品构建）；`go mod tidy` 保留 tool 依赖（Go 1.24+ 语义）。

## 关系图

```text
quality-baseline.json ←──读取──→ tools/qualitygate ←──调用──→ Makefile ←──接线──→ lefthook.yml / ci.yml
        ↑ 唯一真值                    ↑ 过滤/棘轮/行数            ↑ 同一入口         （两端执行点）
.golangci.yml / eslint.config.js / knip.json / vitest.config.ts ──产出──→ 各工具输出 ──喂给──→ qualitygate
```
