// Package cli 是 cc-select 命令行的唯一组装点。
// 它用 cobra 把各子命令挂到 rootCmd 上，业务逻辑下沉到 internal 下其他包。
package cli

import (
	"fmt"
	"os"

	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/prefs"
	"github.com/cc-select/cc-select/internal/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// rootCmd 是 cc-select 的顶层命令。
var rootCmd = &cobra.Command{
	Use:          "cc-select",
	SilenceUsage: true,
}

// initLanguage loads the user's language preference and sets the active locale.
// It is safe to call multiple times.
func initLanguage() {
	pr, _ := prefs.Load()
	i18n.SetLocale(i18n.ResolveLocale(pr.Language))
	retranslateCommands(rootCmd)
}

// localizeCmd stores translation keys for a command's Short/Long text so they can
// be reapplied after the locale is resolved.
func localizeCmd(cmd *cobra.Command, shortKey, longKey string) {
	cmd.Short = i18n.T(shortKey)
	cmd.Long = i18n.T(longKey)
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations["cc-select:shortKey"] = shortKey
	cmd.Annotations["cc-select:longKey"] = longKey
}

// localizeFlag stores the translation key for a flag's usage string so it can be
// reapplied after the locale is resolved.
func localizeFlag(cmd *cobra.Command, name, key string) {
	f := cmd.Flags().Lookup(name)
	if f == nil {
		return
	}
	f.Usage = i18n.T(key)
	if f.Annotations == nil {
		f.Annotations = map[string][]string{}
	}
	f.Annotations["cc-select:key"] = []string{key}
}

// retranslateCommands reapplies translations to a command and all of its
// subcommands/flags after the active locale has changed.
func retranslateCommands(cmd *cobra.Command) {
	if cmd.Annotations != nil {
		if k := cmd.Annotations["cc-select:shortKey"]; k != "" {
			cmd.Short = i18n.T(k)
		}
		if k := cmd.Annotations["cc-select:longKey"]; k != "" {
			cmd.Long = i18n.T(k)
		}
	}
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Annotations != nil {
			if kk := f.Annotations["cc-select:key"]; len(kk) > 0 && kk[0] != "" {
				f.Usage = i18n.T(kk[0])
			}
		}
	})
	for _, c := range cmd.Commands() {
		retranslateCommands(c)
	}
}

// Execute 解析参数并执行对应子命令，返回进程退出码。
func Execute() int {
	initLanguage()
	if err := rootCmd.Execute(); err != nil {
		// cobra 默认会把 usage 打到 stderr，SilenceUsage 已抑制非预期 usage。
		// 错误信息由 cobra 自身打印，这里仅返回非零退出码。
		return 1
	}
	return 0
}

func init() {
	localizeCmd(rootCmd, "cli.root.short", "cli.root.long")
	rootCmd.Version = version.Version
	// cobra 默认会把 version 子命令的模板做格式化，这里保持简洁。
	rootCmd.SetVersionTemplate(fmt.Sprintf("cc-select %s\n", version.Version))
	// 让退出码可被 os.Exit 捕获（cobra 默认 panic 行为不适用）。
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) { initLanguage() }
}
