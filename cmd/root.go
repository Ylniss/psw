package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/samber/lo"

	"github.com/TwiN/go-color"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/strg"
)

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
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupLogger()
		slog.Debug("App started")
		strg.InitConfig()
	},
	Run: func(cmd *cobra.Command, args []string) {
		// list all record names on 'psw' command
		storage, err := strg.GetOrCreateIfNotExists()
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		namesAndUsers := storage.GetNamesAndUsers()
		if len(namesAndUsers) == 0 {
			fmt.Printf("No secrets found. Use %s command first.\n", color.InCyan("add"))
			return
		}

		longestNameLen := len(lo.MaxBy(namesAndUsers, func(a strg.NameAndUser, b strg.NameAndUser) bool {
			return len(a.Name) > len(b.Name)
		}).Name)

		dotsPrinter := func(numOfDots int) string {
			startString := ""
			for i := 0; i < numOfDots; i++ {
				startString += "."
			}
			return startString
		}

		for _, nameAndUser := range namesAndUsers {
			currentNameLen := len(nameAndUser.Name)
			dotsToPrint := longestNameLen + 5 - currentNameLen
			if len(nameAndUser.User) > 0 {
				fmt.Println(color.InGreen(nameAndUser.Name) + dotsPrinter(dotsToPrint) + color.InYellow("("+nameAndUser.User+")"))
			} else {
				fmt.Println(color.InGreen(nameAndUser.Name) + dotsPrinter(dotsToPrint) + color.InCyan("<value only>"))
			}
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		slog.Debug("App finished with errors")
		fmt.Println(err)
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
	_, err := fmt.Fprintf(h.out, "%s [%s]: %s\n",
		r.Time.Format("2006-01-02 15:04:05"),
		strings.ToLower(r.Level.String()),
		strings.TrimRight(r.Message, "\n"),
	)
	return err
}

func (h *easyHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *easyHandler) WithGroup(_ string) slog.Handler      { return h }

func setupLogger() {
	level := slog.LevelInfo
	if verboseFlag {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(&easyHandler{out: os.Stderr, level: level}))
}
