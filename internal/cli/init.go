package cli

import (
	"fmt"

	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/rcinteg"
	"github.com/cc-select/cc-select/internal/shell"
	"github.com/spf13/cobra"
)

var initShellFlag string

var initCmd = &cobra.Command{
	Use: "init",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit(cmd)
	},
}

func init() {
	localizeCmd(initCmd, "cli.init.short", "cli.init.long")
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVar(&initShellFlag, "shell", "", "")
	localizeFlag(initCmd, "shell", "cli.init.shellFlag")
}

func runInit(cmd *cobra.Command) error {
	// 渲染复用 rcinteg.RenderInit（CLI 与 Web 安装共用，杜绝漂移）。
	snippet, s, err := rcinteg.RenderInit(initShellFlag)
	if err != nil {
		return err
	}

	// 函数定义走 stdout（用户重定向到 rc 文件）；使用提示走 stderr。
	fmt.Fprint(cmd.OutOrStdout(), snippet)
	fmt.Fprintln(cmd.ErrOrStderr(), "\n"+i18n.T("cli.init.generated", s))
	switch s {
	case shell.PowerShell:
		fmt.Fprintln(cmd.ErrOrStderr(), i18n.T("cli.init.powershell.line1"))
		fmt.Fprintln(cmd.ErrOrStderr(), i18n.T("cli.init.powershell.line2"))
	default:
		fmt.Fprintln(cmd.ErrOrStderr(), i18n.T("cli.init.zsh"))
	}
	return nil
}
