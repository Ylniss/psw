package cli

import (
	"fmt"
	"os"

	"github.com/TwiN/go-color"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/ylniss/psw/internal/menu"
)

var menuCmd = &cobra.Command{
	Use:   "menu",
	Short: "Interactive launcher for get/add/change/remove",
	Long:  "Show an interactive menu, then run the selected subcommand. Designed for terminals spawned on a hotkey (e.g. foot under niri).",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Println(color.InRed("psw menu requires an interactive terminal"))
			return errSilentExit
		}
		if err := menu.Run(); err != nil {
			fmt.Println(color.InRed(err.Error()))
			return errSilentExit
		}
		return nil
	},
}
