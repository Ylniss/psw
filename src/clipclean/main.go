package main

import (
	"os"
	"strconv"
	"time"

	"golang.design/x/clipboard"
)

// separate application that is run after clipping password to clean after duration
func main() {
	duration, err := strconv.Atoi(os.Args[1])
	if err != nil {
		return // duration in incorrect format
	}
	pass := os.Args[2]

	if len(os.Args) < 3 {
		return // incorrect arguments number
	}

	time.Sleep(time.Duration(duration) * time.Second)
	curClip := clipboard.Read(clipboard.FmtText)
	if string(curClip) == pass {
		clipboard.Write(clipboard.FmtText, []byte(""))
	}
}
