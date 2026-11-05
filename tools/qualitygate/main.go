// qualitygate：代码质量门禁工具（specs/005-quality-gates，需求 R12）。
//
// 子命令（contracts/quality-gates.md Contract 2）：
//   - filelines：Go 产品代码文件行数检查
//   - filter：   linter 输出减去基线（存量违规豁免，棘轮载体）
//   - ratchet：  jscpd/knip 计数棘轮
//   - coverage： Go 覆盖率下限
//
// 退出码约定：0 通过 / 1 门禁失败 / 2 用法或配置错误。
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var code int
	switch os.Args[1] {
	case "filelines":
		code = runFileLines(os.Args[2:], os.Stdout, os.Stderr)
	case "golangci":
		code = runGolangciGate(os.Args[2:], os.Stdout, os.Stderr)
	case "eslint":
		code = runESLintGate(os.Args[2:], os.Stdout, os.Stderr)
	case "ratchet":
		code = runRatchet(os.Args[2:], os.Stdout, os.Stderr)
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "qualitygate: 未知子命令 %q\n", os.Args[1])
		usage()
		code = 2
	}
	os.Exit(code)
}

func usage() {
	fmt.Fprintln(os.Stderr, `qualitygate — 代码质量门禁工具（specs/005-quality-gates）

用法:
  qualitygate filelines --root <dir> --max <N> [--exclude <dir>]... [--baseline <path>]
      Go 产品代码（跳过 _test.go）单文件行数检查。

  qualitygate golangci --baseline <path> [--new-from-rev <rev>] [--packages <pkgs>]
      自执行 go tool golangci-lint run，解析输出并按基线过滤（杜绝管道假绿）。

退出码: 0 通过 / 1 门禁失败 / 2 用法或配置错误。`)
}
