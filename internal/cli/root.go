package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/TwiN/go-color"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/internal/menu"
	"github.com/ylniss/psw/internal/storage"
	"golang.org/x/term"
)

// errSilentExit: caller already printed; signals exit 1. cobra silenced in Execute.
var errSilentExit = errors.New("")

var verboseFlag bool

func init() {
	cobra.EnableCommandSorting = false
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "verbose output, sensitive data will be logged")
	rootCmd.AddCommand(getCmd, addCmd, changeCmd, removeCmd, logCmd, rollbackCmd, configCmd, versionCmd)
	// completion before help in --help; cobra defaults to help-first.
	rootCmd.InitDefaultCompletionCmd()
	rootCmd.InitDefaultHelpCmd()
}

var rootCmd = &cobra.Command{
	Use:   "psw",
	Short: "psw is the simplest password management tool",
	Long: `psw is a simple password manager that secures your passwords using AES-256-GCM with Argon2id key derivation.

A 'psw' directory is created under your user config directory (e.g. ~/.config/psw on Linux, %AppData%\psw on Windows) to store:
storage.psw: an encrypted file where your passwords are saved.
pswcfg.toml: a configuration file for customizing app behavior.

On first use, you'll set a main password to protect your stored passwords.
Run 'psw' with no arguments to open the interactive menu. Use 'psw add' to create your first record.`,
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		setupLogger()
		slog.Debug("App started")
		warnEnvPasswordInTTY()
		return storage.InitConfig()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Println(color.InRed("psw requires an interactive terminal"))
			return errSilentExit
		}
		if err := menu.Run(); err != nil {
			fmt.Println(color.InRed(err.Error()))
			return errSilentExit
		}
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		slog.Debug("App finished with errors")
		if err.Error() != "" {
			fmt.Println(err)
		}
		os.Exit(1)
	}
	slog.Debug("App finished")
}

type simpleSlogHandler struct {
	out   io.Writer
	level slog.Level
	mu    sync.Mutex
}

func (h *simpleSlogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *simpleSlogHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	var prefix string
	if !r.Time.IsZero() {
		prefix = r.Time.Format("2006-01-02 15:04:05") + " "
	}
	var attrs strings.Builder
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&attrs, " %s=%v", a.Key, a.Value.Any())
		return true
	})
	_, err := fmt.Fprintf(h.out, "%s[%s]: %s%s\n",
		prefix,
		strings.ToLower(r.Level.String()),
		strings.TrimRight(r.Message, "\n"),
		attrs.String(),
	)
	return err
}

// No-op: binary doesn't use slog.With(); attrs would be silently dropped.
func (h *simpleSlogHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *simpleSlogHandler) WithGroup(_ string) slog.Handler      { return h }

func setupLogger() {
	level := slog.LevelInfo
	if verboseFlag {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(&simpleSlogHandler{out: os.Stderr, level: level}))
}

// warnEnvPasswordInTTY warns when PSW_MAIN_PASSWORD/PSW_NEW_MAIN_PASSWORD is
// set under a TTY (visible via /proc/<pid>/environ).
func warnEnvPasswordInTTY() {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return
	}
	if os.Getenv("PSW_MAIN_PASSWORD") == "" && os.Getenv("PSW_NEW_MAIN_PASSWORD") == "" {
		return
	}
	fmt.Fprintln(os.Stderr, color.InYellow(
		"warning: PSW_MAIN_PASSWORD/PSW_NEW_MAIN_PASSWORD is set in an interactive session; it is visible via /proc/<pid>/environ"))
}
