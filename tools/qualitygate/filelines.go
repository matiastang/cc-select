package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// fileLinesViolation 是一条文件行数超限（FR-003/FR-007：报告含路径、实测、阈值）。
type fileLinesViolation struct {
	path  string
	lines int
	max   int
}

// fileLinesOptions 是 filelines 子命令的解析结果。
// 字段持 flag 指针：值在 Parse 时写入，注册时解引用会拿到默认值快照（真实踩过的坑）。
type fileLinesOptions struct {
	root         *string
	max          *int
	excludes     fsMultiFlag
	baselinePath *string
}

// parseFileLinesFlags 解析 filelines 参数（与扫描逻辑分离，控制圈复杂度）。
func parseFileLinesFlags(args []string, stderr io.Writer) (*fileLinesOptions, int) {
	fs := flag.NewFlagSet("filelines", flag.ContinueOnError)
	fs.SetOutput(stderr)
	opts := &fileLinesOptions{
		root:         fs.String("root", ".", "扫描根目录"),
		max:          fs.Int("max", 1000, "单文件行数上限"),
		baselinePath: fs.String("baseline", "", "基线文件路径（可选）"),
	}
	fs.Var(&opts.excludes, "exclude", "排除的目录（相对 root，可多次）")
	if err := fs.Parse(args); err != nil {
		return nil, 2
	}
	if *opts.max <= 0 {
		fmt.Fprintf(stderr, "filelines: --max 必须为正数，got %d\n", *opts.max)
		return nil, 2
	}
	return opts, 0
}

// runFileLines 实现 `qualitygate filelines`（contracts Contract 2）：
//
//	filelines --root <dir> --max <N> [--exclude <dir>]... [--baseline <path>]
//
// 遍历 .go 产品文件（_test.go 天然豁免），行数 > max 报违规；基线 filelines
// 组可整文件豁免（存量棘轮，US3）。exit：0 通过 / 1 有违规 / 2 配置错误。
func runFileLines(args []string, stdout, stderr io.Writer) int {
	opts, code := parseFileLinesFlags(args, stderr)
	if code != 0 {
		return code
	}

	info, err := os.Stat(*opts.root)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(stderr, "filelines: root %q 不存在或不是目录\n", *opts.root)
		return 2
	}

	var baseline *Baseline
	if *opts.baselinePath != "" {
		baseline, err = LoadBaseline(*opts.baselinePath)
		if err != nil {
			fmt.Fprintf(stderr, "filelines: %v\n", err)
			return 2
		}
	}

	violations, err := collectFileLinesViolations(opts, baseline)
	if err != nil {
		fmt.Fprintf(stderr, "filelines: %v\n", err)
		return 2
	}
	return reportFileLines(violations, *opts.max, stderr)
}

// collectFileLinesViolations 遍历 root 收集超限文件（基线豁免不计入）。
func collectFileLinesViolations(opts *fileLinesOptions, baseline *Baseline) ([]fileLinesViolation, error) {
	var violations []fileLinesViolation
	err := filepath.WalkDir(*opts.root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(*opts.root, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() {
			if rel != "." && excludedDir(rel, opts.excludes) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		lines, err := countLines(path)
		if err != nil {
			return err
		}
		if lines <= *opts.max {
			return nil
		}
		if baseline != nil && baseline.HasFinding("filelines", Finding{
			File: filepath.ToSlash(rel), Line: 0, Rule: "file-lines",
		}) {
			return nil
		}
		violations = append(violations, fileLinesViolation{path: filepath.ToSlash(rel), lines: lines, max: *opts.max})
		return nil
	})
	return violations, err
}

// reportFileLines 输出违规报告（stderr 供人读；exit code 是门禁真值）。
func reportFileLines(violations []fileLinesViolation, max int, stderr io.Writer) int {
	if len(violations) == 0 {
		return 0
	}
	sort.Slice(violations, func(i, j int) bool { return violations[i].path < violations[j].path })
	fmt.Fprintf(stderr, "文件行数超限 %d 个（max=%d）：\n", len(violations), max)
	for _, v := range violations {
		fmt.Fprintf(stderr, "  %s: %d 行 > %d\n", v.path, v.lines, v.max)
	}
	fmt.Fprintf(stderr, "修复：拆分文件；存量豁免见 docs/quality-gates.md「修基线」。\n")
	return 1
}

// excludedDir 判断相对路径 rel 是否落在排除目录内（前缀匹配目录边界）。
func excludedDir(rel string, excludes []string) bool {
	rel = filepath.ToSlash(rel)
	for _, ex := range excludes {
		ex = strings.TrimSuffix(filepath.ToSlash(ex), "/") + "/"
		if strings.HasPrefix(rel+"/", ex) {
			return true
		}
	}
	return false
}

// countLines 统计文件行数：'\n' 计数 + 无尾换行的最后一行（与 wc -l 差一行时后者补齐）。
func countLines(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	n := strings.Count(string(data), "\n")
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		n++
	}
	return n, nil
}

// fsMultiFlag 收集可重复的 string flag。
type fsMultiFlag []string

func (m *fsMultiFlag) String() string { return strings.Join(*m, ",") }
func (m *fsMultiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}
