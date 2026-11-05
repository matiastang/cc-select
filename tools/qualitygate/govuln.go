package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// govulnAffectedRe 匹配 govulncheck 摘要行「你的代码可触达 N 个漏洞」。
// 只有该口径阻塞（FR-013：可触达才失败）；"imported/required but not called"
// 属未触达，由 govulncheck 原文输出为警告，不阻断。
var govulnAffectedRe = regexp.MustCompile(`Your code is affected by (\d+) vulnerabilities?`)

// parseGovulnSummary 从 govulncheck 输出提取可触达漏洞数。
// 摘要行缺失视为错误（输出口径变化时宁可失败也不假绿）。
func parseGovulnSummary(output string) (int, error) {
	if strings.Contains(output, "No vulnerabilities found") {
		return 0, nil
	}
	if m := govulnAffectedRe.FindStringSubmatch(output); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, fmt.Errorf("解析漏洞数 %q: %w", m[1], err)
		}
		return n, nil
	}
	return 0, fmt.Errorf("无法识别 govulncheck 输出摘要")
}

// runGovulnGate 实现 `qualitygate govuln`：自执行 `go tool govulncheck <packages>`
// （退出码不可靠——实测有漏洞也 exit 0，故以输出摘要为准）。
//
//	govuln [packages...]           # 缺省 ./...
//
// exit：0 无可触达漏洞 / 1 存在可触达漏洞 / 2 工具或配置错误。
func runGovulnGate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("govuln", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	pkgs := fs.Args()
	if len(pkgs) == 0 {
		pkgs = []string{"./..."}
	}

	cmdArgs := append([]string{"tool", "govulncheck"}, pkgs...)
	cmd := exec.Command("go", cmdArgs...)
	// desktop（wails）源码分析需要 CGO（与 Makefile vet 同因）。
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	out, err := cmd.Output()
	if err != nil {
		// govulncheck 的退出码语义不稳定，非零仅代表工具/环境错误。
		fmt.Fprintf(stderr, "govuln: 工具执行失败: %v\n%s\n", err, strings.TrimSpace(string(out)))
		return 2
	}

	affected, err := parseGovulnSummary(string(out))
	if err != nil {
		fmt.Fprintf(stderr, "govuln: %v\n输出:\n%s\n", err, strings.TrimSpace(string(out)))
		return 2
	}

	if affected > 0 {
		fmt.Fprintf(stdout, "govuln: 可触达漏洞 %d 个（详见上方报告）——升级依赖或工具链后重试\n", affected)
		fmt.Fprintln(stdout, string(out))
		return 1
	}
	fmt.Fprintln(stdout, "govuln: 无可触达漏洞 ✓（未触达的依赖漏洞见报告原文，留待升级消解）")
	return 0
}
