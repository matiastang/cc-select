package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
)

// frontendDir 是前端 npm 上下文（eslint 配置与 node_modules 所在）。
const frontendDir = "internal/frontend"

// eslintMessage 是 ESLint --format JSON 输出中的单条消息（只取所需字段）。
type eslintMessage struct {
	RuleID   string `json:"ruleId"`
	Severity int    `json:"severity"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
}

// eslintFileResult 是 ESLint JSON 输出中的单文件结果。
type eslintFileResult struct {
	FilePath string          `json:"filePath"`
	Messages []eslintMessage `json:"messages"`
}

// parseESLintJSON 解析 ESLint JSON 输出为基线同构的 findings：
// 只收 severity=2（error；warning 永不阻塞门禁），filePath 归一化为相对
// baseDir 的斜杠路径（与基线 eslint 组一致）。baseDir 必须是绝对路径；
// 路径不在 baseDir 下属于解析失败（显式报错，绝不静默丢弃——丢弃即假绿）。
func parseESLintJSON(r io.Reader, baseDir string) ([]Finding, error) {
	var results []eslintFileResult
	if err := json.NewDecoder(r).Decode(&results); err != nil {
		return nil, fmt.Errorf("解析 eslint JSON: %w", err)
	}

	var findings []Finding
	for _, res := range results {
		if !filepath.IsAbs(res.FilePath) {
			res.FilePath = filepath.Join(baseDir, res.FilePath)
		}
		rel, err := filepath.Rel(baseDir, res.FilePath)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil, fmt.Errorf("eslint 输出路径 %q 不在 baseDir %q 下", res.FilePath, baseDir)
		}
		for _, m := range res.Messages {
			if m.Severity != 2 {
				continue
			}
			findings = append(findings, Finding{
				File:    filepath.ToSlash(rel),
				Line:    m.Line,
				Rule:    m.RuleID,
				Message: m.Message,
			})
		}
	}
	return findings, nil
}

// frontendStagedFiles 从 git staged 列表过滤前端源文件（相对 internal/frontend）。
func frontendStagedFiles(staged []string) []string {
	var out []string
	for _, f := range staged {
		if rel := strings.TrimPrefix(f, frontendDir+"/"); rel != f && strings.HasPrefix(rel, "src/") {
			out = append(out, rel)
		}
	}
	return out
}

// frontendAbsDir 返回 frontendDir 的绝对路径（qualitygate 由 make 从仓库根调用）。
func frontendAbsDir() string {
	abs, err := filepath.Abs(frontendDir)
	if err != nil {
		return frontendDir
	}
	return abs
}

// runESLintGate 实现 `qualitygate eslint`：自执行 eslint（不走 shell 管道，
// 退出码语义：0 干净/warning、1 有 error、2 配置致命错误），解析并按基线过滤。
//
//	qualitygate eslint --baseline <path> [--staged]
//
// --staged 只检查 git staged 的前端源文件（pre-commit 增量；无 staged 前端文件直接通过）。
// exit：0 通过 / 1 存在基线外违规 / 2 工具或配置错误。
func runESLintGate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("eslint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	baselinePath := fs.String("baseline", "quality-baseline.json", "基线文件路径")
	staged := fs.Bool("staged", false, "只检查 git staged 的前端源文件（pre-commit）")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	baseline, err := LoadBaseline(*baselinePath)
	if err != nil {
		fmt.Fprintf(stderr, "eslint: %v\n", err)
		return 2
	}

	files := []string{"src"}
	if *staged {
		stagedOut, err := exec.Command("git", "diff", "--cached", "--name-only",
			"--diff-filter=ACMR", "--", frontendDir+"/src").Output()
		if err != nil {
			fmt.Fprintf(stderr, "eslint: 读取 staged 列表失败: %v\n", err)
			return 2
		}
		files = frontendStagedFiles(strings.Split(string(stagedOut), "\n"))
		if len(files) == 0 {
			fmt.Fprintln(stdout, "eslint: 无 staged 前端源文件，跳过")
			return 0
		}
	}

	eslintArgs := append([]string{
		filepath.Join("node_modules", "eslint", "bin", "eslint.js"),
	}, files...)
	eslintArgs = append(eslintArgs, "--format", "json", "--no-warn-ignored")
	cmd := exec.Command("node", eslintArgs...)
	// eslint 的 flat config 与文件参数都相对 frontend 目录解析，cwd 必须是 frontend。
	cmd.Dir = frontendDir
	out, err := cmd.Output()
	if err != nil {
		// eslint 约定：exit 0/1 是正常结论，其余（2+）是配置/致命错误。
		exitErr, ok := err.(*exec.ExitError)
		if !ok || exitErr.ExitCode() > 1 {
			fmt.Fprintf(stderr, "eslint: 工具执行失败（node_modules 未安装?）: %v\n%s\n",
				err, strings.TrimSpace(string(out)))
			return 2
		}
	}

	findings, err := parseESLintJSON(strings.NewReader(string(out)), frontendAbsDir())
	if err != nil {
		fmt.Fprintf(stderr, "eslint: %v\n", err)
		return 2
	}

	return reportFindings("eslint", findings, baseline, stdout, stderr)
}
