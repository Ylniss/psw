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
	log.SetFormatter(&easy.Formatter{
		TimestampFormat: "2006-01-02 15:04:05",
		LogFormat:       "%time% [%lvl%]: %msg%",
	})

	logLvl = os.Getenv("PSW_LOG_LEVEL")

	if logLvl == "debug" {
		log.SetLevel(log.DebugLevel)
	}

	err := cmd.SetStoragePaths(os.Getenv("PSW_STORAGE_DIR"))
	if err != nil {
		log.Fatalln(err.Error())
	}

	cmd.SetRecordMarker("!===##$$##$$##$$##$$===!")
}

func main() {
	log.Debug("App started\n")
	cmd.Execute()
	log.Debug("App finished\n")
}
