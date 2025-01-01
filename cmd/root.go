package cmd

import (
	"fmt"
	"os"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	easy "github.com/t-tomalak/logrus-easy-formatter"

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
		log.Debug("App started\n")
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
