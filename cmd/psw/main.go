package main

import (
	"github.com/awnumar/memguard"

	"github.com/ylniss/psw/internal/cli"
)

func main() {
	memguard.CatchInterrupt()
	defer memguard.Purge()
	cli.Execute()
}
