// route.go 是 Mode P 的会话内切换入口（specs/001 contracts/cli.md §1）。
//
// 关键性质：普通子命令——打印结果、写路由表文件，不输出 eval 语句、
// 不触碰调用方 shell 的环境（宪法 I 边界内），因此可在 Claude Code 的
// Bash 工具里直接执行；切换对「下一笔模型请求」生效（FR-001/002）。
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cc-select/cc-select/internal/config"
	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/profile"
	"github.com/cc-select/cc-select/internal/router"
	"github.com/cc-select/cc-select/internal/routes"
	"github.com/spf13/cobra"
)

var (
	routeTIDFlag        string
	routeOlderThanFlag  string
	routePruneOlderFlag string
)

var routeCmd = &cobra.Command{
	Use: "route",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var routeSwitchCmd = &cobra.Command{
	Use:  "switch <provider>",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRouteSwitch(cmd, args[0])
	},
}

var routeListCmd = &cobra.Command{
	Use: "list",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRouteList(cmd)
	},
}

var routeStatusCmd = &cobra.Command{
	Use: "status",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRouteStatus(cmd)
	},
}

var routePruneCmd = &cobra.Command{
	Use: "prune",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRoutePrune(cmd)
	},
}

func init() {
	localizeCmd(routeCmd, "cli.route.short", "cli.route.long")
	localizeCmd(routeSwitchCmd, "cli.route.switch.short", "cli.route.switch.long")
	localizeCmd(routeListCmd, "cli.route.list.short", "cli.route.list.long")
	localizeCmd(routeStatusCmd, "cli.route.status.short", "cli.route.status.long")
	localizeCmd(routePruneCmd, "cli.route.prune.short", "cli.route.prune.long")
	routeCmd.AddCommand(routeSwitchCmd, routeListCmd, routeStatusCmd, routePruneCmd)
	routeSwitchCmd.Flags().StringVar(&routeTIDFlag, "tid", "", "")
	routeStatusCmd.Flags().StringVar(&routeOlderThanFlag, "tid", "", "")
	routePruneCmd.Flags().StringVar(&routePruneOlderFlag, "older-than", "168h", "")
	localizeFlag(routeSwitchCmd, "tid", "cli.route.tidFlag")
	localizeFlag(routeStatusCmd, "tid", "cli.route.tidFlag")
	localizeFlag(routePruneCmd, "older-than", "cli.route.olderThanFlag")
	rootCmd.AddCommand(routeCmd)
}

// resolveRouteTID 解析终端身份：--tid 优先于 env CC_SELECT_TID（claude 的
// Bash 子进程会继承该 env，这正是会话内切换可达的机制，研究 D6）。
func resolveRouteTID() (string, error) {
	tid := routeTIDFlag
	if tid == "" {
		tid = envOrDefault(config.TerminalIDVar)
	}
	if tid == "" {
		return "", errors.New(i18n.T("cli.route.noTID"))
	}
	if !routes.ValidateTID(tid) {
		return "", fmt.Errorf(i18n.T("cli.route.badTID"), tid)
	}
	return tid, nil
}

func runRouteSwitch(cmd *cobra.Command, target string) error {
	tid, err := resolveRouteTID()
	if err != nil {
		return err
	}
	if target == config.OfficialProviderID {
		return errors.New(i18n.T("cli.route.official"))
	}
	cfg, err := appLoadConfig()
	if err != nil {
		return err
	}
	if _, ok := cfg.Providers[target]; !ok {
		return fmt.Errorf(i18n.T("cli.route.unknownProvider"), target)
	}

	old := "∅"
	if e, ok := routes.Get(tid); ok && e.Provider != "" {
		old = e.Provider
	}
	if err := routes.Set(tid, target); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s → %s\n", old, target)

	// 热切刷新选择器（002 US3 / research D3）：经继承的 $CLAUDE_CONFIG_DIR 定位
	// 「发射 profile」的 settings.json（热切后它仍指向发射目录，而非新 provider
	// 的 profile），把 /model 列表刷新为新 provider 的模型清单。best-effort：
	// 失败仅告警——路由表已是真值，选择器显示是增强。
	refreshPickerAfterSwitch(cmd, cfg.Providers[target])
	return nil
}

// refreshPickerAfterSwitch 用目标 provider 的模型计划刷新当前终端的
// settings.json（modelPicker + model）。$CLAUDE_CONFIG_DIR 未设置（非 CC
// 会话上下文）时静默跳过。
func refreshPickerAfterSwitch(cmd *cobra.Command, target config.Provider) {
	dir := os.Getenv("CLAUDE_CONFIG_DIR")
	if dir == "" {
		return
	}
	settingsPath := filepath.Join(dir, "settings.json")
	if _, err := os.Stat(settingsPath); err != nil {
		return // 无 settings.json 的目录不是 profile，不造文件。
	}
	plan := config.ModelPlanFromEnv(target.Env)
	if err := profile.RefreshPicker(settingsPath, plan); err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), i18n.T("profile.refreshFailed", err.Error()))
	}
}

func runRouteList(cmd *cobra.Command) error {
	entries, err := routes.List()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T("cli.route.listEmpty"))
		return nil
	}
	for _, e := range entries {
		fmt.Fprintf(cmd.OutOrStdout(), "%s  %-12s %s\n", shortTID(e.TID), e.Provider, e.UpdatedAt.Local().Format("2006-01-02 15:04"))
	}
	return nil
}

func runRouteStatus(cmd *cobra.Command) error {
	tid, err := resolveRouteTID()
	if err != nil {
		return err
	}
	routerState := "down"
	addr, _ := router.ResolveAddr()
	if _, perr := router.Probe(addr); perr == nil {
		routerState = "ok"
	}
	provider := "∅"
	if e, ok := routes.Get(tid); ok && e.Provider != "" {
		provider = e.Provider
	}
	fmt.Fprintf(cmd.OutOrStdout(), "tid=%s provider=%s router=%s(%s)\n", shortTID(tid), provider, routerState, addr)
	return nil
}

func runRoutePrune(cmd *cobra.Command) error {
	d, err := time.ParseDuration(routePruneOlderFlag)
	if err != nil {
		return fmt.Errorf(i18n.T("cli.route.badDuration"), routePruneOlderFlag)
	}
	if d <= 0 {
		return fmt.Errorf(i18n.T("cli.route.badDuration"), routePruneOlderFlag)
	}
	n, err := routes.Prune(d)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), i18n.T("cli.route.pruned")+"\n", n)
	return nil
}

// shortTID 取 tid 前 12 位短码；对畸形（手改/损坏）条目安全不 panic（评审 #10）。
func shortTID(tid string) string {
	if len(tid) > 12 {
		return tid[:12]
	}
	return tid
}
