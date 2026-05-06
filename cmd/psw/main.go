package main

import (
	_ "github.com/joho/godotenv/autoload"
	"github.com/ylniss/psw/internal/cli"
)

func main() {
	cli.Execute()
}
