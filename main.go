package main

import (
	_ "github.com/joho/godotenv/autoload"
	"github.com/ylniss/psw/cmd"
)

func main() {
	cmd.SetupLogger()
	cmd.Execute()
}
