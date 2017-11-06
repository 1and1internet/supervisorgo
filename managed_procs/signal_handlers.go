package managed_procs

import (
	"os"
	"os/signal"
	"syscall"
	"fmt"
	"log"
)

func (runningData RunningData) KillAllProcessesAndDie() {
	var err error
	for _, program := range runningData.programs {
		if program.programStatus != PROC_FATAL {
			program.channel <- PROC_FATAL
			switch program.config.StopSignal {
			case "TERM":
				err = program.command.Process.Signal(syscall.SIGTERM)
			case "HUP":
				err = program.command.Process.Signal(syscall.SIGHUP)
			case "INT":
				err = program.command.Process.Signal(syscall.SIGINT)
			case "QUIT":
				err = program.command.Process.Signal(syscall.SIGQUIT)
			case "USR1":
				err = program.command.Process.Signal(syscall.SIGUSR1)
			case "USR2":
				err = program.command.Process.Signal(syscall.SIGUSR2)
			case "KILL":
				err = program.command.Process.Kill()
			default:
				err = program.command.Process.Kill()
			}
			if err != nil && program.config.StopSignal != "QUIT" {
				log.Printf("Tried to kill %s but got %s. Sending QUIT signal.", program.config.ProcessName, err)
				program.command.Process.Signal(syscall.SIGQUIT)
			}
		}
	}
	// Note: We are unlikely to get here due to the process manager killing us first
	log.Fatal("End")
}

func (runningData RunningData) Sigusr1() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGUSR1)
	s := <-c
	fmt.Println("Got signal", s)
	runningData.KillAllProcessesAndDie()
}

func (runningData RunningData) SigTerm() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM)
	s := <-c
	fmt.Println("Got signal", s)
	runningData.KillAllProcessesAndDie()
}

func (runningData RunningData) SigInt() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT)
	s := <-c
	fmt.Println("Got signal", s)
	runningData.KillAllProcessesAndDie()
}

func (runningData RunningData) SignalHandlers() {
	go runningData.Sigusr1()
	go runningData.SigTerm()
	go runningData.SigInt()
}