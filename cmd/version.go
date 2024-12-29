package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var Version string

func init() {
	versionFilePath := filepath.Join("..", "VERSION")

	content, err := ioutil.ReadFile(versionFilePath)
	if err != nil {
		log.Fatalf("Failed to read version file: %v", err)
	}

	Version = strings.TrimSpace(string(content))

	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of psw",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(fmt.Sprintf("psw v%s", Version))
	},
}
