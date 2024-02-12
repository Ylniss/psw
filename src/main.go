package main

import (
	"os"

	"github.com/ylniss/psw/cmd"

	log "github.com/sirupsen/logrus"
	easy "github.com/t-tomalak/logrus-easy-formatter"

	_ "github.com/joho/godotenv/autoload"
)

var (
	logLvl      string
	storagePath string
)

func init() {
	logLvl = os.Getenv("PSW_LOG_LEVEL")
	storagePath = os.Getenv("PSW_STORAGE_DIR")
}

func main() {
	log.SetFormatter(&easy.Formatter{
		TimestampFormat: "2006-01-02 15:04:05",
		LogFormat:       "%time% [%lvl%]: %msg%",
	})

	if logLvl == "debug" {
		log.SetLevel(log.DebugLevel)
	}

	log.Debug("App started\n")
	cmd.Execute()
	log.Debug("App finished\n")
}
