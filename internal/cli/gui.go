package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/cc-select/cc-select/internal/i18n"
	"github.com/cc-select/cc-select/internal/web"
	"github.com/spf13/cobra"
)

var (
	guiPort      int
	guiNoBrowser bool
)

var guiCmd = &cobra.Command{
	Use: "gui",
	RunE: func(cmd *cobra.Command, args []string) error {
		srv := web.NewServer(guiPort)

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		go func() {
			if err := srv.Start(ctx, func(port int) {
				fmt.Fprintln(cmd.ErrOrStderr(), i18n.T("cli.gui.started", fmt.Sprintf("http://127.0.0.1:%d", port)))
				if !guiNoBrowser {
					if err := web.OpenURL(fmt.Sprintf("http://127.0.0.1:%d", port)); err != nil {
						fmt.Fprintln(cmd.ErrOrStderr(), i18n.T("cli.gui.openBrowserFailed", err))
					}
				}
			}); err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), i18n.T("cli.gui.serverExit", err))
				os.Exit(1)
			}
		}()

		<-ctx.Done()
		fmt.Fprintln(cmd.ErrOrStderr(), "\n"+i18n.T("cli.gui.stopping"))
		return nil
	},
}

func init() {
	localizeCmd(guiCmd, "cli.gui.short", "cli.gui.long")
	rootCmd.AddCommand(guiCmd)
	guiCmd.Flags().IntVar(&guiPort, "port", 7799, "")
	guiCmd.Flags().BoolVar(&guiNoBrowser, "no-browser", false, "")
	localizeFlag(guiCmd, "port", "cli.gui.portFlag")
	localizeFlag(guiCmd, "no-browser", "cli.gui.noOpenFlag")
}
