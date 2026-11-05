# Quickstart: 质量门禁验收指引（v0.1.3）

**Date**: 2026-11-06 | **Spec**: [spec.md](./spec.md)

前提：Go 1.26 工具链、`npm install`（internal/frontend）、`go mod download`（拉取 tool 依赖）。首次同步后 `go tool golangci-lint version` 应输出 v2.15.0。

## 场景 1：提交拦截（US1，spec 注入验证）

```bash
# 1. 注入一个超长函数（>100 行）到任意 internal 包
cat > internal/version/inject_check.go <<'EOF'
package version

import "strings"

// InjectedOversize 是验收注入函数：故意超过函数长度阈值。
func InjectedOversize() string {
	s := ""
	for i := 0; i < 120; i++ { // 120 次分支/循环体，函数体 >100 行
		s += strings.Repeat("x", 1)
		if i%3 == 0 {
			s += "a"
		} else if i%3 == 1 {
			s += "b"
		} else {
			s += "c"
		}
	}
	return s
}
EOF
# （实际注入需把循环体展开为 >100 行物理代码——用 for 生成文件或直接粘贴长函数）

# 2. 尝试提交 → 必须 FAIL，报告含 文件路径/函数名/实测/阈值
git add internal/version/inject_check.go && git commit -m "test: 注入验收用例"
# 期望：pre-commit 拦截，输出 readProviderInput 同款 funlen 报告

# 3. 清理后提交 → 通过
rm internal/version/inject_check.go && git reset
```

同法注入：400 行 .go 文件（filelines 拦截）、复杂度 >15 的深层嵌套函数（cyclop 拦截）、前端 600 行 tsx（max-lines 拦截）、复制粘贴 60+ 行 Go 代码块（dupl/jsdpd 下次全量扫描计数上升）。

## 场景 2：CI 拦截（US2）

```bash
git commit --no-verify -m "test: 绕过本地钩子"   # 注入违规后绕过本地
git push origin <branch>                          # PR 触发 CI
# 期望：GitHub Actions lint job 失败，日志含与本地同口径报告（同阈值/同基线）
```

## 场景 3：基线与棘轮（US3）

```bash
# 1. 存量不阻塞：对含基线条目的文件做无关修改并提交 → 通过
#    （如 internal/cli/add.go 空注释改动）
# 2. 增量拦截：注入基线外违规提交 → 拦截
# 3. 棘轮不可逆：删除 cyclop 基线中的 add.go:211 条目但保留超限代码 → 全量扫描 FAIL
# 4. 棘轮向下：真的重构 readProviderInput 使 cc ≤15 → 删条目后 PASS
# 5. 计数棘轮：复制粘贴使 jscpd Go 克隆数 10 → 11 → FAIL；修掉一个克隆 → PASS + 提示可收紧
```

## 场景 4：覆盖率阈值（US4）

```bash
make quality-coverage
# 期望输出：Go total statements 76.3%（阈值 76，PASS）；
# 前端 vitest thresholds 79/80/77/79（PASS）
# 注入无测试大函数后重跑 → Go total 跌破 76 → FAIL 且报告两个数值
```

## 场景 5：重复/死代码/漏洞（US5/6/7）

```bash
make quality-dup    # 期望：Go 10 clones / FE 5 clones（棘轮通过）
make quality-dead   # 期望：knip 15 项（棘轮通过，清单完整输出）
make quality-vuln   # 期望：govulncheck "No vulnerabilities found" 或仅未触达警告；npm audit 按 high+ 生产依赖口径
```

## 一次性全量回归

```bash
make check          # 既有检查 + 新增 quality 全绿（SC-002：除基线/配置文件外零业务代码改动）
cd internal/frontend && npm run test:e2e   # 19 用例不回归
```

## 验收对照

| 场景 | Spec 验收 | 覆盖 SC |
|---|---|---|
| 1 | US1 全部 4 场景 | SC-001/004 |
| 2 | US2 全部 3 场景 | SC-001/003 |
| 3 | US3 全部 4 场景 | SC-002/005 |
| 4 | US4 全部 3 场景 | SC-006 |
| 5 | US5/6/7 各场景 | SC-006 |
