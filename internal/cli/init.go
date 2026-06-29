package cli

import (
	"fmt"
	"os"
	"path/filepath"

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
	// 取 cc-select 自身绝对路径，写入函数体（避免依赖 PATH/别名）。
	bin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("定位 cc-select 可执行文件: %w", err)
	}
	// 解析符号链接拿到真实路径；失败时保留原路径（EvalSymlinks 出错会返回空串，
	// 故必须用临时变量承接，不能直接覆盖 bin）。
	if resolved, err := filepath.EvalSymlinks(bin); err == nil {
		bin = resolved
	}

	s := shell.Shell(initShellFlag)
	if s == shell.Unknown {
		s = shell.Detect()
	}
	emitter, err := shell.For(s)
	if err != nil {
		return err
	}

	// 函数定义走 stdout（用户重定向到 rc 文件）；使用提示走 stderr。
	fmt.Fprint(cmd.OutOrStdout(), emitter.InitSnippet(bin))
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
