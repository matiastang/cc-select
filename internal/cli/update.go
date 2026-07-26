package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/updater"
	"github.com/spf13/cobra"
)

var (
	updateCheck      bool
	updateDryRun     bool
	updateForce      bool
	updatePrerelease bool
)

var updateCmd = &cobra.Command{
	Use: "update",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUpdate(cmd)
	},
}

func init() {
	localizeCmd(updateCmd, "cli.update.short", "cli.update.long")
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().BoolVar(&updateCheck, "check", false, "")
	updateCmd.Flags().BoolVar(&updateDryRun, "dry-run", false, "")
	updateCmd.Flags().BoolVar(&updateForce, "force", false, "")
	updateCmd.Flags().BoolVar(&updatePrerelease, "prerelease", false, "")
	localizeFlag(updateCmd, "check", "cli.update.checkFlag")
	localizeFlag(updateCmd, "dry-run", "cli.update.dryRunFlag")
	localizeFlag(updateCmd, "force", "cli.update.forceFlag")
	localizeFlag(updateCmd, "prerelease", "cli.update.prereleaseFlag")
}

func runUpdate(cmd *cobra.Command) error {
	// Ctrl+C 可中断下载（同 gui.go 的 signal 处理）。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()
	opts := updater.Options{
		AllowPrerelease: updatePrerelease,
		Force:           updateForce,
		DryRun:          updateDryRun,
		GitHubToken:     updater.GitHubTokenFromEnv(),
		Logf:            func(msg string) { fmt.Fprintln(errOut, msg) },
	}

	if updateCheck {
		fmt.Fprintln(errOut, i18n.T("cli.update.checking"))
		res, err := updater.Check(ctx, opts)
		if err != nil {
			return err
		}
		if res.DevBuild {
			fmt.Fprintln(errOut, i18n.T("errors.update.devBuild"))
			return nil
		}
		if res.HasUpdate {
			fmt.Fprintln(out, i18n.T("cli.update.available", res.Current, res.Latest))
			if res.AssetName == "" {
				fmt.Fprintln(errOut, i18n.T("errors.update.noAssetForPlatform", runtime.GOOS, runtime.GOARCH))
			}
		} else {
			fmt.Fprintln(out, i18n.T("cli.update.upToDate", res.Current, res.Latest))
		}
		return nil
	}

	result, err := updater.Run(ctx, opts)
	if err != nil {
		var refused *updater.RefusedError
		if errors.As(err, &refused) {
			// 策略拒绝（dev/brew/scoop/不可写）不是失败：提示后正常退出。
			fmt.Fprintln(errOut, refused.Error())
			return nil
		}
		return err
	}
	fmt.Fprintln(out, result.Message)
	if result.Installed {
		fmt.Fprintln(errOut, i18n.T("cli.update.restartHint"))
	}
	return nil
}
