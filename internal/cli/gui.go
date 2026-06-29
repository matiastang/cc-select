package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/cc-select/cc-select/internal/web"
	"github.com/spf13/cobra"
)

var (
	guiPort      int
	guiNoBrowser bool
)

var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "启动本地 Web 配置页（浏览器打开）",
	Long: `启动本地 Web 配置页，通过浏览器可视化管理 provider。

仅监听 127.0.0.1，配置通过 REST API 读写与 CLI 共享的 JSON。
按 Ctrl+C 停止服务。

注意：在 GUI 改配置是改"模板"，已在运行的终端需重新 ccs use 才生效。
（详见 docs/architecture.md §5 配置生效语义）`,
	RunE: func(cmd *cobra.Command, args []string) error {
		srv := web.NewServer(guiPort)

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		go func() {
			if err := srv.Start(ctx, func(port int) {
				fmt.Fprintf(cmd.ErrOrStderr(), "cc-select 配置页已启动：%s\n", fmt.Sprintf("http://127.0.0.1:%d", port))
				if !guiNoBrowser {
					if err := web.OpenURL(fmt.Sprintf("http://127.0.0.1:%d", port)); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "（未能自动打开浏览器：%v，请手动访问上述地址）\n", err)
					}
				}
			}); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "服务退出：%v\n", err)
				os.Exit(1)
			}
		}()

		<-ctx.Done()
		fmt.Fprintln(cmd.ErrOrStderr(), "\n正在停止…")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(guiCmd)
	guiCmd.Flags().IntVar(&guiPort, "port", 7799, "监听端口（0=系统分配）")
	guiCmd.Flags().BoolVar(&guiNoBrowser, "no-browser", false, "不自动打开浏览器")
}
