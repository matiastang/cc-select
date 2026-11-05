package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

// golangciFindingRe 匹配 golangci-lint text 输出的 finding 行：
// <file>:<line>:<col>: <message> (<linter>)
// 代码上下文行（func/^/缩进）与摘要行（N issues: / * linter: n）不匹配，自然被忽略。
var golangciFindingRe = regexp.MustCompile(`^([^\s:]+\.go):(\d+)(?::\d+)?: (.+) \(([a-z]+)\)$`)

// parseGolangciOutput 从 golangci text 输出提取 findings（按输出顺序）。
// 解析不了的行一律忽略：golangci 的输出格式是它的内部细节，解析器保持宽容；
// 门禁的"工具崩溃检测"由 execGolangci 的退出码承担，不依赖解析严格性。
func parseGolangciOutput(r io.Reader) ([]Finding, error) {
	var findings []Finding
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		m := golangciFindingRe.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}
		line := 0
		if _, err := fmt.Sscanf(m[2], "%d", &line); err != nil {
			return nil, fmt.Errorf("解析行号 %q: %w", m[2], err)
		}
		findings = append(findings, Finding{
			File:    m[1],
			Line:    line,
			Rule:    m[4],
			Message: m[3],
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return findings, nil
}

// runGolangciGate 实现 `qualitygate golangci`：自执行 `go tool golangci-lint run`
// （不走 shell 管道——工具崩溃时退出码非 0/1，直接判配置错误，杜绝"空输入假绿"），
// 解析输出并按基线过滤。
//
//	qualitygate golangci --baseline <path> [--new-from-rev <rev>] [--packages <pkgs>]
//
// exit：0 干净或全部基线豁免 / 1 存在基线外违规 / 2 工具或配置错误。
func runGolangciGate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("golangci", flag.ContinueOnError)
	fs.SetOutput(stderr)
	baselinePath := fs.String("baseline", "quality-baseline.json", "基线文件路径")
	newFromRev := fs.String("new-from-rev", "", "只报该 rev 之后的新增违规（pre-commit 增量模式）")
	pkgs := fs.String("packages", "./...", "golangci 目标包")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	baseline, err := LoadBaseline(*baselinePath)
	if err != nil {
		fmt.Fprintf(stderr, "golangci: %v\n", err)
		return 2
	}

	goArgs := []string{"tool", "golangci-lint", "run", "--timeout", "5m", *pkgs}
	if *newFromRev != "" {
		goArgs = append(goArgs, "--new-from-rev", *newFromRev)
	}
	cmd := exec.Command("go", goArgs...)
	cmd.Dir = repoRoot()
	out, err := cmd.Output()
	if err != nil {
		// golangci 约定：exit 0/1 是正常结论（0 干净、1 有 finding），其余是工具/环境错误。
		exitErr, ok := err.(*exec.ExitError)
		if !ok || exitErr.ExitCode() > 1 {
			fmt.Fprintf(stderr, "golangci: 工具执行失败: %v\n%s\n", err, strings.TrimSpace(string(out)))
			return 2
		}
	}

	findings, err := parseGolangciOutput(strings.NewReader(string(out)))
	if err != nil {
		fmt.Fprintf(stderr, "golangci: 解析输出失败: %v\n", err)
		return 2
	}

	return reportFindings("golangci", findings, baseline, stdout, stderr)
}

// reportFindings 是 golangci/eslint 共用的过滤与报告（US1/US3 核心语义）：
// 基线命中的存量豁免并计数；基线外违规逐条输出 → exit 1；
// 基线中未被任何现违反命中的条目（已修复的存量）提示可删除——棘轮向下的唯一合法路径。
func reportFindings(tool string, findings []Finding, baseline *Baseline, stdout, stderr io.Writer) int {
	var fresh []Finding
	baselined := 0
	matched := make([]bool, len(baseline.Findings[tool]))
	for _, f := range findings {
		hit := false
		for i, b := range baseline.Findings[tool] {
			if b.matches(f) {
				matched[i] = true
				hit = true
				break
			}
		}
		if hit {
			baselined++
			continue
		}
		fresh = append(fresh, f)
	}

	if len(fresh) == 0 {
		if baselined > 0 {
			fmt.Fprintf(stdout, "%s: %d 条存量违规经基线豁免，无新增\n", tool, baselined)
		} else {
			fmt.Fprintf(stdout, "%s: 通过（无违规）\n", tool)
		}
		reportRecyclable(tool, baseline, matched, stdout)
		return 0
	}

	sort.Slice(fresh, func(i, j int) bool { return fresh[i].File < fresh[j].File })
	fmt.Fprintf(stdout, "%s: 基线外违规 %d 条（基线豁免 %d 条）:\n", tool, len(fresh), baselined)
	for _, f := range fresh {
		fmt.Fprintf(stdout, "  %s:%d [%s] %s\n", f.File, f.Line, f.Rule, f.Message)
	}
	reportRecyclable(tool, baseline, matched, stdout)
	fmt.Fprintf(stdout, "修复指引见 docs/quality-gates.md（单点豁免 / 修基线规则）。\n")
	return 1
}

// reportRecyclable 输出失效基线条目（对应违规已不存在，棘轮向下可删除）。
func reportRecyclable(tool string, baseline *Baseline, matched []bool, stdout io.Writer) {
	var recyclable []Finding
	for i, b := range baseline.Findings[tool] {
		if !matched[i] {
			recyclable = append(recyclable, b)
		}
	}
	if len(recyclable) == 0 {
		return
	}
	fmt.Fprintf(stdout, "%s: 基线中 %d 条已失效（对应违规不存在），可删除条目收紧基线:\n", tool, len(recyclable))
	for _, f := range recyclable {
		fmt.Fprintf(stdout, "  可删除: %s:%d [%s]\n", f.File, f.Line, f.Rule)
	}
}

// repoRoot 返回仓库根（qualitygate 可能在任意 cwd 下被调用；go.mod 所在即根）。
// 简化：假定从仓库根运行（Makefile/CI/lefthook 均保证），失败时回退当前目录。
func repoRoot() string {
	if _, err := os.Stat("go.mod"); err == nil {
		return "."
	}
	return ""
}
