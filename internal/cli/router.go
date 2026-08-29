// router.go 是 Mode P 路由 daemon 的生命周期命令（specs/001 contracts/cli.md §2）：
//
//   - ensure：幂等确保 daemon 在位（健康复用 / 死亡或版本不匹配则拉起）；
//   - serve：daemon 入口（--foreground 前台排障；默认 = ensure 语义）；
//   - stop：经停止侧信道优雅退出（幂等）。
//
// use/route（proxy 模式）内部同样走 ensure——崩溃自愈后无需重启 claude 会话（FR-011）。
package cli

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/router"
	"github.com/cc-select/cc-select/internal/secrets"
	"github.com/cc-select/cc-select/internal/version"
	"github.com/spf13/cobra"
)

var routerForegroundFlag bool

var routerCmd = &cobra.Command{
	Use: "router",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var routerEnsureCmd = &cobra.Command{
	Use: "ensure",
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, err := ensureRouter()
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), i18n.T("cli.router.running")+"\n", addr)
		return nil
	},
}

var routerServeCmd = &cobra.Command{
	Use: "serve",
	RunE: func(cmd *cobra.Command, args []string) error {
		if routerForegroundFlag {
			return serveForeground()
		}
		// 默认 = 拉起 detached daemon 并等待就绪（ensure 语义）。
		addr, err := ensureRouter()
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), i18n.T("cli.router.running")+"\n", addr)
		return nil
	},
}

var routerStopCmd = &cobra.Command{
	Use: "stop",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRouterStop(cmd)
	},
}

func init() {
	localizeCmd(routerCmd, "cli.router.short", "cli.router.long")
	localizeCmd(routerEnsureCmd, "cli.router.ensure.short", "cli.router.ensure.long")
	localizeCmd(routerServeCmd, "cli.router.serve.short", "cli.router.serve.long")
	localizeCmd(routerStopCmd, "cli.router.stop.short", "cli.router.stop.long")
	routerServeCmd.Flags().BoolVar(&routerForegroundFlag, "foreground", false, "")
	localizeFlag(routerServeCmd, "foreground", "cli.router.foregroundFlag")
	routerCmd.AddCommand(routerEnsureCmd, routerServeCmd, routerStopCmd)
	rootCmd.AddCommand(routerCmd)
}

// ensureRouter 幂等确保 daemon 在位（研究 D7）。Spawn 失败/超时的错误文本
// 自带恢复指引（state.go Ensure），直接透传给用户（FR-011）。
func ensureRouter() (string, error) {
	deps := router.EnsureDeps{
		Spawn:       router.SpawnDetached,
		SelfVersion: version.Version,
	}
	return deps.Ensure()
}

// ensureRouterFn 是 use（proxy 模式）的注入点：测试替换为桩，避免真拉 daemon。
var ensureRouterFn = ensureRouter

// serveForeground 前台运行 daemon（排障用）：auth → model 改写 → keychain 解析 → 转发。
func serveForeground() error {
	pipeline := router.ModelRewrite(router.NewForward(router.NewKeychainResolver(secrets.New())))
	addr, err := router.ResolveAddr()
	if err != nil {
		return err
	}
	s := router.NewServer(addr, version.Version, pipeline)
	ln, err := s.Listen()
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "router listening on http://%s (version %s)\n", ln.Addr().String(), version.Version)
	return s.Serve(ln)
}

// runRouterStop 经停止侧信道优雅退出 daemon（幂等；无状态 = 未运行）。
func runRouterStop(cmd *cobra.Command) error {
	st, err := router.LoadState()
	if err != nil {
		return err
	}
	if st == nil || st.Addr == "" {
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T("cli.router.notRunning"))
		return nil
	}
	req, err := http.NewRequest(http.MethodPost, "http://"+st.Addr+"/healthz", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-CC-Select-Stop", st.StopToken)
	client := &http.Client{Timeout: 5 * time.Second} // 无响应 daemon 不得挂死命令（评审 #9）
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf(i18n.T("cli.router.stopFailed"), st.Addr, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(i18n.T("cli.router.stopFailed"), st.Addr, resp.Status)
	}
	fmt.Fprintf(cmd.OutOrStdout(), i18n.T("cli.router.stopped")+"\n", st.Addr)
	return nil
}
