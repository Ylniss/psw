package cli

import color "github.com/TwiN/go-color"

// TwiN/go-color's color_windows.go init() zeros every ANSI escape string when
// stdout is not a console handle (pipes, redirects). psw always emits colors
// regardless of stdout type — matches the Linux build, which has no such gate.
// Restore the defaults; runs after the library's init via import order.
func init() {
	color.Reset = "\033[0m"
	color.Bold = "\033[1m"
	color.Underline = "\033[4m"
	color.Black = "\033[30m"
	color.Red = "\033[31m"
	color.Green = "\033[32m"
	color.Yellow = "\033[33m"
	color.Blue = "\033[34m"
	color.Purple = "\033[35m"
	color.Cyan = "\033[36m"
	color.Gray = "\033[37m"
	color.White = "\033[97m"
}
