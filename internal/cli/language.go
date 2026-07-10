package cli

import (
	"fmt"

	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/prefs"
	"github.com/spf13/cobra"
)

var languageCmd = &cobra.Command{
	Use:  "language [en|zh]",
	Args: cobra.MaximumNArgs(1),
	RunE: runLanguage,
}

func runLanguage(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T("cli.language.current", i18n.CurrentLocale()))
		return nil
	}

	raw := args[0]
	l := i18n.NormalizeLocale(raw)
	if !i18n.IsSupportedLocale(string(l)) {
		return fmt.Errorf(i18n.T("cli.language.invalid"), raw)
	}

	pr, err := prefs.Load()
	if err != nil {
		return err
	}
	pr.Language = string(l)
	if err := prefs.Save(pr); err != nil {
		return err
	}

	i18n.SetLocale(l)
	retranslateCommands(rootCmd)
	fmt.Fprintln(cmd.OutOrStdout(), i18n.T("cli.language.set", l))
	return nil
}

func init() {
	localizeCmd(languageCmd, "cli.language.short", "cli.language.long")
	rootCmd.AddCommand(languageCmd)
}
