package cli

import "github.com/TwiN/go-color"

// go-color zeros ANSI escapes on Windows when stdout isn't a console.
// psw wants colors regardless of stdout type. Restore the ones psw uses.
func init() {
	color.Reset = "\033[0m"
	color.Red = "\033[31m"
	color.Green = "\033[32m"
	color.Yellow = "\033[33m"
	color.Purple = "\033[35m"
	color.Cyan = "\033[36m"
	color.Gray = "\033[37m"
}
