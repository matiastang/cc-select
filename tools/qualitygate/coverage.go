package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// coverStats 是 coverprofile 的聚合结果。
type coverStats struct {
	totalStmts   int
	coveredStmts int
}

// percent 返回覆盖率百分比（语句口径，与 go tool cover -func total 一致）。
func (s coverStats) percent() float64 {
	if s.totalStmts == 0 {
		return 0
	}
	return float64(s.coveredStmts) / float64(s.totalStmts) * 100
}

// parseCoverProfile 解析 go test -coverprofile 输出：
//
//	mode: set
//	<path>:<startL>.<startC>,<endL>.<endC> <numStmt> <count>
//
// 空 profile（只有 mode 行）报错：按 0% 放行是假绿，按失败处理才能暴露测试未运行。
func parseCoverProfile(r io.Reader) (coverStats, error) {
	var stats coverStats
	scanner := bufio.NewScanner(r)
	first := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if first {
			first = false
			if !strings.HasPrefix(line, "mode:") {
				return stats, fmt.Errorf("coverprofile 缺 mode 行: %q", line)
			}
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return stats, fmt.Errorf("coverprofile 非法行: %q", line)
		}
		numStmt, err := strconv.Atoi(fields[1])
		if err != nil || numStmt < 0 {
			// numStmt=0 真实存在（如空语句块 56.11,56.11 0 0），合法，贡献 0。
			return stats, fmt.Errorf("coverprofile 语句数非法: %q", line)
		}
		count, err := strconv.Atoi(fields[2])
		if err != nil || count < 0 {
			return stats, fmt.Errorf("coverprofile 计数非法: %q", line)
		}
		stats.totalStmts += numStmt
		if count > 0 {
			stats.coveredStmts += numStmt
		}
	}
	if err := scanner.Err(); err != nil {
		return stats, err
	}
	if first {
		return stats, fmt.Errorf("coverprofile 为空")
	}
	if stats.totalStmts == 0 {
		return stats, fmt.Errorf("coverprofile 无语句（测试未运行?）")
	}
	return stats, nil
}

// compareCoverage 是覆盖率棘轮的纯比较内核（US4）：
// 现值 < 阈值 → 1；≥ → 0（只升不降的收紧由人工上调基线完成）。
func compareCoverage(metric string, pct, min float64, stdout io.Writer) int {
	if pct+1e-9 < min {
		fmt.Fprintf(stdout, "coverage %s: %.1f%% < 阈值 %.0f%% —— 覆盖率棘轮只升不降，请为新增代码补充测试\n",
			metric, pct, min)
		return 1
	}
	fmt.Fprintf(stdout, "coverage %s: %.1f%% ≥ 阈值 %.0f%% ✓ 通过\n", metric, pct, min)
	return 0
}

// runCoverageWithProfile 是 coverage 的可测内核：给定参数、profile 流与已加载基线。
func runCoverageWithProfile(args []string, profile io.Reader, baseline *Baseline) int {
	fs := flag.NewFlagSet("coverage", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	metric := fs.String("metric", "go-statements-min", "基线 coverage 组的 key")
	_ = fs.Parse(args)

	minPct, ok := baseline.Coverage[*metric]
	if !ok {
		fmt.Fprintf(os.Stderr, "coverage: 基线无 coverage.%s 条目\n", *metric)
		return 2
	}

	stats, err := parseCoverProfile(profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "coverage: %v\n", err)
		return 2
	}
	return compareCoverage(*metric, stats.percent(), minPct, os.Stdout)
}

// runCoverageGate 实现 `qualitygate coverage`（contracts Contract 2）：
//
//	coverage --baseline <path> --metric <go-statements-min> [--input <coverprofile>]
//
// stdin（或 --input）读 go test -coverprofile 文件，聚合语句覆盖率并与基线比较（US4）。
// 前端覆盖率门禁内置于 vitest thresholds（data-model Entity 5），不经此命令。
// exit：0 达标 / 1 跌破 / 2 用法或配置错误。
func runCoverageGate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("coverage", flag.ContinueOnError)
	fs.SetOutput(stderr)
	baselinePath := fs.String("baseline", "quality-baseline.json", "基线文件路径")
	metric := fs.String("metric", "go-statements-min", "基线 coverage 组的 key")
	input := fs.String("input", "", "coverprofile 文件（缺省读 stdin）")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	baseline, err := LoadBaseline(*baselinePath)
	if err != nil {
		fmt.Fprintf(stderr, "coverage: %v\n", err)
		return 2
	}
	if _, ok := baseline.Coverage[*metric]; !ok {
		fmt.Fprintf(stderr, "coverage: 基线无 coverage.%s 条目\n", *metric)
		return 2
	}

	var reader io.Reader = os.Stdin
	if *input != "" {
		f, err := os.Open(*input)
		if err != nil {
			fmt.Fprintf(stderr, "coverage: %v\n", err)
			return 2
		}
		defer f.Close()
		reader = f
	}

	stats, err := parseCoverProfile(reader)
	if err != nil {
		fmt.Fprintf(stderr, "coverage: %v\n", err)
		return 2
	}
	return compareCoverage(*metric, stats.percent(), baseline.Coverage[*metric], stdout)
}
