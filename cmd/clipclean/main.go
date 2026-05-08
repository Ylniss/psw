package main

import (
	"os"
	"strconv"
	"time"

	"github.com/atotto/clipboard"
)

func main() {
	if len(os.Args) < 2 {
		return // incorrect arguments number
	}
	duration, err := strconv.Atoi(os.Args[1])
	if err != nil {
		return // duration in incorrect format
	}
	if duration <= 0 {
		return // non-positive duration: nothing to wait for
	}

	originalClip, err := clipboard.ReadAll()
	if err != nil {
		return // failed to read clipboard
	}

	time.Sleep(time.Duration(duration) * time.Second)

	currentClip, err := clipboard.ReadAll()
	if err != nil {
		return // failed to read clipboard
	}

	if currentClip == originalClip {
		_ = clipboard.WriteAll("") // clear clipboard
	}
}
