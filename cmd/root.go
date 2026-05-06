package cmd

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
	"github.com/ylniss/psw/strg"
)

// errExit: caller already printed; signals exit 1. cobra silenced in Execute.
var errExit = errors.New("")

var verboseFlag bool

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "verbose output, sensitive data will be logged")
}

var rootCmd = &cobra.Command{
	Use: `psw        lists all stored record names
  psw`,
	Short: "psw is the simplest password management tool",
	Long: `psw is a simple password manager that secures your passwords using AES encryption with SHA256.

The directory ~/.psw is created to store all necessary files:
storage.psw: an encrypted file where your passwords are saved.
pswcfg.toml: a configuration file for customizing app behavior.

On first use, you’ll set a main password to protect your stored passwords.`,
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupLogger()
		slog.Debug("App started")
		strg.InitConfig()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// list all record names on 'psw' command
		storage, err := strg.GetOrCreateIfNotExists()
		if err != nil {
			fmt.Println(err.Error())
			return nil
		}

		namesAndUsers := storage.GetNamesAndUsers()
		if len(namesAndUsers) == 0 {
			fmt.Printf("No secrets found. Use %s command first.\n", color.InCyan("add"))
			return nil
		}

		longest := slices.MaxFunc(namesAndUsers, func(a, b strg.NameAndUser) int {
			return len(a.Name) - len(b.Name)
		})
		longestNameLen := len(longest.Name)

		for _, nameAndUser := range namesAndUsers {
			fmt.Println(formatRecordLine(nameAndUser, longestNameLen))
		}
		return nil
	},
}

func formatRecordLine(nu strg.NameAndUser, longestNameLen int) string {
	dots := strings.Repeat(".", longestNameLen+5-len(nu.Name))
	suffix := color.InCyan("<value only>")
	if len(nu.User) > 0 {
		suffix = color.InYellow("(" + nu.User + ")")
	}
	return color.InGreen(nu.Name) + dots + suffix
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

type easyHandler struct {
	out   io.Writer
	level slog.Level
	mu    sync.Mutex
}

func (h *easyHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *easyHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	var prefix string
	if !r.Time.IsZero() {
		prefix = r.Time.Format("2006-01-02 15:04:05") + " "
	}
	_, err := fmt.Fprintf(h.out, "%s[%s]: %s\n",
		prefix,
		strings.ToLower(r.Level.String()),
		strings.TrimRight(r.Message, "\n"),
	)
	return err
}

// No-op: binary doesn't use slog.With(); attrs would be silently dropped.
func (h *easyHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *easyHandler) WithGroup(_ string) slog.Handler      { return h }

func setupLogger() {
	level := slog.LevelInfo
	if verboseFlag {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(&easyHandler{out: os.Stderr, level: level}))
}
