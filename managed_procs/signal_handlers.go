package managed_procs

import (
	"os"
	"os/signal"
	"syscall"
	"fmt"
	"log"
	"strings"
)

func (runningData RunningData) KillAllProcessesAndDie() {
	var err error
	var exitOK = true
	runningData.inShutDown = true
	for _, program := range runningData.programs {
		status := program.programStatus
		if status != PROC_FATAL && status != PROC_EXITED && status != PROC_STOPPED {
			program.channel <- PROC_FATAL
			switch program.config.StopSignal {
			case "TERM":
				log.Printf("Killing %s with SIGTERM", program.config.ProcessName)
				err = program.command.Process.Signal(syscall.SIGTERM)
			case "HUP":
				log.Printf("Killing %s with SIGHUP", program.config.ProcessName)
				err = program.command.Process.Signal(syscall.SIGHUP)
			case "INT":
				log.Printf("Killing %s with SIGINT", program.config.ProcessName)
				err = program.command.Process.Signal(syscall.SIGINT)
			case "QUIT":
				log.Printf("Killing %s with SIGQUIT", program.config.ProcessName)
				err = program.command.Process.Signal(syscall.SIGQUIT)
			case "USR1":
				log.Printf("Killing %s with SIGUSR1", program.config.ProcessName)
				err = program.command.Process.Signal(syscall.SIGUSR1)
			case "USR2":
				log.Printf("Killing %s with SIGUSR2", program.config.ProcessName)
				err = program.command.Process.Signal(syscall.SIGUSR2)
			case "KILL":
				log.Printf("Killing %s with SIGKILL", program.config.ProcessName)
				err = program.command.Process.Kill()
			default:
				log.Printf("Killing %s with SIGKILL", program.config.ProcessName)
				err = program.command.Process.Kill()
			}
			if err != nil &&
						program.config.StopSignal != "KILL" &&
						!strings.Contains(err.Error(), "process already finished") {
				log.Printf("Tried to kill %s but got %s. Sending SIGKILL signal.", program.config.ProcessName, err)
				program.command.Process.Signal(syscall.SIGKILL)
			}
		}

		exitOK = exitOK && program.command.ProcessState.Success()
	}
	// Note: We might not get here due to the process manager killing us first
	if exitOK {
		syscall.Exit(0)
	}
	syscall.Exit(1)
}

func (runningData RunningData) SigTerm() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM)
	log.Println("Capturing SIGTERM")
	s := <-c
	fmt.Println("Got signal", s)
	runningData.KillAllProcessesAndDie()
}

func (runningData RunningData) SigInt() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT)
	log.Println("Capturing SIGINT")
	s := <-c
	fmt.Println("Got signal", s)
	runningData.KillAllProcessesAndDie()
}

func (runningData RunningData) SignalHandlers() {
	go runningData.SigTerm()
	go runningData.SigInt()
}