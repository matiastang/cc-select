package cli

import (
	"fmt"

	"github.com/cc-select/cc-select/internal/rcinteg"
	"github.com/cc-select/cc-select/internal/shell"
	"github.com/spf13/cobra"
)

var initShellFlag string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "输出 ccs shell 函数（追加到 shell 启动脚本）",
	Long: `输出 ccs() shell 函数定义，用于注入 cc-select 的命令别名。

用法：
  cc-select init >> ~/.zshrc       # zsh/bash
  cc-select init >> ~/.bashrc      # bash
  # PowerShell: 把输出加入 $PROFILE

source 后即可用 ccs use <provider> 切换当前 shell 的 provider。

ccs 是 cc-select 的短别名：use 走 eval 注入，其余子命令直接转发。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit(cmd)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVar(&initShellFlag, "shell", "",
		"目标 shell（zsh/bash/powershell），默认自动检测")
}

func runInit(cmd *cobra.Command) error {
	// 渲染复用 rcinteg.RenderInit（CLI 与 Web 安装共用，杜绝漂移）。
	snippet, s, err := rcinteg.RenderInit(initShellFlag)
	if err != nil {
		return err
	}

	// 函数定义走 stdout（用户重定向到 rc 文件）；使用提示走 stderr。
	fmt.Fprint(cmd.OutOrStdout(), snippet)
	fmt.Fprintf(cmd.ErrOrStderr(),
		"\n已生成 %s 集成代码。请将其追加到启动脚本并 source：\n", s)
	switch s {
	case shell.PowerShell:
		fmt.Fprintln(cmd.ErrOrStderr(), "  1) 追加到 $PROFILE（若不存在先 New-Item）")
		fmt.Fprintln(cmd.ErrOrStderr(), "  2) 重启 PowerShell 或 . $PROFILE")
	default:
		fmt.Fprintln(cmd.ErrOrStderr(), "  cc-select init >> ~/.zshrc && source ~/.zshrc")
	}
	return nil
}
