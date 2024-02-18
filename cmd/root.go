package cmd

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	easy "github.com/t-tomalak/logrus-easy-formatter"

	"github.com/TwiN/go-color"
	"github.com/spf13/cobra"
	"github.com/ylniss/psw/strg"
)

var verboseFlag bool

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "verbose output")
}

var rootCmd = &cobra.Command{
	Use: `psw         lists all record names
  psw`,
	Short: "psw is a simple password manager",
	Long: `psw is a simple password manager that employs AES SHA256 encryption to secure your passwords.
It consolidates your passwords into a single file, safeguarded by a main password you choose.
The initial interaction with any command prompts the setup of this main password. For added flexibility,
you can customize the storage file's default directory by setting the PSW_STORAGE_DIR environment variable
in your shell configuration file.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupLogger()
		log.Debug("App started\n")
	},
	Run: func(cmd *cobra.Command, args []string) {
		// list all record names on 'psw' command
		storage, err := strg.GetOrCreateIfNotExists()
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		names := storage.GetNames()
		if len(names) == 0 {
			fmt.Println("No secrets found. Use 'add' command first.")
			return
		}

		for _, name := range names {
			fmt.Println(color.InGreen(name))
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Debug("App finished with errors\n")
		fmt.Println(err)
		os.Exit(1)
	}
	log.Debug("App finished\n")
}

func setupLogger() {
	log.SetFormatter(&easy.Formatter{
		TimestampFormat: "2006-01-02 15:04:05",
		LogFormat:       "%time% [%lvl%]: %msg%",
	})

	if verboseFlag {
		log.SetLevel(log.DebugLevel)
	}
}
