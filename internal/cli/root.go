package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/TwiN/go-color"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/internal/storage"
)

// errSilentExit: caller already printed; signals exit 1. cobra silenced in Execute.
var errSilentExit = errors.New("")

var verboseFlag bool

func init() {
	cobra.EnableCommandSorting = false
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "verbose output, sensitive data will be logged")
	rootCmd.AddCommand(getCmd, addCmd, changeCmd, removeCmd, menuCmd, logCmd, rollbackCmd, versionCmd)
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
Run 'psw' with no arguments to list all stored record names.`,
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		setupLogger()
		slog.Debug("App started")
		return storage.InitConfig()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := storage.GetOrCreateForRead()
		if done, ret := handleCmdErr(err); done {
			return ret
		}

		namesAndUsers := store.GetNamesAndUsers()
		if len(namesAndUsers) == 0 {
			fmt.Printf("No secrets found. Use %s command first.\n", color.InCyan("add"))
			return nil
		}

		recordWithLongestName := slices.MaxFunc(namesAndUsers, func(a, b storage.NameAndUser) int {
			return len(a.Name) - len(b.Name)
		})
		longestNameLen := len(recordWithLongestName.Name)

		for _, nameAndUser := range namesAndUsers {
			fmt.Println(formatRecordLine(nameAndUser, longestNameLen))
		}
		return nil
	},
}

func formatRecordLine(nameAndUser storage.NameAndUser, longestNameLen int) string {
	dots := strings.Repeat(".", longestNameLen+5-len(nameAndUser.Name))
	suffix := color.InCyan("<value only>")
	if len(nameAndUser.Username) > 0 {
		suffix = color.InYellow("(" + nameAndUser.Username + ")")
	}
	return color.InGreen(nameAndUser.Name) + dots + suffix
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
