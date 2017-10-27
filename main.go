package main

import (
	"flag"
	"github.com/1and1internet/supervisorgo/managed_procs"
	"os"
	"log"
	"fmt"
)

func main() {
	var supervisorConf = flag.String(
		"c",
		"/etc/supervisor/supervisord.conf",
		"The master config file. Default is /etc/supervisor/supervisord.conf")

	var nodaemon = flag.Bool(
		"n",
		false,
		"Run in foreground (no daemon)")

	var loglevel string
	flag.StringVar(
		&loglevel,
		"e",
		"error",
		"The log level. Valid levels are trace, debug, info, warn, error, and critical")
	flag.StringVar(
		&loglevel,
		"loglevel",
		"error",
		"The log level. Valid levels are trace, debug, info, warn, error, and critical")

	flag.Parse()

	allConfig := managed_procs.LoadAllConfig(supervisorConf)
	allConfig.SuperVisorD.Nodaemon = *nodaemon
	allConfig.SuperVisorD.LogLevel = loglevel

	loggingFilename := allConfig.SuperVisorD.LogFile
	fmt.Printf("Supervisor is logging to %s\n", loggingFilename)
	f, err := os.OpenFile(loggingFilename, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)

	allConfig.RunAllProcesses()
}
