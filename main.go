package main

import (
	"flag"
	"fasthosts.com/supervisorgo/managed_procs"
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
	allConfig.RunAllProcesses2()
}
